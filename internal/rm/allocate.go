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
)

// distributedAlloc returns a list of devices such that any replicated
// devices are distributed across all replicated GPUs equally. It takes into
// account already allocated replicas to ensure a proper balance across them.
func (r *resourceManager) distributedAlloc(available, required []string, size int) ([]string, error) {
	// Get the set of candidate devices as the difference between available and required.
	candidates := r.devices.Subset(available).Difference(r.devices.Subset(required)).GetIDs()
	needed := size - len(required)

	if len(candidates) < needed {
		return nil, fmt.Errorf("not enough available devices to satisfy allocation")
	}

	// Count the number of currently-available replicas per underlying device.
	replicas := make(map[string]int)
	for _, c := range candidates {
		replicas[AnnotatedID(c).GetID()]++
	}

	// Grab the set of 'needed' devices one-by-one from the candidates list.
	// Before selecting each candidate, sort the list so that the device with
	// the most remaining replicas comes first. This balances allocations
	// across devices, including across devices with heterogeneous replica
	// counts.
	var devices []string
	for i := 0; i < needed; i++ {
		sort.Slice(candidates, func(i, j int) bool {
			iid := AnnotatedID(candidates[i]).GetID()
			jid := AnnotatedID(candidates[j]).GetID()
			return replicas[iid] > replicas[jid]
		})
		id := AnnotatedID(candidates[0]).GetID()
		replicas[id]--
		devices = append(devices, candidates[0])
		candidates = candidates[1:]
	}

	// Add the set of required devices to this list and return it.
	devices = append(required, devices...)

	return devices, nil
}
