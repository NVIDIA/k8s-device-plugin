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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/info"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

// resourceManager forms the base type for specific resource manager implementations
type resourceManager struct {
	config   *spec.Config
	resource spec.ResourceName
	devices  Devices
}

// ResourceManager provides an interface for listing a set of Devices and checking health on them
type ResourceManager interface {
	Resource() spec.ResourceName
	Devices() Devices
	GetDevicePaths([]string) ([]string, []string)
	GetPreferredAllocation(available, required []string, size int) ([]string, error)
	CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error
}

// NewResourceManagers returns a []ResourceManager, one for each resource in 'config'.
func NewResourceManagers(nvmllib nvml.Interface, config *spec.Config) ([]ResourceManager, error) {
	// logWithReason logs the output of the has* / is* checks from the info.Interface
	logWithReason := func(f func() (bool, string), tag string) bool {
		is, reason := f()
		if !is {
			tag = "non-" + tag
		}
		log.Printf("Detected %v platform: %v", tag, reason)
		return is
	}

	infolib := info.New()

	hasNVML := logWithReason(infolib.HasNvml, "NVML")
	isTegra := logWithReason(infolib.IsTegraSystem, "Tegra")

	// The NVIDIA container stack does not yet support the use of integrated AND discrete GPUs on the same node.
	if hasNVML && isTegra {
		log.Printf("WARNING: Disabling Tegra-based resources on NVML system")
		isTegra = false
	}

	var resourceManagers []ResourceManager

	if hasNVML {
		nvmlManagers, err := NewNVMLResourceManagers(nvmllib, config)
		if err != nil {
			return nil, fmt.Errorf("failed to construct NVML resource managers: %v", err)
		}
		resourceManagers = append(resourceManagers, nvmlManagers...)
	}

	if isTegra {
		tegraManagers, err := NewTegraResourceManagers(config)
		if err != nil {
			return nil, fmt.Errorf("failed to construct Tegra resource managers: %v", err)
		}
		resourceManagers = append(resourceManagers, tegraManagers...)
	}

	return resourceManagers, nil
}

// Resource gets the resource name associated with the ResourceManager
func (r *resourceManager) Resource() spec.ResourceName {
	return r.resource
}

// Resource gets the devices managed by the ResourceManager
func (r *resourceManager) Devices() Devices {
	return r.devices
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
