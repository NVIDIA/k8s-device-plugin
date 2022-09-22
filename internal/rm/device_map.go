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
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

type deviceMapBuilder struct {
	device.Interface
	config *spec.Config
}

// DeviceMap stores a set of devices per resource name.
type DeviceMap map[spec.ResourceName]Devices

// NewDeviceMap creates a device map for the specified NVML library and config.
func NewDeviceMap(nvmllib nvml.Interface, config *spec.Config) (DeviceMap, error) {
	b := deviceMapBuilder{
		Interface: device.New(device.WithNvml(nvmllib)),
		config:    config,
	}
	return b.build()
}

// build builds a map of resource names to devices.
func (b *deviceMapBuilder) build() (DeviceMap, error) {
	devices, err := b.buildDeviceMapFromConfigResources()
	if err != nil {
		return nil, fmt.Errorf("error building device map from config.resources: %v", err)
	}
	devices, err = updateDeviceMapWithReplicas(b.config, devices)
	if err != nil {
		return nil, fmt.Errorf("error updating device map with replicas from config.sharing.timeSlicing.resources: %v", err)
	}
	return devices, nil
}

// buildDeviceMapFromConfigResources builds a map of resource names to devices from spec.Config.Resources
func (b *deviceMapBuilder) buildDeviceMapFromConfigResources() (DeviceMap, error) {
	devices := make(DeviceMap)

	numGPUs, err := b.buildGPUDeviceMap(devices)
	if err != nil {
		return nil, fmt.Errorf("error building GPU device map: %v", err)
	}

	if *b.config.Flags.MigStrategy == spec.MigStrategyNone {
		return devices, nil
	}

	numMIGs, err := b.buildMigDeviceMap(devices)
	if err != nil {
		return nil, fmt.Errorf("error building MIG device map: %v", err)
	}

	var requireUniformMIGDevices bool
	if *b.config.Flags.MigStrategy == spec.MigStrategySingle {
		requireUniformMIGDevices = true
	}

	err = b.assertAllMigDevicesAreValid(requireUniformMIGDevices)
	if err != nil {
		return nil, fmt.Errorf("invalid MIG configuration: %v", err)
	}

	if requireUniformMIGDevices && numGPUs > 0 && numMIGs > 0 {
		return nil, fmt.Errorf("all devices on the node must be configured with the same migEnabled value")
	}

	return devices, nil
}

// buildGPUDeviceMap builds a map of resource names to GPU devices
func (b *deviceMapBuilder) buildGPUDeviceMap(devices DeviceMap) (int, error) {
	var numMatches int

	b.VisitDevices(func(i int, gpu device.Device) error {
		name, ret := gpu.GetName()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting product name for GPU: %v", ret)
		}
		migEnabled, err := gpu.IsMigEnabled()
		if err != nil {
			return fmt.Errorf("error checking if MIG is enabled on GPU: %v", err)
		}
		if migEnabled && *b.config.Flags.MigStrategy != spec.MigStrategyNone {
			return nil
		}
		for _, resource := range b.config.Resources.GPUs {
			if resource.Pattern.Matches(name) {
				return devices.setGPUEntry(i, gpu, &resource)
			}
		}
		return fmt.Errorf("GPU name '%v' does not match any resource patterns", name)
	})
	return numMatches, nil
}

// buildMigDeviceMap builds a map of resource names to MIG devices
func (b *deviceMapBuilder) buildMigDeviceMap(devices DeviceMap) (int, error) {
	var numMatches int
	err := b.VisitMigDevices(func(i int, d device.Device, j int, mig device.MigDevice) error {
		migProfile, err := mig.GetProfile()
		if err != nil {
			return fmt.Errorf("error getting MIG profile for MIG device at index '(%v, %v)': %v", i, j, err)
		}
		for _, resource := range b.config.Resources.MIGs {
			if resource.Pattern.Matches(migProfile.String()) {
				numMatches++
				return devices.setMigEntry(i, j, mig, &resource)
			}
		}
		return fmt.Errorf("MIG profile '%v' does not match any resource patterns", migProfile)
	})
	return numMatches, err
}

// assertAllMigDevicesAreValid ensures that each MIG-enabled device has at least one MIG device
// associated with it.
func (b *deviceMapBuilder) assertAllMigDevicesAreValid(uniform bool) error {
	err := b.VisitDevices(func(i int, d device.Device) error {
		isMigEnabled, err := d.IsMigEnabled()
		if err != nil {
			return err
		}
		if !isMigEnabled {
			return nil
		}
		migDevices, err := d.GetMigDevices()
		if err != nil {
			return err
		}
		if len(migDevices) == 0 {
			i := 0
			return fmt.Errorf("device %v has an invalid MIG configuration", i)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("At least one device with migEnabled=true was not configured correctly: %v", err)
	}

	if !uniform {
		return nil
	}

	var previousAttributes *nvml.DeviceAttributes
	return b.VisitMigDevices(func(i int, d device.Device, j int, m device.MigDevice) error {
		attrs, ret := m.GetAttributes()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting device attributes: %v", ret)
		}
		if previousAttributes == nil {
			previousAttributes = &attrs
		} else if attrs != *previousAttributes {
			return fmt.Errorf("more than one MIG device type present on node")
		}

		return nil
	})
}

// setGPUEntry sets the DeviceMap entry for a given GPU device
func (d DeviceMap) setGPUEntry(i int, gpu device.Device, resource *spec.Resource) error {
	err := d.setEntry(resource.Name, fmt.Sprintf("%v", i), nvmlDevice{gpu})
	if err != nil {
		return fmt.Errorf("error setting GPU device entry: %v", err)
	}
	return nil
}

// setMigEntry sets the DeviceMap entry for a given MIG device
func (d DeviceMap) setMigEntry(i, j int, mig nvml.Device, resource *spec.Resource) error {
	err := d.setEntry(resource.Name, fmt.Sprintf("%v:%v", i, j), nvmlMigDevice{mig})
	if err != nil {
		return fmt.Errorf("error setting MIG device entry: %v", err)
	}
	return nil
}

// setEntry sets the DeviceMap entry for the specified resource
func (d DeviceMap) setEntry(name spec.ResourceName, index string, device deviceInfo) error {
	dev, err := BuildDevice(index, device)
	if err != nil {
		return fmt.Errorf("error building Device: %v", err)
	}
	d.insert(name, dev)
	return nil
}

// insert adds the specified device to the device map
func (d DeviceMap) insert(name spec.ResourceName, dev *Device) {
	if d[name] == nil {
		d[name] = make(Devices)
	}
	d[name][dev.ID] = dev
}
