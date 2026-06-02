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
	"container/heap"
	"fmt"
)

// gpuAllocState holds per-physical-GPU bookkeeping for a single
// distributedAlloc call.
type gpuAllocState struct {
	used       int      // (total advertised) - (currently available to this allocation)
	pickedFrom int      // slots picked from this device in the current allocation
	replicas   []string // remaining annotated-ID candidates belonging to this device
}

// gpuPriorityQueue is a min-heap of *gpuAllocState ordered primarily by
// `used` so that devices with the fewest already-allocated replicas come
// first, and tie-broken by `pickedFrom` so that devices we have not yet
// touched during this allocation are preferred when used counts match.
type gpuPriorityQueue []*gpuAllocState

func (q gpuPriorityQueue) Len() int { return len(q) }
func (q gpuPriorityQueue) Less(i, j int) bool {
	if q[i].used != q[j].used {
		return q[i].used < q[j].used
	}
	return q[i].pickedFrom < q[j].pickedFrom
}
func (q gpuPriorityQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *gpuPriorityQueue) Push(x any)   { *q = append(*q, x.(*gpuAllocState)) }
func (q *gpuPriorityQueue) Pop() any {
	n := len(*q) - 1
	x := (*q)[n]
	*q = (*q)[:n]
	return x
}

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

	// Bucket candidates by their underlying physical device and tally counts.
	// `used` is computed as (total replica records the plugin advertises for
	// this device) minus (the number of those records present in candidates).
	byGPU := make(map[string]*gpuAllocState)
	for _, c := range candidates {
		id := AnnotatedID(c).GetID()
		s, ok := byGPU[id]
		if !ok {
			s = &gpuAllocState{}
			byGPU[id] = s
		}
		s.replicas = append(s.replicas, c)
	}
	for d := range r.devices {
		if s, ok := byGPU[AnnotatedID(d).GetID()]; ok {
			s.used++
		}
	}
	for _, s := range byGPU {
		s.used -= len(s.replicas)
	}

	// Build the priority queue once; subsequent picks reorder it in O(log m).
	pq := make(gpuPriorityQueue, 0, len(byGPU))
	for _, s := range byGPU {
		pq = append(pq, s)
	}
	heap.Init(&pq)

	// Pop the highest-priority device, take one of its replicas, update its
	// counters, and push it back if more replicas remain. Total cost is
	// O(n log m), where n is `needed` and m is the number of distinct
	// physical devices contributing candidates.
	devices := make([]string, 0, needed)
	for i := 0; i < needed; i++ {
		top := heap.Pop(&pq).(*gpuAllocState)
		last := len(top.replicas) - 1
		pick := top.replicas[last]
		top.replicas = top.replicas[:last]
		top.used++
		top.pickedFrom++
		if len(top.replicas) > 0 {
			heap.Push(&pq, top)
		}
		devices = append(devices, pick)
	}

	return append(required, devices...), nil
}
