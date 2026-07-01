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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// replicaCount tracks the total and available replica counts for a physical GPU.
type replicaCount struct {
	total, available int
}

// allocated returns the number of replicas currently allocated on the GPU.
func (rc *replicaCount) allocated() int {
	return rc.total - rc.available
}

// replicaComparator decides whether the physical GPU represented by i should
// be preferred over the one represented by j when greedily selecting the next
// device to allocate.
type replicaComparator func(i, j *replicaCount) bool

// allocationComparators maps each allocation policy to the comparator that
// implements it. All policies share the same greedy selection loop
// (greedyAlloc) and differ only in how the next best candidate is chosen.
var allocationComparators = map[string]replicaComparator{
	// distributed prefers GPUs with the fewest allocated replicas to spread
	// workload evenly across physical GPUs.
	spec.AllocationPolicyDistributed: func(i, j *replicaCount) bool {
		return i.allocated() < j.allocated()
	},
	// packed prefers GPUs with the most allocated replicas to consolidate
	// workloads onto fewer physical GPUs.
	spec.AllocationPolicyPacked: func(i, j *replicaCount) bool {
		return i.allocated() > j.allocated()
	},
}

// comparatorForPolicy returns the comparator implementing the given
// allocation policy. Unknown policies are rejected at startup, but fall back
// to the default distributed policy here as a safety net.
func comparatorForPolicy(policy string) replicaComparator {
	if comparator, ok := allocationComparators[policy]; ok {
		return comparator
	}
	return allocationComparators[spec.AllocationPolicyDistributed]
}

// prepareCandidates filters candidates from available devices (excluding required),
// validates there are enough, and builds a per-GPU replica count map.
func (r *resourceManager) prepareCandidates(available, required []string, size int) ([]string, map[string]*replicaCount, int, error) {
	candidates := r.devices.Subset(available).Difference(r.devices.Subset(required)).GetIDs()
	needed := size - len(required)

	if len(candidates) < needed {
		return nil, nil, 0, fmt.Errorf("not enough available devices to satisfy allocation")
	}

	replicas := make(map[string]*replicaCount)
	for _, c := range candidates {
		id := AnnotatedID(c).GetID()
		if _, exists := replicas[id]; !exists {
			replicas[id] = &replicaCount{}
		}
		replicas[id].available++
	}
	for d := range r.devices {
		id := AnnotatedID(d).GetID()
		if _, exists := replicas[id]; !exists {
			continue
		}
		replicas[id].total++
	}

	return candidates, replicas, needed, nil
}

// greedyAlloc returns a list of devices by repeatedly selecting the best
// remaining candidate according to the supplied comparator. It takes into
// account already allocated replicas so that consecutive allocations keep
// following the policy the comparator implements.
func (r *resourceManager) greedyAlloc(available, required []string, size int, preferred replicaComparator) ([]string, error) {
	candidates, replicas, needed, err := r.prepareCandidates(available, required, size)
	if err != nil {
		return nil, err
	}

	// Track how many slots have already been picked from each physical device
	// during this allocation. Used as the tie-break sort key below so that,
	// when the comparator ranks two physical GPUs equally, the allocator
	// rotates to a sibling device it has touched the least this round. This
	// keeps the distributed policy spreading replicas across physical GPUs
	// even when their allocated counts tie.
	pickedFrom := make(map[string]int)

	// Select devices one-by-one. The supplied comparator decides which
	// physical GPU is preferred for the current policy; when it ranks two
	// GPUs equally, fall back to the pickedFrom tie-break above.
	var devices []string
	for i := 0; i < needed; i++ {
		sort.Slice(candidates, func(i, j int) bool {
			iid := AnnotatedID(candidates[i]).GetID()
			jid := AnnotatedID(candidates[j]).GetID()
			ri, rj := replicas[iid], replicas[jid]
			if preferred(ri, rj) {
				return true
			}
			if preferred(rj, ri) {
				return false
			}
			return pickedFrom[iid] < pickedFrom[jid]
		})
		id := AnnotatedID(candidates[0]).GetID()
		pickedFrom[id]++
		replicas[id].available--
		devices = append(devices, candidates[0])
		candidates = candidates[1:]
	}

	return append(required, devices...), nil
}
