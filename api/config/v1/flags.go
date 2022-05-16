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
	"time"

	cli "github.com/urfave/cli/v2"
)

// Flags holds the full list of flags used to configure the device plugin and GFD.
type Flags struct {
	CommandLineFlags
}

// CommandLineFlags holds the list of command line flags used to configure the device plugin and GFD.
type CommandLineFlags struct {
	MigStrategy      string                  `json:"migStrategy"                yaml:"migStrategy"`
	FailOnInitError  bool                    `json:"failOnInitError"            yaml:"failOnInitError"`
	NvidiaDriverRoot string                  `json:"nvidiaDriverRoot,omitempty" yaml:"nvidiaDriverRoot,omitempty"`
	Plugin           *PluginCommandLineFlags `json:"plugin,omitempty"           yaml:"plugin,omitempty"`
	GFD              *GFDCommandLineFlags    `json:"gfd,omitempty"              yaml:"gfd,omitempty"`
}

// PluginCommandLineFlags holds the list of command line flags specific to the device plugin.
type PluginCommandLineFlags struct {
	PassDeviceSpecs    bool   `json:"passDeviceSpecs"    yaml:"passDeviceSpecs"`
	DeviceListStrategy string `json:"deviceListStrategy" yaml:"deviceListStrategy"`
	DeviceIDStrategy   string `json:"deviceIDStrategy"   yaml:"deviceIDStrategy"`
}

// GFDCommandLineFlags holds the list of command line flags specific to GFD.
type GFDCommandLineFlags struct {
	Oneshot       bool          `json:"oneshot"       yaml:"oneshot"`
	NoTimestamp   bool          `json:"noTimestamp"   yaml:"noTimestamp"`
	SleepInterval time.Duration `json:"sleepInterval" yaml:"sleepInterval"`
	OutputFile    string        `json:"outputFile"    yaml:"outputFile"`
}

// NewCommandLineFlags builds out a CommandLineFlags struct from the flags in cli.Context.
func NewCommandLineFlags(c *cli.Context) CommandLineFlags {
	flags := CommandLineFlags{
		MigStrategy:      c.String("mig-strategy"),
		FailOnInitError:  c.Bool("fail-on-init-error"),
		NvidiaDriverRoot: c.String("nvidia-driver-root"),
	}
	flags.setPluginFlags(c)
	flags.setGFDFlags(c)
	return flags
}

// initPluginFlags initializes the CommandLineFlags.Plugin struct if it is currently nil
func (f *CommandLineFlags) initPluginFlags() {
	if f.Plugin == nil {
		f.Plugin = &PluginCommandLineFlags{}
	}
}

// setPluginFlags sets the Plugin specific flags in the CommandLineFlags struct (if there are any)
func (f *CommandLineFlags) setPluginFlags(c *cli.Context) {
	for _, flag := range c.App.Flags {
		for _, n := range flag.Names() {
			switch n {
			case "pass-device-specs":
				f.initPluginFlags()
				f.Plugin.PassDeviceSpecs = c.Bool(n)
			case "device-list-strategy":
				f.initPluginFlags()
				f.Plugin.DeviceListStrategy = c.String(n)
			case "device-id-strategy":
				f.initPluginFlags()
				f.Plugin.DeviceIDStrategy = c.String(n)
			}
		}
	}
}

// initGFDFlags initializes the CommandLineFlags.GFD struct if it is currently nil
func (f *CommandLineFlags) initGFDFlags() {
	if f.GFD == nil {
		f.GFD = &GFDCommandLineFlags{}
	}
}

// setGFDFlags sets the GFD specific flags in the CommandLineFlags struct (if there are any)
func (f *CommandLineFlags) setGFDFlags(c *cli.Context) {
	for _, flag := range c.App.Flags {
		for _, n := range flag.Names() {
			switch n {
			case "oneshot":
				f.initGFDFlags()
				f.GFD.Oneshot = c.Bool(n)
			case "output-file":
				f.initGFDFlags()
				f.GFD.OutputFile = c.String(n)
			case "sleep-interval":
				f.initGFDFlags()
				f.GFD.SleepInterval = c.Duration(n)
			case "no-timestamp":
				f.initGFDFlags()
				f.GFD.NoTimestamp = c.Bool(n)
			}
		}
	}
}

// ToMap converts a Flags struct into a generic map[interface{}]interface{}
func (f *Flags) ToMap() map[interface{}]interface{} {
	m := map[interface{}]interface{}{}
	if f == nil {
		return m
	}
	// Common flags
	m["mig-strategy"] = f.MigStrategy
	m["fail-on-init-error"] = f.FailOnInitError
	m["nvidia-driver-root"] = f.NvidiaDriverRoot
	// Plugin specific flags
	if f.Plugin != nil {
		m["pass-device-specs"] = f.Plugin.PassDeviceSpecs
		m["device-list-strategy"] = f.Plugin.DeviceListStrategy
		m["device-id-strategy"] = f.Plugin.DeviceIDStrategy
	}
	// GFD specific flags
	if f.GFD != nil {
		m["oneshot"] = f.GFD.Oneshot
		m["output-file"] = f.GFD.OutputFile
		m["sleep-interval"] = f.GFD.SleepInterval
		m["no-timestamp"] = f.GFD.NoTimestamp
	}
	return m
}
