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

type tegraResourceManager struct {
	resourceManager
}

var _ ResourceManager = (*tegraResourceManager)(nil)

// NewTegraResourceManagers returns a set of ResourceManagers for tegra resources
func NewTegraResourceManagers(config *spec.Config) ([]ResourceManager, error) {
	deviceMap, err := buildTegraDeviceMap(config)
	if err != nil {
		return nil, fmt.Errorf("error building Tegra device map: %v", err)
	}

	deviceMap, err = updateDeviceMapWithReplicas(config.Sharing.ReplicatedResources(), deviceMap)
	if err != nil {
		return nil, fmt.Errorf("error updating device map with replicas from sharing resources: %v", err)
	}

	var rms []ResourceManager
	for resourceName, devices := range deviceMap {
		if len(devices) == 0 {
			continue
		}
		r := &tegraResourceManager{
			resourceManager: resourceManager{
				config:   config,
				resource: resourceName,
				devices:  devices,
			},
		}
		if len(devices) != 0 {
			rms = append(rms, r)
		}
	}

	return rms, nil
}

// GetPreferredAllocation returns a standard allocation for the Tegra resource manager.
func (r *tegraResourceManager) GetPreferredAllocation(available, required []string, size int) ([]string, error) {
	return r.distributedAlloc(available, required, size)
}

// GetDevicePaths returns an empty slice for the tegraResourceManager
func (r *tegraResourceManager) GetDevicePaths(ids []string) []string {
	return nil
}

// CheckHealth is disabled for the tegraResourceManager
func (r *tegraResourceManager) CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error {
	return nil
}
