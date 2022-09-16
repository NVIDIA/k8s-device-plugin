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
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/info"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

var _ ResourceManager = (*resourceManager)(nil)

// resourceManager implements the ResourceManager interface
type resourceManager struct {
	nvml     nvml.Interface
	config   *spec.Config
	resource spec.ResourceName
	devices  Devices
}

// ResourceManager provides an interface for listing a set of Devices and checking health on them
type ResourceManager interface {
	Resource() spec.ResourceName
	Devices() Devices
	GetPreferredAllocation(available, required []string, size int) ([]string, error)
	CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error
}

// NewResourceManagers returns a []ResourceManager, one for each resource in 'config'.
func NewResourceManagers(nvmllib nvml.Interface, config *spec.Config) ([]ResourceManager, error) {
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
		r := &resourceManager{
			nvml:     nvmllib,
			config:   config,
			resource: resourceName,
			devices:  devices,
		}
		if len(r.Devices()) != 0 {
			rms = append(rms, r)
		}
	}

	return rms, nil
}

// Resource gets the resource name associated with the ResourceManager
func (r *resourceManager) Resource() spec.ResourceName {
	return r.resource
}

// Resource gets the devices managed by the ResourceManager
func (r *resourceManager) Devices() Devices {
	return r.devices
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *resourceManager) CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error {
	return r.checkHealth(stop, r.devices, unhealthy)
}

// GetPreferredAllocation runs an allocation algorithm over the inputs.
// The algorithm chosen is based both on the incoming set of available devices and various config settings.
func (r *resourceManager) GetPreferredAllocation(available, required []string, size int) ([]string, error) {
	return r.getPreferredAllocation(available, required, size)
}

// AddDefaultResourcesToConfig adds default resource matching rules to config.Resources
func AddDefaultResourcesToConfig(config *spec.Config) error {
	config.Resources.AddGPUResource("*", "gpu")
	switch *config.Flags.MigStrategy {
	case spec.MigStrategySingle:
		return config.Resources.AddMIGResource("*", "gpu")
	case spec.MigStrategyMixed:
		hasNVML, reason := info.New().HasNvml()
		if !hasNVML {
			log.Printf("WARNING: mig-strategy=%q is only supported with NVML", spec.MigStrategyMixed)
			log.Printf("NVML not detected: %v", reason)
			return nil
		}

		nvmllib := nvml.New()
		ret := nvmllib.Init()
		if ret != nvml.SUCCESS {
			if *config.Flags.FailOnInitError {
				return fmt.Errorf("failed to initialize NVML: %v", ret)
			}
			return nil
		}
		defer func() {
			ret := nvmllib.Shutdown()
			if ret != nvml.SUCCESS {
				log.Printf("Error shutting down NVML: %v", ret)
			}
		}()

		devicelib := device.New(
			device.WithNvml(nvmllib),
		)
		return devicelib.VisitMigProfiles(func(p device.MigProfile) error {
			info := p.GetInfo()
			if info.C != info.G {
				return nil
			}
			return config.Resources.AddMIGResource(p.String(), "mig-"+p.String())
		})
	}
	return nil
}
