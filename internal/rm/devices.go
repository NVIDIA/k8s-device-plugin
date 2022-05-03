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
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/mig"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Device wraps pluginapi.Device with extra metadata and functions.
type Device struct {
	pluginapi.Device
	Paths []string
	Index string
}

// DeviceSlice wraps a []*Device with some functions.
type DeviceSlice []*Device

// nvmlDevice wraps an nvml.Device with more functions.
type nvmlDevice nvml.Device

// ContainsMigDevices checks if a DeviceSlice contains any MIG devices or not
func (ds DeviceSlice) ContainsMigDevices() bool {
	for _, d := range ds {
		if d.IsMigDevice() {
			return true
		}
	}
	return false
}

// IsMigDevice returns checks whether d is a MIG device or not.
func (d Device) IsMigDevice() bool {
	return strings.Contains(d.Index, ":")
}

// buildDeviceMap builds a map of resource names to devices
func buildDeviceMap(config *spec.Config) (map[spec.ResourceName][]*Device, error) {
	devices := make(map[spec.ResourceName][]*Device)

	err := buildGPUDeviceMap(config, devices)
	if err != nil {
		return nil, fmt.Errorf("error building GPU device mapi: %v", err)
	}

	if config.Sharing.Mig.Strategy == spec.MigStrategyNone {
		return devices, nil
	}

	err = buildMigDeviceMap(config, devices)
	if err != nil {
		return nil, fmt.Errorf("error building MIG device map: %v", err)
	}

	return devices, nil
}

// buildGPUDeviceMap builds a map of resource names to GPU devices
func buildGPUDeviceMap(config *spec.Config, devices map[spec.ResourceName][]*Device) error {
	return walkGPUDevices(func(i int, gpu nvml.Device) error {
		name, ret := gpu.GetName()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting product name for GPU with index '%v': %v", i, nvml.ErrorString(ret))
		}
		migEnabled, err := nvmlDevice(gpu).isMigEnabled()
		if err != nil {
			return fmt.Errorf("error checking if MIG is enabled on GPU with index '%v': %v", i, err)
		}
		if migEnabled && config.Sharing.Mig.Strategy != spec.MigStrategyNone {
			return nil
		}
		for _, resource := range config.Resources.GPUs {
			if resource.Pattern.Matches(name) {
				return setGPUDeviceMapEntry(i, gpu, &resource, devices)
			}
		}
		resource := defaultGPUResource()
		return setGPUDeviceMapEntry(i, gpu, resource, devices)
	})
}

// setMigDeviceMapEntry sets the deviceMap entry for a given GPU device
func setGPUDeviceMapEntry(i int, gpu nvml.Device, resource *spec.Resource, devices map[spec.ResourceName][]*Device) error {
	dev, err := buildDevice(fmt.Sprintf("%v", i), gpu)
	if err != nil {
		return fmt.Errorf("error building GPU Device: %v", err)
	}
	devices[resource.Name] = append(devices[resource.Name], dev)
	return nil
}

// buildMigDeviceMap builds a map of resource names to MIG devices
func buildMigDeviceMap(config *spec.Config, devices map[spec.ResourceName][]*Device) error {
	return walkMigDevices(func(i, j int, mig nvml.Device) error {
		migProfile, err := nvmlDevice(mig).getMigProfile()
		if err != nil {
			return fmt.Errorf("error getting MIG profile for MIG device at index '(%v, %v)': %v", i, j, err)
		}
		for _, resource := range config.Resources.MIGs {
			if resource.Pattern.Matches(migProfile) {
				return setMigDeviceMapEntry(i, j, mig, &resource, devices)
			}
		}
		resource := defaultMigResource(migProfile, config.Sharing.Mig.Strategy)
		return setMigDeviceMapEntry(i, j, mig, resource, devices)
	})
}

// setMigDeviceMapEntry sets the deviceMap entry for a given MIG device
func setMigDeviceMapEntry(i, j int, mig nvml.Device, resource *spec.Resource, devices map[spec.ResourceName][]*Device) error {
	dev, err := buildDevice(fmt.Sprintf("%v:%v", i, j), mig)
	if err != nil {
		return fmt.Errorf("error building Device from MIG device: %v", err)
	}
	devices[resource.Name] = append(devices[resource.Name], dev)
	return nil
}

