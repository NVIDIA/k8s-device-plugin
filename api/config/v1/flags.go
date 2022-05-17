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

// untypedMap is a map of 'any' to 'any' for use when using the urfave/cli altsrc input
type untypedMap map[interface{}]interface{}

// prt returns a reference to whatever type is passed into it
func ptr[T any](x T) *T {
	return &x
}

// SetIfNotNil sets the 'key' in 'untypedMap' to '*pvalue' iff 'pvalue' is not nil
func setIfNotNil[T1 any, T2 any](m untypedMap, key T1, pvalue *T2) {
	if pvalue != nil {
		m[key] = *pvalue
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
	Plugin           *PluginCommandLineFlags `json:"plugin,omitempty"           yaml:"plugin,omitempty"`
	GFD              *GFDCommandLineFlags    `json:"gfd,omitempty"              yaml:"gfd,omitempty"`
}

// PluginCommandLineFlags holds the list of command line flags specific to the device plugin.
type PluginCommandLineFlags struct {
	PassDeviceSpecs    *bool   `json:"passDeviceSpecs"    yaml:"passDeviceSpecs"`
	DeviceListStrategy *string `json:"deviceListStrategy" yaml:"deviceListStrategy"`
	DeviceIDStrategy   *string `json:"deviceIDStrategy"   yaml:"deviceIDStrategy"`
}

// GFDCommandLineFlags holds the list of command line flags specific to GFD.
type GFDCommandLineFlags struct {
	Oneshot       *bool     `json:"oneshot"       yaml:"oneshot"`
	NoTimestamp   *bool     `json:"noTimestamp"   yaml:"noTimestamp"`
	SleepInterval *Duration `json:"sleepInterval" yaml:"sleepInterval"`
	OutputFile    *string   `json:"outputFile"    yaml:"outputFile"`
}

// NewCommandLineFlags builds out a CommandLineFlags struct from the flags in cli.Context.
func NewCommandLineFlags(c *cli.Context) CommandLineFlags {
	flags := CommandLineFlags{
		MigStrategy:      ptr(c.String("mig-strategy")),
		FailOnInitError:  ptr(c.Bool("fail-on-init-error")),
		NvidiaDriverRoot: ptr(c.String("nvidia-driver-root")),
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
				f.Plugin.PassDeviceSpecs = ptr(c.Bool(n))
			case "device-list-strategy":
				f.initPluginFlags()
				f.Plugin.DeviceListStrategy = ptr(c.String(n))
			case "device-id-strategy":
				f.initPluginFlags()
				f.Plugin.DeviceIDStrategy = ptr(c.String(n))
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
				f.GFD.Oneshot = ptr(c.Bool(n))
			case "output-file":
				f.initGFDFlags()
				f.GFD.OutputFile = ptr(c.String(n))
			case "sleep-interval":
				f.initGFDFlags()
				f.GFD.SleepInterval = ptr(Duration(c.Duration(n)))
			case "no-timestamp":
				f.initGFDFlags()
				f.GFD.NoTimestamp = ptr(c.Bool(n))
			}
		}
	}
}

// toMap converts a Flags struct into a generic 'untypedMap'
func (f *Flags) toMap() untypedMap {
	m := make(untypedMap)
	if f == nil {
		return m
	}
	// Common flags
	setIfNotNil(m, "mig-strategy", f.MigStrategy)
	setIfNotNil(m, "fail-on-init-error", f.FailOnInitError)
	setIfNotNil(m, "nvidia-driver-root", f.NvidiaDriverRoot)
	// Plugin specific flags
	if f.Plugin != nil {
		setIfNotNil(m, "pass-device-specs", f.Plugin.PassDeviceSpecs)
		setIfNotNil(m, "device-list-strategy", f.Plugin.DeviceListStrategy)
		setIfNotNil(m, "device-id-strategy", f.Plugin.DeviceIDStrategy)
	}
	// GFD specific flags
	if f.GFD != nil {
		setIfNotNil(m, "oneshot", f.GFD.Oneshot)
		setIfNotNil(m, "output-file", f.GFD.OutputFile)
		setIfNotNil(m, "sleep-interval", f.GFD.SleepInterval)
		setIfNotNil(m, "no-timestamp", f.GFD.NoTimestamp)
	}
	return m
}
