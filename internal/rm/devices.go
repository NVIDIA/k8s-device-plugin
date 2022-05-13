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

// GetByID returns a reference to the device matching the specified ID (nil otherwise).
func (ds Devices) GetByID(id string) *Device {
	return ds[id]
}

// GetByIndex returns a reference to the device matching the specified Index (nil otherwise).
func (ds Devices) GetByIndex(index string) *Device {
	for _, d := range ds {
		if d.Index == index {
			return d
		}
	}
	return nil
}

// Subset returns the subset of devices in Devices matching the provided ids.
// If any id in ids is not in Devices, then the subset that did match will be returned.
func (ds Devices) Subset(ids []string) Devices {
	res := make(Devices)
	for _, id := range ids {
		if ds.Contains(id) {
			res[id] = ds[id]
		}
	}
	return res
}

// Difference returns the set of devices contained in ds but not in ods.
func (ds Devices) Difference(ods Devices) Devices {
	res := make(Devices)
	for id := range ds {
		if !ods.Contains(id) {
			res[id] = ds[id]
		}
	}
	return res
}

// GetIDs returns the ids from all devices in the Devices
func (ds Devices) GetIDs() []string {
	var res []string
	for _, d := range ds {
		res = append(res, d.ID)
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

// HasAnnotations checks if an AnnotatedID has any annotations or not.
func (r AnnotatedID) HasAnnotations() bool {
	split := strings.SplitN(string(r), "::", 2)
	if len(split) != 2 {
		return false
	}
	return true
}

// Split splits a AnnotatedID into its ID and replica number parts.
func (r AnnotatedID) Split() (string, int) {
	split := strings.SplitN(string(r), "::", 2)
	if len(split) != 2 {
		return string(r), 0
	}
	replica, _ := strconv.ParseInt(split[1], 10, 0)
	return split[0], int(replica)
}

// GetID returns just the ID part of the replicated ID
func (r AnnotatedID) GetID() string {
	id, _ := r.Split()
	return id
}

// AnyHasAnnotations checks if any ID has annotations or not.
func (rs AnnotatedIDs) AnyHasAnnotations() bool {
	for _, r := range rs {
		if AnnotatedID(r).HasAnnotations() {
			return true
		}
	}
	return false
}

// GetIDs returns just the ID parts of the annotated IDs as a []string
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
	devices, err = updateDeviceMapWithReplicas(config, devices)
	if err != nil {
		return nil, fmt.Errorf("error updating device map with replicas from config.sharing.timeSlicing.resources: %v", err)
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

	if config.Flags.MigStrategy == spec.MigStrategyNone {
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
		if migEnabled && config.Flags.MigStrategy != spec.MigStrategyNone {
			return nil
		}
		for _, resource := range config.Resources.GPUs {
			if resource.Pattern.Matches(name) {
				return setGPUDeviceMapEntry(i, gpu, &resource, devices)
			}
		}
		return fmt.Errorf("GPU name '%v' does not match any resource patterns", name)
	})
}

// setMigDeviceMapEntry sets the deviceMap entry for a given GPU device
func setGPUDeviceMapEntry(i int, gpu nvml.Device, resource *spec.Resource, devices map[spec.ResourceName]Devices) error {
	dev, err := buildDevice(fmt.Sprintf("%v", i), gpu)
	if err != nil {
		return fmt.Errorf("error building GPU Device: %v", err)
	}
	if devices[resource.Name] == nil {
		devices[resource.Name] = make(Devices)
	}
	devices[resource.Name][dev.ID] = dev
	return nil
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
		return fmt.Errorf("MIG profile '%v' does not match any resource patterns", migProfile)
	})
}

