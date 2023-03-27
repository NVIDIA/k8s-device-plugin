/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

package v1

import (
	cli "github.com/urfave/cli/v2"
)

// prt returns a reference to whatever type is passed into it
func ptr[T any](x T) *T {
	return &x
}

// updateFromCLIFlag conditionally updates the config flag at 'pflag' to the value of the CLI flag with name 'flagName'
func updateFromCLIFlag[T any](pflag **T, c *cli.Context, flagName string) {
	if c.IsSet(flagName) || *pflag == (*T)(nil) {
		switch flag := any(pflag).(type) {
		case **string:
			*flag = ptr(c.String(flagName))
		case **[]string:
			*flag = ptr(c.StringSlice(flagName))
		case **bool:
			*flag = ptr(c.Bool(flagName))
		case **Duration:
			*flag = ptr(Duration(c.Duration(flagName)))
		}
	}
}

// Flags holds the full list of flags used to configure the device plugin and GFD.
type Flags struct {
	CommandLineFlags
}

// CommandLineFlags holds the list of command line flags used to configure the device plugin and GFD.
type CommandLineFlags struct {
	MigStrategy      *string                 `json:"migStrategy"                yaml:"migStrategy"`
	FailOnInitError  *bool                   `json:"failOnInitError"            yaml:"failOnInitError"`
	NvidiaDriverRoot *string                 `json:"nvidiaDriverRoot,omitempty" yaml:"nvidiaDriverRoot,omitempty"`
	GDSEnabled       *bool                   `json:"gdsEnabled"                 yaml:"gdsEnabled"`
	MOFEDEnabled     *bool                   `json:"mofedEnabled"               yaml:"mofedEnabled"`
	Plugin           *PluginCommandLineFlags `json:"plugin,omitempty"           yaml:"plugin,omitempty"`
	GFD              *GFDCommandLineFlags    `json:"gfd,omitempty"              yaml:"gfd,omitempty"`
}

// PluginCommandLineFlags holds the list of command line flags specific to the device plugin.
type PluginCommandLineFlags struct {
	PassDeviceSpecs    *bool     `json:"passDeviceSpecs"    yaml:"passDeviceSpecs"`
	DeviceListStrategy *string   `json:"deviceListStrategy" yaml:"deviceListStrategy"`
	DeviceIDStrategy   *string   `json:"deviceIDStrategy"   yaml:"deviceIDStrategy"`
	CDIEnabled         *bool     `json:"CDIEnabled"         yaml:"CDIEnabled"`
	NvidiaCTKPath      *string   `json:"nvidiaCTKPath"      yaml:"nvidiaCTKPath"`
	DriverRootCtrPath  *string   `json:"driverRootCtrPath"  yaml:"driverRootCtrPath"`
	PreStartCommands   *[]string `json:"preStartCommands"    yaml:"preStartCommands"`
}

// GFDCommandLineFlags holds the list of command line flags specific to GFD.
type GFDCommandLineFlags struct {
	Oneshot         *bool     `json:"oneshot"         yaml:"oneshot"`
	NoTimestamp     *bool     `json:"noTimestamp"     yaml:"noTimestamp"`
	SleepInterval   *Duration `json:"sleepInterval"   yaml:"sleepInterval"`
	OutputFile      *string   `json:"outputFile"      yaml:"outputFile"`
	MachineTypeFile *string   `json:"machineTypeFile" yaml:"machineTypeFile"`
}

// UpdateFromCLIFlags updates Flags from settings in the cli Flags if they are set.
func (f *Flags) UpdateFromCLIFlags(c *cli.Context, flags []cli.Flag) {
	for _, flag := range flags {
		for _, n := range flag.Names() {
			// Common flags
			switch n {
			case "mig-strategy":
				updateFromCLIFlag(&f.MigStrategy, c, n)
			case "fail-on-init-error":
				updateFromCLIFlag(&f.FailOnInitError, c, n)
			case "nvidia-driver-root":
				updateFromCLIFlag(&f.NvidiaDriverRoot, c, n)
			case "gds-enabled":
				updateFromCLIFlag(&f.GDSEnabled, c, n)
			case "mofed-enabled":
				updateFromCLIFlag(&f.MOFEDEnabled, c, n)
			}
			// Plugin specific flags
			if f.Plugin == nil {
				f.Plugin = &PluginCommandLineFlags{}
			}
			switch n {
			case "pass-device-specs":
				updateFromCLIFlag(&f.Plugin.PassDeviceSpecs, c, n)
			case "device-list-strategy":
				updateFromCLIFlag(&f.Plugin.DeviceListStrategy, c, n)
			case "device-id-strategy":
				updateFromCLIFlag(&f.Plugin.DeviceIDStrategy, c, n)
			case "cdi-enabled":
				updateFromCLIFlag(&f.Plugin.CDIEnabled, c, n)
			case "nvidia-ctk-path":
				updateFromCLIFlag(&f.Plugin.NvidiaCTKPath, c, n)
			case "driver-root-ctr-path":
				updateFromCLIFlag(&f.Plugin.DriverRootCtrPath, c, n)
			case "pre-start-command":
				updateFromCLIFlag(&f.Plugin.PreStartCommands, c, n)
			}
			// GFD specific flags
			if f.GFD == nil {
				f.GFD = &GFDCommandLineFlags{}
			}
			switch n {
			case "oneshot":
				updateFromCLIFlag(&f.GFD.Oneshot, c, n)
			case "output-file":
				updateFromCLIFlag(&f.GFD.OutputFile, c, n)
			case "sleep-interval":
				updateFromCLIFlag(&f.GFD.SleepInterval, c, n)
			case "no-timestamp":
				updateFromCLIFlag(&f.GFD.NoTimestamp, c, n)
			case "machine-type-file":
				updateFromCLIFlag(&f.GFD.MachineTypeFile, c, n)
			}
		}
	}
}
