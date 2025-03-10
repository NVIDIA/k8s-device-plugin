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

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Device wraps pluginapi.Device with extra metadata and functions.
type Device struct {
	pluginapi.Device
	Paths             []string
	Index             string
	TotalMemory       uint64
	ComputeCapability string
	// Replicas stores the total number of times this device is replicated.
	// If this is 0 or 1 then the device is not shared.
	Replicas int
}

// deviceInfo defines the information the required to construct a Device
type deviceInfo interface {
	GetUUID() (string, error)
	GetPaths() ([]string, error)
	GetNumaNode() (bool, int, error)
	GetTotalMemory() (uint64, error)
	GetComputeCapability() (string, error)
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

	totalMemory, err := d.GetTotalMemory()
	if err != nil {
		return nil, fmt.Errorf("error getting device memory: %w", err)
	}

	computeCapability, err := d.GetComputeCapability()
	if err != nil {
		return nil, fmt.Errorf("error getting device compute capability: %w", err)
	}

	dev := Device{
		TotalMemory:       totalMemory,
		ComputeCapability: computeCapability,
	}
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

// GetByIDOrIndex returns a reference to the device matching the either specified ID or Index (nil otherwise).
func (ds Devices) GetByIDOrIndex(in string) *Device {
	d := ds.GetByID(in)
	if d != nil {
		return d
	}
	return ds.GetByIndex(in)
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

// FilterByIDOrIndex returns the subset of devices matching the selected devices but not in excluded devices.
// Basically, this is the combination of the above Subset and Difference methods.
// It accepts inputs with the device format as ID(s) and Index(es).
func (ds Devices) FilterByIDOrIndex(selected []string, excluded []string) Devices {
	filterFunc := func(ds Devices, filter []string) Devices {
		res := make(Devices)
		for _, item := range filter {
			if d := ds.GetByIDOrIndex(item); d != nil {
				res[d.ID] = d
			}
		}
		return res
	}

	res := ds
	// Skip on the edge cases when seleted/excluded is an empty slice ([]string{""})
	if !(len(selected) == 1 && selected[0] == "") {
		f := filterFunc(ds, selected)
		res = res.Subset(f.GetIDs())
	}
	if !(len(excluded) == 1 && excluded[0] == "") {
		f := filterFunc(ds, excluded)
		res = res.Difference(f)
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

// GetUUIDs returns the uuids associated with the Device in the set.
func (ds Devices) GetUUIDs() []string {
	var res []string
	seen := make(map[string]bool)
	for _, d := range ds {
		uuid := d.GetUUID()
		if seen[uuid] {
			continue
		}
		seen[uuid] = true
		res = append(res, uuid)
	}
	return res
}

// GetPluginDevices returns the plugin Devices from all devices in the Devices
func (ds Devices) GetPluginDevices() []*pluginapi.Device {
	var res []*pluginapi.Device
	for _, device := range ds {
		d := device
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

// AlignedAllocationSupported checks whether all devices support an aligned allocation
func (ds Devices) AlignedAllocationSupported() bool {
	for _, d := range ds {
		if !d.AlignedAllocationSupported() {
			return false
		}
	}
	return true
}

// AlignedAllocationSupported checks whether the device supports an aligned allocation
func (d Device) AlignedAllocationSupported() bool {
	if d.IsMigDevice() {
		return false
	}

	for _, p := range d.Paths {
		if p == "/dev/dxg" {
			return false
		}
	}

	return true
}

// IsMigDevice returns checks whether d is a MIG device or not.
func (d Device) IsMigDevice() bool {
	return strings.Contains(d.Index, ":")
}

// GetUUID returns the UUID for the device from the annotated ID.
func (d Device) GetUUID() string {
	return AnnotatedID(d.ID).GetID()
}

// NewAnnotatedID creates a new AnnotatedID from an ID and a replica number.
func NewAnnotatedID(id string, replica int) AnnotatedID {
	return AnnotatedID(fmt.Sprintf("%s::%d", id, replica))
}

// HasAnnotations checks if an AnnotatedID has any annotations or not.
func (r AnnotatedID) HasAnnotations() bool {
	split := strings.SplitN(string(r), "::", 2)
	return len(split) == 2
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
