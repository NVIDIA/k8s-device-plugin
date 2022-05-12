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
	"sort"

	"github.com/NVIDIA/go-gpuallocator/gpuallocator"
	config "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

var bestEffortAllocatePolicy = gpuallocator.NewBestEffortPolicy()

func (r *resourceManager) GetPreferredAllocation(available, required []string, size int) ([]string, error) {
	var devices []string

	// If an allocation policy is set and none of the available device IDs has
	// attributes (i.e. is replicated), then use the gpuallocator to get the
	// preferred set of allocated devices.
	if !r.Devices().ContainsMigDevices() && !AnnotatedIDs(available).AnyHasAnnotations() {
		available, err := gpuallocator.NewDevicesFrom(available)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve list of available devices: %v", err)
		}

		required, err := gpuallocator.NewDevicesFrom(required)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve list of required devices: %v", err)
		}

		allocated := bestEffortAllocatePolicy.Allocate(available, required, size)

		for _, device := range allocated {
			devices = append(devices, device.UUID)
		}

		return devices, nil
	}

	// Otherwise return a list of sorted devices, being sure to include any
	// required ones at the front. Sorting them ensures that devices from the
	// same GPU (in the case of sharing) are chosen first before moving on to
	// the next one (i.e we follow a packed sharing strategy rather than a
	// distributed one).
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}
	for _, a := range available {
		if !requiredSet[a] {
			devices = append(devices, a)
		}
	}
	sort.Strings(devices)
	devices = append(required, devices...)
	if len(devices) < size {
		return nil, fmt.Errorf("not enough available devices to satisfy allocation")
	}

	return devices[:size], nil
}
