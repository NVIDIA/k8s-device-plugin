/**
# Copyright 2024 NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package wait

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
	"github.com/NVIDIA/k8s-device-plugin/internal/logger"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"github.com/NVIDIA/k8s-device-plugin/internal/watch"
)

// NewCommand constructs a mount command.
func NewCommand() *cli.Command {
	// Create the 'generate-cdi' command
	return &cli.Command{
		Name:  "wait",
		Usage: "Waits for the mps-daemon(s) to be ready",
		Action: func(ctx *cli.Context) error {
			return waitForMps(ctx, append(ctx.Command.Flags, ctx.App.Flags...))
		},
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
	config.Flags.GFD = nil
	return config, nil
}

// waitForMps waits for an MPS daemon to be ready.
func waitForMps(c *cli.Context, flags []cli.Flag) error {
	config, err := getConfig(c, flags)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if config.Sharing.SharingStrategy() != spec.SharingStrategyMPS {
		klog.InfoS("MPS sharing not enabled; exiting", "strategy", config.Sharing.SharingStrategy())
		return nil
	}

	klog.Info("Starting OS watcher.")
	sigs := watch.Signals(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var retry <-chan time.Time

restart:
	err = assertMpsIsReady(config)
	if err == nil {
		klog.Infof("MPS is ready")
		return nil
	}
	klog.ErrorS(err, "MPS is not ready; retrying ...")
	retry = time.After(30 * time.Second)

	for {
		select {
		case <-retry:
			goto restart
			// Watch for any signals from the OS. On SIGHUP, restart this loop,
			// restarting all of the plugins in the process. On all other
			// signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Info("Received SIGHUP, restarting.")
				goto restart
			default:
				klog.Infof("Received signal \"%v\", shutting down.", s)
				goto exit
			}
		}

	}
exit:
	return nil
}

func assertMpsIsReady(config *spec.Config) error {
	mpsManager, err := mps.New(
		mps.WithConfig(config),
	)
	if err != nil {
		return fmt.Errorf("failed to create MPS manager: %w", err)
	}
	if err := mpsManager.AssertReady(); err != nil {
		return fmt.Errorf("mps manager is not ready: %w", err)
	}
	mpsDaemons, err := mpsManager.Daemons()
	if err != nil {
		return fmt.Errorf("failed to get MPS daemons: %w", err)
	}

	var daemonErrors error
	for _, mpsDaemon := range mpsDaemons {
		err := mpsDaemon.AssertHealthy()
		daemonErrors = errors.Join(daemonErrors, err)
	}
	return daemonErrors
}

func getConfig(c *cli.Context, flags []cli.Flag) (*spec.Config, error) {
	// Load the configuration file
	klog.Info("Loading configuration.")
	config, err := loadConfig(c, flags)
	if err != nil {
		return nil, fmt.Errorf("unable to load config: %v", err)
	}
	spec.DisableResourceNamingInConfig(logger.ToKlog, config)

	// Update the configuration file with default resources.
	klog.Info("Updating config with default resource matching patterns.")
	err = rm.AddDefaultResourcesToConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to add default resources to config: %v", err)
	}

	// Print the config to the output.
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to JSON: %v", err)
	}
	klog.Infof("\nRunning with config:\n%v", string(configJSON))
	return config, nil
}
