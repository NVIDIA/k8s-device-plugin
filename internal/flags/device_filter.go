/*
 * Copyright 2025 NVIDIA CORPORATION.
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

package flags

import (
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	DevicesSeparator = ","
)

type DeviceFilter struct {
	Enabled        *bool   `json:"enabled" yaml:"enabled"`
	SelectDevices  *string `json:"selectDevices,omitempty" yaml:"selectDevices,omitempty"`
	ExcludeDevices *string `json:"excludeDevices,omitempty" yaml:"excludeDevices,omitempty"`
}

func (f DeviceFilter) GetSelectDevicesList() []string {
	return strings.Split(*f.SelectDevices, DevicesSeparator)
}

func (f DeviceFilter) GetExcludeDevicesList() []string {
	return strings.Split(*f.ExcludeDevices, DevicesSeparator)
}

func (f *DeviceFilter) Flags() []cli.Flag {
	flags := []cli.Flag{
		&cli.BoolFlag{
			Name:        "device-filter-enabled",
			Usage:       "Enable device filter",
			Value:       false,
			Destination: f.Enabled,
			EnvVars:     []string{"DEVICE_FILTER_ENABLED"},
		},
		&cli.StringFlag{
			Name:        "device-filter-select-devices",
			Usage:       "The comma separated list used for the selected devices.",
			Value:       "",
			Destination: f.SelectDevices,
			EnvVars:     []string{"DEVICE_FILTER_SELECT_DEVICES"},
		},
		&cli.StringFlag{
			Name:        "device-filter-exclude-devices",
			Usage:       "The comma separated list used for the excluded devices.",
			Value:       "",
			Destination: f.ExcludeDevices,
			EnvVars:     []string{"DEVICE_FILTER_EXCLUDE_DEVICES"},
		},
	}
	return flags
}
