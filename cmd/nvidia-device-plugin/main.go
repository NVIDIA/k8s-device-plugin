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

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	config "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/fsnotify/fsnotify"
	cli "github.com/urfave/cli/v2"
	altsrc "github.com/urfave/cli/v2/altsrc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var version string // This should be set at build time to indicate the actual version

func main() {
	var flags config.CommandLineFlags
	var config config.Config
	var configFile string

	c := cli.NewApp()
	c.Version = version
	c.Before = func(ctx *cli.Context) error {
		cfg, err := setup(ctx, c.Flags)
		if err == nil {
			config = *cfg
		}
		return err
	}
	c.Action = func(ctx *cli.Context) error {
		return start(ctx, &config)
	}

	c.Flags = []cli.Flag{
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:        "mig-strategy",
				Value:       "none",
				Usage:       "the desired strategy for exposing MIG devices on GPUs that support it:\n\t\t[none | single | mixed]",
				Destination: &flags.MigStrategy,
				EnvVars:     []string{"MIG_STRATEGY"},
			},
		),
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:        "fail-on-init-error",
				Value:       true,
				Usage:       "fail the plugin if an error is encountered during initialization, otherwise block indefinitely",
				Destination: &flags.FailOnInitError,
				EnvVars:     []string{"FAIL_ON_INIT_ERROR"},
			},
		),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:        "nvidia-driver-root",
				Value:       "/",
				Usage:       "the root path for the NVIDIA driver installation (typical values are '/' or '/run/nvidia/driver')",
				Destination: &flags.NvidiaDriverRoot,
				EnvVars:     []string{"NVIDIA_DRIVER_ROOT"},
			},
		),
		altsrc.NewBoolFlag(
			&cli.BoolFlag{
				Name:        "pass-device-specs",
				Value:       false,
				Usage:       "pass the list of DeviceSpecs to the kubelet on Allocate()",
				Destination: &flags.Plugin.PassDeviceSpecs,
				EnvVars:     []string{"PASS_DEVICE_SPECS"},
			},
		),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:        "device-list-strategy",
				Value:       "envvar",
				Usage:       "the desired strategy for passing the device list to the underlying runtime:\n\t\t[envvar | volume-mounts]",
				Destination: &flags.Plugin.DeviceListStrategy,
				EnvVars:     []string{"DEVICE_LIST_STRATEGY"},
			},
		),
		altsrc.NewStringFlag(
			&cli.StringFlag{
				Name:        "device-id-strategy",
				Value:       "uuid",
				Usage:       "the desired strategy for passing device IDs to the underlying runtime:\n\t\t[uuid | index]",
				Destination: &flags.Plugin.DeviceIDStrategy,
				EnvVars:     []string{"DEVICE_ID_STRATEGY"},
			},
		),
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

func validateFlags(config *config.Config) error {
	if config.Flags.Plugin.DeviceListStrategy != DeviceListStrategyEnvvar && config.Flags.Plugin.DeviceListStrategy != DeviceListStrategyVolumeMounts {
		return fmt.Errorf("invalid --device-list-strategy option: %v", config.Flags.Plugin.DeviceListStrategy)
	}

	if config.Flags.Plugin.DeviceIDStrategy != DeviceIDStrategyUUID && config.Flags.Plugin.DeviceIDStrategy != DeviceIDStrategyIndex {
		return fmt.Errorf("invalid --device-id-strategy option: %v", config.Flags.Plugin.DeviceIDStrategy)
	}
	return nil
}

func setup(c *cli.Context, flags []cli.Flag) (*config.Config, error) {
	config, err := config.NewConfig(c, flags)
	if err != nil {
		return nil, fmt.Errorf("unable to finalize config: %v", err)
	}
	err = validateFlags(config)
	if err != nil {
		return nil, fmt.Errorf("unable to validate flags: %v", err)
	}
	return config, nil
}

func start(c *cli.Context, config *config.Config) error {
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %v", err)
	}
	log.Printf("\nRunning with config:\n%v", string(configJSON))

	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("Failed to initialize NVML: %v.", err)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")
		log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
		log.Printf("If this is not a GPU node, you should set up a toleration or nodeSelector to only deploy this plugin on GPU nodes")
		if config.Flags.FailOnInitError {
			return fmt.Errorf("failed to initialize NVML: %v", err)
		}
		select {}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		return fmt.Errorf("failed to create FS watcher: %v", err)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var plugins []*NvidiaDevicePlugin
restart:
	// If we are restarting, idempotently stop any running plugins before
	// recreating them below.
	for _, p := range plugins {
		p.Stop()
	}

	log.Println("Retreiving plugins.")
	migStrategy, err := NewMigStrategy(config)
	if err != nil {
		return fmt.Errorf("error creating MIG strategy: %v", err)
	}
	plugins = migStrategy.GetPlugins()

	// Loop through all plugins, starting them if they have any devices
	// to serve. If even one plugin fails to start properly, try
	// starting them all again.
	started := 0
	pluginStartError := make(chan struct{})
	for _, p := range plugins {
		// Just continue if there are no devices to serve for plugin p.
		if len(p.Devices()) == 0 {
			continue
		}

		// Start the gRPC server for plugin p and connect it with the kubelet.
		if err := p.Start(); err != nil {
			log.SetOutput(os.Stderr)
			log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
			log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
			close(pluginStartError)
			goto events
		}
		started++
	}

	if started == 0 {
		log.Println("No devices found. Waiting indefinitely.")
	}

events:
	// Start an infinite loop, waiting for several indicators to either log
	// some messages, trigger a restart of the plugins, or exit the program.
	for {
		select {
		// If there was an error starting any plugins, restart them all.
		case <-pluginStartError:
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
				for _, p := range plugins {
					p.Stop()
				}
				break events
			}
		}
	}
	return nil
}
