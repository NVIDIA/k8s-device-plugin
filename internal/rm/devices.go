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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Device wraps pluginapi.Device with extra metadata and functions.
type Device struct {
	pluginapi.Device
	Paths []string
	Index string
}

// deviceInfo defines the information the required to construct a Device
type deviceInfo interface {
	GetUUID() (string, error)
	GetPaths() ([]string, error)
	GetNumaNode() (bool, int, error)
}

// Devices wraps a map[string]*Device with some functions.
type Devices map[string]*Device

// AnnotatedID represents an ID with a replica number embedded in it.
type AnnotatedID string

// AnnotatedIDs can be used to treat a []string as a []AnnotatedID.
type AnnotatedIDs []string

// BuildDevice builds an rm.Device with the specified index and deviceInfo
func BuildDevice(index string, d deviceInfo) (*Device, error) {
	uuid, err := d.GetUUID()
	if err != nil {
		return nil, fmt.Errorf("error getting UUID device: %v", err)
	}

	paths, err := d.GetPaths()
	if err != nil {
		return nil, fmt.Errorf("error getting device paths: %v", err)
	}

	hasNuma, numa, err := d.GetNumaNode()
	if err != nil {
		return nil, fmt.Errorf("error getting device NUMA node: %v", err)
	}

	dev := Device{}
	dev.ID = uuid
	dev.Index = index
	dev.Paths = paths
	dev.Health = pluginapi.Healthy
	if hasNuma {
		dev.Topology = &pluginapi.TopologyInfo{
			Nodes: []*pluginapi.NUMANode{
				{
					ID: int64(numa),
				},
			},
		}
	}

	return &dev, nil
}

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

// updateDeviceMapWithReplicas returns an updated map of resource names to devices with replica information from spec.Config.Sharing.TimeSlicing.Resources
func updateDeviceMapWithReplicas(config *spec.Config, oDevices DeviceMap) (DeviceMap, error) {
	devices := make(DeviceMap)

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
