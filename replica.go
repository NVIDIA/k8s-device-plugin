/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const joinStr = "-replica-"

func stripReplica(deviceReplica string) string {
	return strings.Split(deviceReplica, joinStr)[0]
}

func stripReplicas(deviceReplicaIDs []string) []string {
	deviceIDs := make([]string, 0, len(deviceReplicaIDs))
	// remove replicas. We only want the raw devices now.
	devices := make(map[string]bool)
	for _, id := range deviceReplicaIDs {
		devID := stripReplica(id)
		if _, exists := devices[devID]; !exists {
			devices[devID] = true
			deviceIDs = append(deviceIDs, devID)
		}
	}
	sort.Strings(deviceIDs)
	return deviceIDs
}

func find(a []string, x string) int {
	for i, n := range a {
		if x == n {
			return i
		}
	}
	return len(a)
}

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

type devCount struct {
	Allocated          bool
	ReplicaDeviceNames []string
}

// allocate a specific replica device to this physical device
func (d *devCount) allocate(allocatedDevice string) bool {
	idx := find(d.ReplicaDeviceNames, allocatedDevice)
	if idx == len(d.ReplicaDeviceNames) {
		return false
	}
	d.ReplicaDeviceNames = remove(d.ReplicaDeviceNames, idx)
	d.Allocated = true
	return true
}

// allocate one of it's replicas and return the one allocated
func (d *devCount) allocateAny() string {
	allocatedDevice := d.ReplicaDeviceNames[0]
	d.ReplicaDeviceNames = d.ReplicaDeviceNames[1:]
	d.Allocated = true
	return allocatedDevice
}

type NonUniqueError struct{}

var _ error = NonUniqueError{}

func (m NonUniqueError) Error() string {
	return "allocation resulted in non-unique devices due to requesting multiple GPU replicas and not having enough physical GPUs"
}

// Generate a list of devices in order in which they should be used.
func prioritizeDevices(availableDeviceIDs []string, mustIncludeDeviceIDs []string, allocationSize int) ([]string, error) {

	rawDeviceCount := make(map[string]*devCount)

	// Get the counts by raw device
	for _, id := range availableDeviceIDs {
		dev := stripReplica(id)
		deviceCount, exists := rawDeviceCount[dev]
		if exists {
			deviceCount.ReplicaDeviceNames = append(deviceCount.ReplicaDeviceNames, id)
		} else {
			rawDeviceCount[dev] = &devCount{
				Allocated:          false,
				ReplicaDeviceNames: []string{id},
			}
		}
	}

	// sort ReplicaDeviceNames to make the following deterministic
	for _, deviceCount := range rawDeviceCount {
		sort.Strings(deviceCount.ReplicaDeviceNames)
	}

	// Pick GPU one at a time (that is least used) but try to make them unique
	// return best effort slice and true if non-unique).
	allocated := make([]string, len(mustIncludeDeviceIDs), allocationSize)

	unique := true

	// allocate all the replicas that must be included
	for i, deviceID := range mustIncludeDeviceIDs {
		deviceCount, exists := rawDeviceCount[stripReplica(deviceID)]
		if !exists {
			return nil, fmt.Errorf("device '%s' in mustIncludeDeviceIDs is missing from availableDeviceIDs", deviceID)
		}
		if deviceCount.Allocated {
			// This physical GPU is already allocated so we are no longer unique
			unique = false
		}
		if !deviceCount.allocate(deviceID) {
			return nil, fmt.Errorf("device '%s' in mustIncludeDeviceIDs is missing from availableDeviceIDs", deviceID)
		}
		allocated[i] = deviceID
	}

	// Used to make the algorithm deterministic.
	// We pick the lexicongraphically first device name is always chosen.
	rawDeviceCountSorted := make([]string, 0)
	for k := range rawDeviceCount {
		rawDeviceCountSorted = append(rawDeviceCountSorted, k)
	}
	sort.Strings(rawDeviceCountSorted)

	for i := len(allocated); i < allocationSize; i++ {
		// The goal is to get the least used and most unique device.
		// First priority is selecting a unique device.
		// Second priority is selecting the least utilized device.

		// Find the least utilized device also determining if the device is unique or not.
		allocatedHighest := 0
		unallocatedHighest := 0
		var leastUtilizedDevAllocated *devCount
		var leastUtilizedDevUnallocated *devCount

		for _, dev := range rawDeviceCountSorted {
			deviceCount := rawDeviceCount[dev]
			count := len(deviceCount.ReplicaDeviceNames)
			if deviceCount.Allocated {
				if count > allocatedHighest {
					leastUtilizedDevAllocated = deviceCount
					allocatedHighest = count
				}
			} else {
				if count > unallocatedHighest {
					leastUtilizedDevUnallocated = deviceCount
					unallocatedHighest = count
				}
			}
		}

		// Prioritize unique (aka "unallocated") devices.
		var deviceCount *devCount
		if leastUtilizedDevUnallocated != nil {
			deviceCount = leastUtilizedDevUnallocated
		} else if leastUtilizedDevAllocated != nil {
			deviceCount = leastUtilizedDevAllocated
		} else {
			return nil, errors.New("no devices left to allocate")
		}
		if deviceCount.Allocated {
			// This physical GPU is already allocated so we are no longer unique
			unique = false
		}
		allocated = append(allocated, deviceCount.allocateAny())
	}
	sort.Strings(allocated)

	var err error
	if !unique {
		err = &NonUniqueError{}
	}
	return allocated, err
}
