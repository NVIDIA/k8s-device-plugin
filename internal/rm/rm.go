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
	"errors"
	"fmt"
	"strings"

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/info"
	"github.com/NVIDIA/go-nvlib/pkg/nvml"
	"k8s.io/klog/v2"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
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
	GetDevicePaths([]string) []string
	GetPreferredAllocation(available, required []string, size int) ([]string, error)
	CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error
	ValidateRequest(AnnotatedIDs) error
}

// NewResourceManagers returns a []ResourceManager, one for each resource in 'config'.
func NewResourceManagers(nvmllib nvml.Interface, config *spec.Config) ([]ResourceManager, error) {
	// logWithReason logs the output of the has* / is* checks from the info.Interface
	logWithReason := func(f func() (bool, string), tag string) bool {
		is, reason := f()
		if !is {
			tag = "non-" + tag
		}
		klog.Infof("Detected %v platform: %v", tag, reason)
		return is
	}

	infolib := info.New()

	hasNVML := logWithReason(infolib.HasNvml, "NVML")
	isTegra := logWithReason(infolib.IsTegraSystem, "Tegra")

	if !hasNVML && !isTegra {
		klog.Error("Incompatible platform detected")
		klog.Error("If this is a GPU node, did you configure the NVIDIA Container Toolkit?")
		klog.Error("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		klog.Error("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
		klog.Error("If this is not a GPU node, you should set up a toleration or nodeSelector to only deploy this plugin on GPU nodes")
		if *config.Flags.FailOnInitError {
			return nil, fmt.Errorf("platform detection failed")
		}
		return nil, nil
	}

	// The NVIDIA container stack does not yet support the use of integrated AND discrete GPUs on the same node.
	if hasNVML && isTegra {
		klog.Warning("Disabling Tegra-based resources on NVML system")
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

var errInvalidRequest = errors.New("invalid request")

// ValidateRequests checks the requested IDs against the resource manager configuration.
// This asserts that all requested IDs are known to the resource manager and that the request is
// valid for a specified sharing configuration.
// specified sharing configuration is respected.
func (r *resourceManager) ValidateRequest(ids AnnotatedIDs) error {
	for _, id := range ids {
		if !r.devices.Contains(id) {
			return fmt.Errorf("%w: unknown device: %s", errInvalidRequest, id)
		}
	}
	// If the devices being allocated are replicas, then (conditionally)
	// error out if more than one resource is being allocated.
	if len(ids) > 1 && r.config.Sharing.ReplicatedResources().FailRequestsGreaterThanOne && ids.AnyHasAnnotations() {
		return fmt.Errorf("%w: maximum request size for shared resources is 1; found %d", errInvalidRequest, len(ids))
	}
	return nil
}

// AddDefaultResourcesToConfig adds default resource matching rules to config.Resources
func AddDefaultResourcesToConfig(config *spec.Config) error {
	_ = config.Resources.AddGPUResource("*", "gpu")
	if config.Flags.MigStrategy == nil {
		return nil
	}
	switch *config.Flags.MigStrategy {
	case spec.MigStrategySingle:
		return config.Resources.AddMIGResource("*", "gpu")
	case spec.MigStrategyMixed:
		hasNVML, reason := info.New().HasNvml()
		if !hasNVML {
			klog.Warningf("mig-strategy=%q is only supported with NVML", spec.MigStrategyMixed)
			klog.Warningf("NVML not detected: %v", reason)
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
				klog.Errorf("Error shutting down NVML: %v", ret)
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
			resourceName := strings.ReplaceAll("mig-"+p.String(), "+", ".")
			return config.Resources.AddMIGResource(p.String(), resourceName)
		})
	}
	return nil
}
