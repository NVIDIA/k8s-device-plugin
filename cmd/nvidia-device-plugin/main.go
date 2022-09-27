/*
 * Copyright (c) 2019-2021, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/info"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"github.com/fsnotify/fsnotify"
	cli "github.com/urfave/cli/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func main() {
	var configFile string

	c := cli.NewApp()
	c.Name = "NVIDIA Device Plugin"
	c.Usage = "NVIDIA device plugin for Kubernetes"
	c.Version = info.GetVersionString()
	c.Action = func(ctx *cli.Context) error {
		return start(ctx, c.Flags)
	}

	c.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "mig-strategy",
			Value:   spec.MigStrategyNone,
			Usage:   "the desired strategy for exposing MIG devices on GPUs that support it:\n\t\t[none | single | mixed]",
			EnvVars: []string{"MIG_STRATEGY"},
		},
		&cli.BoolFlag{
			Name:    "fail-on-init-error",
			Value:   true,
			Usage:   "fail the plugin if an error is encountered during initialization, otherwise block indefinitely",
			EnvVars: []string{"FAIL_ON_INIT_ERROR"},
		},
		&cli.StringFlag{
			Name:    "nvidia-driver-root",
			Value:   "/",
			Usage:   "the root path for the NVIDIA driver installation (typical values are '/' or '/run/nvidia/driver')",
			EnvVars: []string{"NVIDIA_DRIVER_ROOT"},
		},
		&cli.BoolFlag{
			Name:    "pass-device-specs",
			Value:   false,
			Usage:   "pass the list of DeviceSpecs to the kubelet on Allocate()",
			EnvVars: []string{"PASS_DEVICE_SPECS"},
		},
		&cli.StringFlag{
			Name:    "device-list-strategy",
			Value:   spec.DeviceListStrategyEnvvar,
			Usage:   "the desired strategy for passing the device list to the underlying runtime:\n\t\t[envvar | volume-mounts]",
			EnvVars: []string{"DEVICE_LIST_STRATEGY"},
		},
		&cli.StringFlag{
			Name:    "device-id-strategy",
			Value:   spec.DeviceIDStrategyUUID,
			Usage:   "the desired strategy for passing device IDs to the underlying runtime:\n\t\t[uuid | index]",
			EnvVars: []string{"DEVICE_ID_STRATEGY"},
		},
		&cli.BoolFlag{
			Name:    "gds-enabled",
			Usage:   "ensure that containers are started with NVIDIA_GDS=enabled",
			EnvVars: []string{"GDS_ENABLED"},
		},
		&cli.BoolFlag{
			Name:    "mofed-enabled",
			Usage:   "ensure that containers are started with NVIDIA_MOFED=enabled",
			EnvVars: []string{"MOFED_ENABLED"},
		},
		&cli.StringFlag{
			Name:        "config-file",
			Usage:       "the path to a config file as an alternative to command line options or environment variables",
			Destination: &configFile,
			EnvVars:     []string{"CONFIG_FILE"},
		},
	}

	err := c.Run(os.Args)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func validateFlags(config *spec.Config) error {
	if *config.Flags.Plugin.DeviceListStrategy != spec.DeviceListStrategyEnvvar && *config.Flags.Plugin.DeviceListStrategy != spec.DeviceListStrategyVolumeMounts {
		return fmt.Errorf("invalid --device-list-strategy option: %v", *config.Flags.Plugin.DeviceListStrategy)
	}

	if *config.Flags.Plugin.DeviceIDStrategy != spec.DeviceIDStrategyUUID && *config.Flags.Plugin.DeviceIDStrategy != spec.DeviceIDStrategyIndex {
		return fmt.Errorf("invalid --device-id-strategy option: %v", *config.Flags.Plugin.DeviceIDStrategy)
	}
	return nil
}

func loadConfig(c *cli.Context, flags []cli.Flag) (*spec.Config, error) {
	config, err := spec.NewConfig(c, flags)
	if err != nil {
		return nil, fmt.Errorf("unable to finalize config: %v", err)
	}
	err = validateFlags(config)
	if err != nil {
		return nil, fmt.Errorf("unable to validate flags: %v", err)
	}
	config.Flags.GFD = nil
	return config, nil
}

func start(c *cli.Context, flags []cli.Flag) error {
	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		return fmt.Errorf("failed to create FS watcher: %v", err)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var restarting bool
	var restartTimeout <-chan time.Time
	var plugins []*NvidiaDevicePlugin
restart:
	// If we are restarting, stop plugins from previous run.
	if restarting {
		err := stopPlugins(plugins)
		if err != nil {
			return fmt.Errorf("error stopping plugins from previous run: %v", err)
		}
	}

	log.Println("Starting Plugins.")
	plugins, restartPlugins, err := startPlugins(c, flags, restarting)
	if err != nil {
		return fmt.Errorf("error starting plugins: %v", err)
	}

	if restartPlugins {
		log.Printf("Failed to start one or more plugins. Retrying in 30s...")
		restartTimeout = time.After(30 * time.Second)
	}

	restarting = true

	// Start an infinite loop, waiting for several indicators to either log
	// some messages, trigger a restart of the plugins, or exit the program.
	for {
		select {
		// If the restart timout has expired, then restart the plugins
		case <-restartTimeout:
			goto restart

		// Detect a kubelet restart by watching for a newly created
		// 'pluginapi.KubeletSocket' file. When this occurs, restart this loop,
		// restarting all of the plugins in the process.
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				goto restart
			}

		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		// Watch for any signals from the OS. On SIGHUP, restart this loop,
		// restarting all of the plugins in the process. On all other
		// signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				goto restart
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				goto exit
			}
		}
	}
exit:
	err = stopPlugins(plugins)
	if err != nil {
		return fmt.Errorf("error stopping plugins: %v", err)
	}
	return nil
}

func startPlugins(c *cli.Context, flags []cli.Flag, restarting bool) ([]*NvidiaDevicePlugin, bool, error) {
	// Load the configuration file
	log.Println("Loading configuration.")
	config, err := loadConfig(c, flags)
	if err != nil {
		return nil, false, fmt.Errorf("unable to load config: %v", err)
	}
	disableResourceRenamingInConfig(config)

	// Update the configuration file with default resources.
	log.Println("Updating config with default resource matching patterns.")
	err = rm.AddDefaultResourcesToConfig(config)
	if err != nil {
		return nil, false, fmt.Errorf("unable to add default resources to config: %v", err)
	}

	// Print the config to the output.
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal config to JSON: %v", err)
	}
	log.Printf("\nRunning with config:\n%v", string(configJSON))

	// Get the set of plugins.
	log.Println("Retreiving plugins.")
	pluginManager, err := NewPluginManager(config)
	if err != nil {
		return nil, false, fmt.Errorf("error creating plugin manager: %v", err)
	}
	plugins, err := pluginManager.GetPlugins()
	if err != nil {
		return nil, false, fmt.Errorf("error getting plugins: %v", err)
	}

	// Loop through all plugins, starting them if they have any devices
	// to serve. If even one plugin fails to start properly, try
	// starting them all again.
	started := 0
	for _, p := range plugins {
		// Just continue if there are no devices to serve for plugin p.
		if len(p.Devices()) == 0 {
			continue
		}

		// Start the gRPC server for plugin p and connect it with the kubelet.
		if err := p.Start(); err != nil {
			log.SetOutput(os.Stderr)
			log.Println("Could not contact Kubelet. Did you enable the device plugin feature gate?")
			log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
			log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
			log.SetOutput(os.Stdout)
			return plugins, true, nil
		}
		started++
	}

	if started == 0 {
		log.Println("No devices found. Waiting indefinitely.")
	}

	return plugins, false, nil
}

func stopPlugins(plugins []*NvidiaDevicePlugin) error {
	log.Println("Stopping plugins.")
	for _, p := range plugins {
		p.Stop()
	}
	return nil
}

// disableResourceRenamingInConfig temporarily disable the resource renaming feature of the plugin.
// We plan to reeenable this feature in a future release.
func disableResourceRenamingInConfig(config *spec.Config) {
	// Disable resource renaming through config.Resource
	if len(config.Resources.GPUs) > 0 || len(config.Resources.MIGs) > 0 {
		log.Printf("Customizing the 'resources' field is not yet supported in the config. Ignoring...")
	}
	config.Resources.GPUs = nil
	config.Resources.MIGs = nil

	// Disable renaming / device selection in Sharing.TimeSlicing.Resources
	renameByDefault := config.Sharing.TimeSlicing.RenameByDefault
	setsNonDefaultRename := false
	setsDevices := false
	for i, r := range config.Sharing.TimeSlicing.Resources {
		if !renameByDefault && r.Rename != "" {
			setsNonDefaultRename = true
			config.Sharing.TimeSlicing.Resources[i].Rename = ""
		}
		if renameByDefault && r.Rename != r.Name.DefaultSharedRename() {
			setsNonDefaultRename = true
			config.Sharing.TimeSlicing.Resources[i].Rename = r.Name.DefaultSharedRename()
		}
		if !r.Devices.All {
			setsDevices = true
			config.Sharing.TimeSlicing.Resources[i].Devices.All = true
			config.Sharing.TimeSlicing.Resources[i].Devices.Count = 0
			config.Sharing.TimeSlicing.Resources[i].Devices.List = nil
		}
	}
	if setsNonDefaultRename {
		log.Printf("Setting the 'rename' field in sharing.timeSlicing.resources is not yet supported in the config. Ignoring...")
	}
	if setsDevices {
		log.Printf("Customizing the 'devices' field in sharing.timeSlicing.resources is not yet supported in the config. Ignoring...")
	}
}
