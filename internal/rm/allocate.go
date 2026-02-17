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

// replicaCount tracks the total and available replica counts for a physical GPU.
type replicaCount struct {
	total, available int
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

// distributedAlloc returns a list of devices such that any replicated
// devices are distributed across all replicated GPUs equally. It takes into
// account already allocated replicas to ensure a proper balance across them.
func (r *resourceManager) distributedAlloc(available, required []string, size int) ([]string, error) {
	candidates, replicas, needed, err := r.prepareCandidates(available, required, size)
	if err != nil {
		return nil, err
	}

	// Select devices one-by-one, preferring GPUs with the fewest allocated
	// replicas to spread workload evenly across physical GPUs.
	var devices []string
	for i := 0; i < needed; i++ {
		sort.Slice(candidates, func(i, j int) bool {
			iid := AnnotatedID(candidates[i]).GetID()
			jid := AnnotatedID(candidates[j]).GetID()
			idiff := replicas[iid].total - replicas[iid].available
			jdiff := replicas[jid].total - replicas[jid].available
			return idiff < jdiff
		})
		id := AnnotatedID(candidates[0]).GetID()
		replicas[id].available--
		devices = append(devices, candidates[0])
		candidates = candidates[1:]
	}

	return append(required, devices...), nil
}

// packedAlloc returns a list of devices such that any replicated devices are
// packed onto as few physical GPUs as possible. It preferentially allocates
// replicas from GPUs that already have the most allocated replicas, which
// helps consolidate workloads and free up entire GPUs for other uses.
func (r *resourceManager) packedAlloc(available, required []string, size int) ([]string, error) {
	candidates, replicas, needed, err := r.prepareCandidates(available, required, size)
	if err != nil {
		return nil, err
	}

	// Select devices one-by-one, preferring GPUs with the most allocated
	// replicas to consolidate onto fewer physical GPUs.
	var devices []string
	for i := 0; i < needed; i++ {
		sort.Slice(candidates, func(i, j int) bool {
			iid := AnnotatedID(candidates[i]).GetID()
			jid := AnnotatedID(candidates[j]).GetID()
			idiff := replicas[iid].total - replicas[iid].available
			jdiff := replicas[jid].total - replicas[jid].available
			return idiff > jdiff
		})
		id := AnnotatedID(candidates[0]).GetID()
		replicas[id].available--
		devices = append(devices, candidates[0])
		candidates = candidates[1:]
	}

	return append(required, devices...), nil
}