// buildDevice builds an rm.Device from an nvml.Device
func buildDevice(index string, d nvml.Device) (*Device, error) {
	uuid, ret := d.GetUUID()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("error getting UUID device: %v", nvml.ErrorString(ret))
	}

	paths, err := nvmlDevice(d).getPaths()
	if err != nil {
		return nil, fmt.Errorf("error getting device paths: %v", err)
	}

	numa, err := nvmlDevice(d).getNumaNode()
	if err != nil {
		return nil, fmt.Errorf("error getting device NUMA node: %v", err)
	}

	dev := Device{}
	dev.ID = uuid
	dev.Index = index
	dev.Paths = paths
	dev.Health = pluginapi.Healthy
	if numa != nil {
		dev.Topology = &pluginapi.TopologyInfo{
			Nodes: []*pluginapi.NUMANode{
				{
					ID: int64(*numa),
				},
			},
		}
	}

	return &dev, nil
}

// defaultGPUResource returns a Resource matching all GPUs with resource name 'gpu'.
func defaultGPUResource() *spec.Resource {
	return &spec.Resource{
		Pattern: spec.ResourcePattern("*"),
		Name:    "gpu",
	}
}

// defaultMigResource returns a Resource pairing the provided 'migProfile' with the proper resourceName depending on the 'migStrategy'.
func defaultMigResource(migProfile string, migStrategy string) *spec.Resource {
	name := spec.ResourceName("gpu")
	if migStrategy == spec.MigStrategyMixed {
		name = spec.ResourceName("mig-" + migProfile)
	}
	return &spec.Resource{
		Pattern: spec.ResourcePattern(migProfile),
		Name:    name,
	}
}

// walkGPUDevices walks all of the GPU devices reported by NVML
func walkGPUDevices(f func(i int, d nvml.Device) error) error {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("error getting device count: %v", nvml.ErrorString(ret))
	}

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting device handle for index '%v': %v", i, nvml.ErrorString(ret))
		}
		err := f(i, device)
		if err != nil {
			return err
		}
	}
	return nil
}

// walkMigDevices walks all of the MIG devices across all GPU devices reported by NVML
func walkMigDevices(f func(i, j int, d nvml.Device) error) error {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("error getting GPU device count: %v", nvml.ErrorString(ret))
	}

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting device handle for GPU with index '%v': %v", i, nvml.ErrorString(ret))
		}

		migEnabled, err := nvmlDevice(device).isMigEnabled()
		if err != nil {
			return fmt.Errorf("error checking if MIG is enabled on GPU with index '%v': %v", i, err)
		}

		if !migEnabled {
			continue
		}

		err = nvmlDevice(device).walkMigDevices(func(j int, device nvml.Device) error {
			return f(i, j, device)
		})
		if err != nil {
			return fmt.Errorf("error walking MIG devices on GPU with index '%v': %v", i, err)
		}
	}
	return nil
}

// walkMigDevices walks all of the MIG devices on a specific GPU device reported by NVML
func (d nvmlDevice) walkMigDevices(f func(i int, d nvml.Device) error) error {
	count, ret := nvml.Device(d).GetMaxMigDeviceCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("error getting max MIG device count: %v", nvml.ErrorString(ret))
	}

	for i := 0; i < count; i++ {
		device, ret := nvml.Device(d).GetMigDeviceHandleByIndex(i)
		if ret == nvml.ERROR_NOT_FOUND {
			continue
		}
		if ret == nvml.ERROR_INVALID_ARGUMENT {
			continue
		}
		if ret != nvml.SUCCESS {
			return fmt.Errorf("error getting MIG device handle at index '%v': %v", i, nvml.ErrorString(ret))
		}
		err := f(i, device)
		if err != nil {
			return err
		}
	}
	return nil
}

