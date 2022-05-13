/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

	"github.com/NVIDIA/go-gpuallocator/gpuallocator"
)

var alignedAllocationPolicy = gpuallocator.NewBestEffortPolicy()

// getPreferredAllocation runs an allocation algorithm over the inputs.
// The algorithm chosen is based both on the incoming set of available devices and various config settings.
func (r *resourceManager) getPreferredAllocation(available, required []string, size int) ([]string, error) {
	// If all of the available devices are full GPUs without replicas, then
	// calculate an aligned allocation across those devices.
	if !r.Devices().ContainsMigDevices() && !AnnotatedIDs(available).AnyHasAnnotations() {
		return r.alignedAlloc(available, required, size)
	}

	// Otherwise, run a standard allocation algorithm.
	return r.alloc(available, required, size)
}

// alignedAlloc shells out to the alignedAllocationPolicy that is set in
// order to calculate the preferred allocation.
func (r *resourceManager) alignedAlloc(available, required []string, size int) ([]string, error) {
	var devices []string

	availableDevices, err := gpuallocator.NewDevicesFrom(available)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve list of available devices: %v", err)
	}

	requiredDevices, err := gpuallocator.NewDevicesFrom(required)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve list of required devices: %v", err)
	}

	allocatedDevices := alignedAllocationPolicy.Allocate(availableDevices, requiredDevices, size)

	for _, device := range allocatedDevices {
		devices = append(devices, device.UUID)
	}

	return devices, nil
}

// alloc runs a standard allocation algorithm to decide which devices should be preferred.
// At present, nothing intelligent is being done here. We plan to expand this
// in the future to implement a more sophisticated allocation algorithm.
func (r *resourceManager) alloc(available, required []string, size int) ([]string, error) {
	remainder := r.devices.Subset(available).Difference(r.devices.Subset(required)).GetIDs()
	devices := append(required, remainder...)
	if len(devices) < size {
		return nil, fmt.Errorf("not enough available devices to satisfy allocation")
	}
	return devices[:size], nil
}
