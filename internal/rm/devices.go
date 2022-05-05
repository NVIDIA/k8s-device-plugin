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
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Device wraps pluginapi.Device with extra metadata and functions.
type Device struct {
	pluginapi.Device
	Paths []string
	Index string
}

// Devices wraps a map[string]*Device with some functions.
type Devices map[string]*Device

// AnnotatedID represents an ID with a replica number embedded in it.
type AnnotatedID string

// AnnotatedIDs can be used to treat a []string as a []AnnotatedID.
type AnnotatedIDs []string

// ContainsMigDevices checks if Devices contains any MIG devices or not
func (ds Devices) ContainsMigDevices() bool {
	for _, d := range ds {
		if d.IsMigDevice() {
			return true
		}
	}
	return false
}

// Contains checks if Devices contains devices matching all ids.
func (ds Devices) Contains(ids ...string) bool {
	for _, id := range ids {
		if _, exists := ds[id]; !exists {
			return false
		}
	}
	return true
}

// Subset returns the subset of devices in Devices matching the provided ids.
// If any id in ids is not in Devices, then the subset that did match will be returned.
func (ds Devices) Subset(ids []string) Devices {
	res := make(Devices)
	for _, id := range ids {
		if d, exists := ds[id]; exists {
			res[id] = d
		}
	}
	return res
}

// GetPluginDevices returns the plugin Devices from all devices in the Devices
func (ds Devices) GetPluginDevices() []*pluginapi.Device {
	var res []*pluginapi.Device
	for _, d := range ds {
		res = append(res, &d.Device)
	}
	return res
}

// GetIndices returns the Indices from all devices in the Devices
func (ds Devices) GetIndices() []string {
	var res []string
	for _, d := range ds {
		res = append(res, d.Index)
	}
	return res
}

// GetPaths returns the Paths from all devices in the Devices
func (ds Devices) GetPaths() []string {
	var res []string
	for _, d := range ds {
		res = append(res, d.Paths...)
	}
	return res
}

// IsMigDevice returns checks whether d is a MIG device or not.
func (d Device) IsMigDevice() bool {
	return strings.Contains(d.Index, ":")
}

// NewAnnotatedID creates a new AnnotatedID from an ID and a replica number.
func NewAnnotatedID(id string, replica int) AnnotatedID {
	return AnnotatedID(fmt.Sprintf("%s::%d", id, replica))
}

// Split splits a AnnotatedID into its ID and replica number parts.
func (r AnnotatedID) Split() (string, int) {
	split := strings.SplitN(string(r), "::", 2)
	if len(split) != 2 {
		return string(r), 1
	}
	replica, _ := strconv.ParseInt(split[1], 10, 0)
	return split[0], int(replica)
}

// GetID returns just the ID part of the replicated ID
func (r AnnotatedID) GetID() string {
	id, _ := r.Split()
	return id
}

// GetIDs returns just the ID parts of the replicated IDs as a []string
func (rs AnnotatedIDs) GetIDs() []string {
	res := make([]string, len(rs))
	for i, r := range rs {
		res[i] = AnnotatedID(r).GetID()
	}
	return res
}

// buildDeviceMap builds a map of resource names to devices
func buildDeviceMap(config *spec.Config) (map[spec.ResourceName]Devices, error) {
	devices, err := buildDeviceMapFromConfigResources(config)
	if err != nil {
		return nil, fmt.Errorf("error building device map from config.resources: %v", err)
	}
	return devices, nil
}

// buildDeviceMapFromConfigResources builds a map of resource names to devices from spec.Config.Resources
func buildDeviceMapFromConfigResources(config *spec.Config) (map[spec.ResourceName]Devices, error) {
	devices := make(map[spec.ResourceName]Devices)

	err := buildGPUDeviceMap(config, devices)
	if err != nil {
		return nil, fmt.Errorf("error building GPU device map: %v", err)
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
func buildGPUDeviceMap(config *spec.Config, devices map[spec.ResourceName]Devices) error {
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
func setGPUDeviceMapEntry(i int, gpu nvml.Device, resource *spec.Resource, devices map[spec.ResourceName]Devices) error {
	dev, err := buildDevice(fmt.Sprintf("%v", i), gpu, 1)
	if err != nil {
		return fmt.Errorf("error building GPU Device: %v", err)
	}
	if devices[resource.Name] == nil {
		devices[resource.Name] = make(Devices)
	}
	devices[resource.Name][dev.ID] = dev
	return nil
}

// defaultGPUResource returns a Resource matching all GPUs with resource name 'gpu'.
func defaultGPUResource() *spec.Resource {
	return &spec.Resource{
		Pattern: spec.ResourcePattern("*"),
		Name:    "gpu",
	}
}

// buildMigDeviceMap builds a map of resource names to MIG devices
func buildMigDeviceMap(config *spec.Config, devices map[spec.ResourceName]Devices) error {
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
func setMigDeviceMapEntry(i, j int, mig nvml.Device, resource *spec.Resource, devices map[spec.ResourceName]Devices) error {
	dev, err := buildDevice(fmt.Sprintf("%v:%v", i, j), mig, 1)
	if err != nil {
		return fmt.Errorf("error building Device from MIG device: %v", err)
	}
	if devices[resource.Name] == nil {
		devices[resource.Name] = make(Devices)
	}
	devices[resource.Name][dev.ID] = dev
	return nil
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

// buildDevice builds an rm.Device from an nvml.Device
func buildDevice(index string, d nvml.Device, replica int) (*Device, error) {
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
	dev.ID = string(NewAnnotatedID(uuid, replica))
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