// isMigEnabled checks if MIG is enabled on the given GPU device
func (d nvmlDevice) isMigEnabled() (bool, error) {
	err := nvmlLookupSymbol("nvmlDeviceGetMigMode")
	if err != nil {
		return false, nil
	}

	mode, _, ret := nvml.Device(d).GetMigMode()
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return false, nil
	}
	if ret != nvml.SUCCESS {
		return false, fmt.Errorf("error getting MIG mode: %v", nvml.ErrorString(ret))
	}

	return (mode == nvml.DEVICE_MIG_ENABLE), nil
}

// isMigDevice checks if the given NVMl device is a MIG device (as opposed to a GPU device)
func (d nvmlDevice) isMigDevice() (bool, error) {
	err := nvmlLookupSymbol("nvmlDeviceIsMigDeviceHandle")
	if err != nil {
		return false, nil
	}
	isMig, ret := nvml.Device(d).IsMigDeviceHandle()
	if ret != nvml.SUCCESS {
		return false, fmt.Errorf("%v", nvml.ErrorString(ret))
	}
	return isMig, nil
}

// getMigProfile gets the MIG profile name associated with the given MIG device
func (d nvmlDevice) getMigProfile() (string, error) {
	isMig, err := d.isMigDevice()
	if err != nil {
		return "", fmt.Errorf("error checking if device is a MIG device: %v", err)
	}
	if !isMig {
		return "", fmt.Errorf("device handle is not a MIG device")
	}

	attr, ret := nvml.Device(d).GetAttributes()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("error getting MIG device attributes: %v", nvml.ErrorString(ret))
	}

	g := attr.GpuInstanceSliceCount
	c := attr.ComputeInstanceSliceCount
	gb := ((attr.MemorySizeMB + 1024 - 1) / 1024)

	var p string
	if g == c {
		p = fmt.Sprintf("%dg.%dgb", g, gb)
	} else {
		p = fmt.Sprintf("%dc.%dg.%dgb", c, g, gb)
	}

	return p, nil
}

// getPaths returns the set of Paths associated with the given device (MIG or GPU)
func (d nvmlDevice) getPaths() ([]string, error) {
	isMig, err := d.isMigDevice()
	if err != nil {
		return nil, fmt.Errorf("error checking if device is a MIG device: %v", err)
	}

	if !isMig {
		minor, ret := nvml.Device(d).GetMinorNumber()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("error getting GPU device minor number: %v", nvml.ErrorString(ret))
		}
		return []string{fmt.Sprintf("/dev/nvidia%d", minor)}, nil
	}

	uuid, ret := nvml.Device(d).GetUUID()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("error getting UUID of MIG device: %v", nvml.ErrorString(ret))
	}

	paths, err := mig.GetMigDeviceNodePaths(uuid)
	if err != nil {
		return nil, fmt.Errorf("error getting MIG device paths: %v", err)
	}

	return paths, nil
}

// getNumaNode returns the NUMA node associated with the given device (MIG or GPU)
func (d nvmlDevice) getNumaNode() (*int, error) {
	isMig, err := d.isMigDevice()
	if err != nil {
		return nil, fmt.Errorf("error checking if device is a MIG device: %v", err)
	}

	if isMig {
		parent, ret := nvml.Device(d).GetDeviceHandleFromMigDeviceHandle()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("error getting parent GPU device from MIG device: %v", nvml.ErrorString(ret))
		}
		d = nvmlDevice(parent)
	}

	info, ret := nvml.Device(d).GetPciInfo()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("error getting PCI Bus Info of device: %v", nvml.ErrorString(ret))
	}

	// Discard leading zeros.
	busID := strings.ToLower(strings.TrimPrefix(int8Slice(info.BusId[:]).String(), "0000"))

	b, err := os.ReadFile(fmt.Sprintf("/sys/bus/pci/devices/%s/numa_node", busID))
	if err != nil {
		// Report nil if NUMA support isn't enabled
		return nil, nil
	}

	node, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 8)
	if err != nil {
		return nil, fmt.Errorf("eror parsing value for NUMA node: %v", err)
	}

	if node < 0 {
		return nil, nil
	}

	n := int(node)
	return &n, nil
}
