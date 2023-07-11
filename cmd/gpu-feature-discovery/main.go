// Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/NVIDIA/gpu-feature-discovery/internal/info"
	"github.com/NVIDIA/gpu-feature-discovery/internal/lm"
	"github.com/NVIDIA/gpu-feature-discovery/internal/resource"
	"github.com/NVIDIA/gpu-feature-discovery/internal/vgpu"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

var nodeFeatureAPI bool

func main() {
	var configFile string

	c := cli.NewApp()
	c.Name = "GPU Feature Discovery"
	c.Usage = "generate labels for NVIDIA devices"
	c.Version = info.GetVersionString()
	c.Action = func(ctx *cli.Context) error {
		return start(ctx, c.Flags)
	}

	c.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "mig-strategy",
			Value:   spec.MigStrategyNone,
			Usage:   "the desired strategy for exposing MIG devices on GPUs that support it:\n\t\t[none | single | mixed]",
			EnvVars: []string{"GFD_MIG_STRATEGY", "MIG_STRATEGY"},
		},
		&cli.BoolFlag{
			Name:    "fail-on-init-error",
			Value:   true,
			Usage:   "fail the plugin if an error is encountered during initialization, otherwise block indefinitely",
			EnvVars: []string{"GFD_FAIL_ON_INIT_ERROR", "FAIL_ON_INIT_ERROR"},
		},
		&cli.BoolFlag{
			Name:    "oneshot",
			Value:   false,
			Usage:   "Label once and exit",
			EnvVars: []string{"GFD_ONESHOT"},
		},
		&cli.BoolFlag{
			Name:    "no-timestamp",
			Value:   false,
			Usage:   "Do not add the timestamp to the labels",
			EnvVars: []string{"GFD_NO_TIMESTAMP"},
		},
		&cli.DurationFlag{
			Name:    "sleep-interval",
			Value:   60 * time.Second,
			Usage:   "Time to sleep between labeling",
			EnvVars: []string{"GFD_SLEEP_INTERVAL"},
		},
		&cli.StringFlag{
			Name:    "output-file",
			Aliases: []string{"output", "o"},
			Value:   "/etc/kubernetes/node-feature-discovery/features.d/gfd",
			EnvVars: []string{"GFD_OUTPUT_FILE"},
		},
		&cli.StringFlag{
			Name:    "machine-type-file",
			Value:   "/sys/class/dmi/id/product_name",
			Usage:   "a path to a file that contains the DMI (SMBIOS) information for the node",
			EnvVars: []string{"GFD_MACHINE_TYPE_FILE"},
		},
		&cli.StringFlag{
			Name:        "config-file",
			Usage:       "the path to a config file as an alternative to command line options or environment variables",
			Destination: &configFile,
			EnvVars:     []string{"GFD_CONFIG_FILE", "CONFIG_FILE"},
		},
		&cli.BoolFlag{
			Name:        "use-node-feature-api",
			Value:       false,
			Destination: &nodeFeatureAPI,
			Usage:       "Use NFD NodeFeature API to publish labels",
			EnvVars:     []string{"GFD_USE_NODE_FEATURE_API"},
		},
	}

	if err := c.Run(os.Args); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

func validateFlags(config *spec.Config) error {
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
	config.Flags.Plugin = nil
	return config, nil
}

func start(c *cli.Context, flags []cli.Flag) error {
	defer func() {
		klog.Info("Exiting")
	}()

	klog.Info("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for {
		// Load the configuration file
		klog.Info("Loading configuration.")
		config, err := loadConfig(c, flags)
		if err != nil {
			return fmt.Errorf("unable to load config: %v", err)
		}
		disableResourceRenamingInConfig(config)

		// Print the config to the output.
		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %v", err)
		}
		klog.Infof("\nRunning with config:\n%v", string(configJSON))

		manager := resource.NewManager(config)
		vgpul := vgpu.NewVGPULib(vgpu.NewNvidiaPCILib())

		klog.Info("Start running")
		restart, err := run(manager, vgpul, config, sigs)
		if err != nil {
			return err
		}

		if !restart {
			return nil
		}
	}
}

func run(manager resource.Manager, vgpu vgpu.Interface, config *spec.Config, sigs chan os.Signal) (bool, error) {
	defer func() {
		if !nodeFeatureAPI && !*config.Flags.GFD.Oneshot && *config.Flags.GFD.OutputFile != "" {
			err := removeOutputFile(*config.Flags.GFD.OutputFile)
			if err != nil {
				klog.Warningf("Error removing output file: %v", err)
			}
		}
	}()

	timestampLabeler := lm.NewTimestampLabeler(config)
rerun:
	loopLabelers, err := lm.NewLabelers(manager, vgpu, config)
	if err != nil {
		return false, err
	}

	labelers := lm.Merge(
		timestampLabeler,
		loopLabelers,
	)

	labels, err := labelers.Labels()
	if err != nil {
		return false, fmt.Errorf("error generating labels: %v", err)
	}

	if len(labels) <= 1 {
		klog.Warning("No labels generated from any source")
	}

	klog.Info("Creating Labels")
	err = labels.Output(*config.Flags.GFD.OutputFile, nodeFeatureAPI)
	if err != nil {
		return false, err
	}

	if *config.Flags.GFD.Oneshot {
		return false, nil
	}

	klog.Info("Sleeping for ", *config.Flags.GFD.SleepInterval)
	rerunTimeout := time.After(time.Duration(*config.Flags.GFD.SleepInterval))

	for {
		select {
		case <-rerunTimeout:
			goto rerun

		// Watch for any signals from the OS. On SIGHUP trigger a reload of the config.
		// On all other signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Info("Received SIGHUP, restarting.")
				return true, nil
			default:
				klog.Infof("Received signal %v, shutting down.", s)
				return false, nil
			}
		}
	}
}

func removeOutputFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to retrieve absolute path of output file: %v", err)
	}

	absDir := filepath.Dir(absPath)
	tmpDir := filepath.Join(absDir, "gfd-tmp")

	err = os.RemoveAll(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to remove temporary output directory: %v", err)
	}

	err = os.Remove(absPath)
	if err != nil {
		return fmt.Errorf("failed to remove output file: %v", err)
	}

	return nil
}

// disableResourceRenamingInConfig temporarily disable the resource renaming feature of the plugin.
// We plan to reeenable this feature in a future release.
func disableResourceRenamingInConfig(config *spec.Config) {
	// Disable resource renaming through config.Resource
	if len(config.Resources.GPUs) > 0 || len(config.Resources.MIGs) > 0 {
		klog.Info("Customizing the 'resources' field is not yet supported in the config. Ignoring...")
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
		klog.Info("Setting the 'rename' field in sharing.timeSlicing.resources is not yet supported in the config. Ignoring...")
	}
	if setsDevices {
		klog.Info("Customizing the 'devices' field in sharing.timeSlicing.resources is not yet supported in the config. Ignoring...")
	}
}
