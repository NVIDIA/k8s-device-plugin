/*
 * Copyright (c) 2019-2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY Type, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

import (
	"fmt"
	"log"
	"os"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

type nvmlResourceManager struct {
	resourceManager
	nvml nvml.Interface
}

var _ ResourceManager = (*nvmlResourceManager)(nil)

// NewNVMLResourceManagers returns a set of ResourceManagers, one for each NVML resource in 'config'.
func NewNVMLResourceManagers(nvmllib nvml.Interface, config *spec.Config) ([]ResourceManager, error) {
	ret := nvmllib.Init()
	if ret != nvml.SUCCESS {
		log.SetOutput(os.Stderr)
		log.Printf("Failed to initialize NVML: %v.", ret)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")
		log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
		log.Printf("If this is not a GPU node, you should set up a toleration or nodeSelector to only deploy this plugin on GPU nodes")
		log.SetOutput(os.Stdout)
		if *config.Flags.FailOnInitError {
			return nil, fmt.Errorf("failed to initialize NVML: %v", ret)
		}
		return nil, nil
	}
	defer func() {
		ret := nvmllib.Shutdown()
		if ret != nvml.SUCCESS {
			log.Printf("Error shutting down NVML: %v", ret)
		}
	}()

	deviceMap, err := NewDeviceMap(nvmllib, config)
	if err != nil {
		return nil, fmt.Errorf("error building device map: %v", err)
	}

	var rms []ResourceManager
	for resourceName, devices := range deviceMap {
		if len(devices) == 0 {
			continue
		}
		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config:   config,
				resource: resourceName,
				devices:  devices,
			},
			nvml: nvmllib,
		}
		rms = append(rms, r)
	}

	return rms, nil
}

// GetPreferredAllocation runs an allocation algorithm over the inputs.
// The algorithm chosen is based both on the incoming set of available devices and various config settings.
func (r *nvmlResourceManager) GetPreferredAllocation(available, required []string, size int) ([]string, error) {
	return r.getPreferredAllocation(available, required, size)
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *nvmlResourceManager) CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error {
	return r.checkHealth(stop, r.devices, unhealthy)
}