// setMigDeviceMapEntry sets the deviceMap entry for a given MIG device
func setMigDeviceMapEntry(i, j int, mig nvml.Device, resource *spec.Resource, devices map[spec.ResourceName]Devices) error {
	dev, err := buildDevice(fmt.Sprintf("%v:%v", i, j), mig)
	if err != nil {
		return fmt.Errorf("error building Device from MIG device: %v", err)
	}
	if devices[resource.Name] == nil {
		devices[resource.Name] = make(Devices)
	}
	devices[resource.Name][dev.ID] = dev
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

// updateDeviceMapWithReplicas returns an updated map of resource names to devices with replica information from spec.Config.Sharing.TimeSlicing.Resources
func updateDeviceMapWithReplicas(config *spec.Config, oDevices map[spec.ResourceName]Devices) (map[spec.ResourceName]Devices, error) {
	devices := make(map[spec.ResourceName]Devices)

	// Begin by walking config.Sharing.TimeSlicing.Resources and building a map of just the resource names.
	names := make(map[spec.ResourceName]bool)
	for _, r := range config.Sharing.TimeSlicing.Resources {
		names[r.Name] = true
	}

	// Copy over all devices from oDevices without a resource reference in TimeSlicing.Resources.
	for r, ds := range oDevices {
		if !names[r] {
			devices[r] = ds
		}
	}

	// Walk TimeSlicing.Resources and update devices in the device map as appropriate.
	for _, r := range config.Sharing.TimeSlicing.Resources {
		// Skip any resources not matched in oDevices
		if _, exists := oDevices[r.Name]; !exists {
			continue
		}

		// Get the IDs of the devices we want to replicate from oDevices
		ids, err := getIDsOfDevicesToReplicate(&r, oDevices[r.Name])
		if err != nil {
			return nil, fmt.Errorf("unable to get IDs of devices to replicate for '%v' resource: %v", r.Name, err)
		}

		// Add any devices we don't want replicated directly into the device map.
		devices[r.Name] = make(Devices)
		for _, d := range oDevices[r.Name].Difference(oDevices[r.Name].Subset(ids)) {
			devices[r.Name][d.ID] = d
		}

		// Create replicated devices add them to the device map.
		// Rename the resource for replicated devices as requested.
		name := r.Name
		if r.Rename != "" {
			name = r.Rename
		}
		if devices[name] == nil {
			devices[name] = make(Devices)
		}
		for _, id := range ids {
			for i := 0; i < r.Replicas; i++ {
				annotatedID := string(NewAnnotatedID(id, i))
				replicatedDevice := *(oDevices[r.Name][id])
				replicatedDevice.ID = annotatedID
				devices[name][annotatedID] = &replicatedDevice
			}
		}
	}

	return devices, nil
}

// getIDsOfDevicesToReplicate returns a list of dervice IDs that we want to replicate.
func getIDsOfDevicesToReplicate(r *spec.ReplicatedResource, devices Devices) ([]string, error) {
	// If all devices for this resource type are to be replicated.
	if r.Devices.All {
		return devices.GetIDs(), nil
	}

	// If a specific number of devices for this resource type are to be replicated.
	if r.Devices.Count > 0 {
		if r.Devices.Count > len(devices) {
			return nil, fmt.Errorf("requested %d devices to be replicated, but only %d devices available", r.Devices.Count, len(devices))
		}
		return devices.GetIDs()[:r.Devices.Count], nil
	}

	// If a specific set of devices for this resource type are to be replicated.
	if len(r.Devices.List) > 0 {
		var ids []string
		for _, ref := range r.Devices.List {
			if ref.IsUUID() {
				d := devices.GetByID(string(ref))
				if d == nil {
					return nil, fmt.Errorf("no matching device with UUID: %v", ref)
				}
				ids = append(ids, d.ID)
			}
			if ref.IsGPUIndex() || ref.IsMigIndex() {
				d := devices.GetByIndex(string(ref))
				if d == nil {
					return nil, fmt.Errorf("no matching device at index: %v", ref)
				}
				ids = append(ids, d.ID)
			}
		}
		return ids, nil
	}

	return nil, fmt.Errorf("unexpected error")
}
