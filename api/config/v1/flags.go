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

// Flags holds the full list of flags used to configure the device plugin.
type Flags struct {
	*CommandLineFlags
}

// CommandLineFlags holds the list of command line flags used to configure the device plugin.
type CommandLineFlags struct {
	MigStrategy        string `json:"migStrategy"        yaml:"migStrategy"`
	FailOnInitError    bool   `json:"failOnInitError"    yaml:"failOnInitError"`
	PassDeviceSpecs    bool   `json:"passDeviceSpecs"    yaml:"passDeviceSpecs"`
	DeviceListStrategy string `json:"deviceListStrategy" yaml:"deviceListStrategy"`
	DeviceIDStrategy   string `json:"deviceIDStrategy"   yaml:"deviceIDStrategy"`
	NvidiaDriverRoot   string `json:"nvidiaDriverRoot"   yaml:"nvidiaDriverRoot"`
}

// NewCommandLineFlags builds out a CommandLineFlags struct from the flags in cli.Context.
func NewCommandLineFlags(c *cli.Context) *CommandLineFlags {
	return &CommandLineFlags{
		MigStrategy:        c.String("mig-strategy"),
		FailOnInitError:    c.Bool("fail-on-init-error"),
		PassDeviceSpecs:    c.Bool("pass-device-specs"),
		DeviceListStrategy: c.String("device-list-strategy"),
		DeviceIDStrategy:   c.String("device-id-strategy"),
		NvidiaDriverRoot:   c.String("nvidia-driver-root"),
	}
}

// ToMap converts a Flags struct into a generic map[interface{}]interface{}
func (f *Flags) ToMap() map[interface{}]interface{} {
	return map[interface{}]interface{}{
		"mig-strategy":         f.MigStrategy,
		"fail-on-init-error":   f.FailOnInitError,
		"pass-device-specs":    f.PassDeviceSpecs,
		"device-list-strategy": f.DeviceListStrategy,
		"device-id-strategy":   f.DeviceIDStrategy,
		"nvidia-driver-root":   f.NvidiaDriverRoot,
	}
}
