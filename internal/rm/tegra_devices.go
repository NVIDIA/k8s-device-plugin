/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

package rm

import (
	"fmt"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

const (
	tegraDeviceName = "tegra"
)

// buildTegraDeviceMap creates a DeviceMap for the tegra devices in the sytesm.
// NOTE: At present only a single tegra device is expected.
func buildTegraDeviceMap(config *spec.Config) (DeviceMap, error) {
	devices := make(DeviceMap)

	name := tegraDeviceName
	i := 0
	for _, resource := range config.Resources.GPUs {
		if resource.Pattern.Matches(name) {
			index := fmt.Sprintf("%d", i)
			err := devices.setEntry(resource.Name, index, &tegraDevice{})
			if err != nil {
				return nil, err
			}
			i++
		}

	}
	return devices, nil
}

type tegraDevice struct{}

var _ deviceInfo = (*tegraDevice)(nil)

// GetUUID returns the UUID of the tegra device.
// TODO: This is currently hardcoded to `tegra`
func (d *tegraDevice) GetUUID() (string, error) {
	return tegraDeviceName, nil
}

// GetPaths returns the paths for a tegra device.
// A tegra device does not have paths associated with it.
func (d *tegraDevice) GetPaths() ([]string, error) {
	return nil, nil
}

// GetNumaNode always returns unsupported for a Tegra device
func (d *tegraDevice) GetNumaNode() (bool, int, error) {
	return false, -1, nil
}

// GetTotalMemory is unsupported for a Tegra device.
func (d *tegraDevice) GetTotalMemory() (uint64, error) {
	return 0, nil
}
