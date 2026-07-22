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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// replicaCount tracks the total and available replica counts for a physical GPU.
type replicaCount struct {
	total     int
	available int
}

// allocated returns the number of replicas currently allocated on the GPU.
func (rc *replicaCount) allocated() int {
	return rc.total - rc.available
}

// gpuAllocState is the per-physical-GPU bookkeeping the greedy allocator
// tracks while it consumes candidates. A comparator sees both the shared
// cluster-wide replicaCount and the per-allocation pickedFrom counter so it
// can express policies that mix the two axes (e.g. spread, which orders
// primarily by pickedFrom and only tie-breaks by allocated()).
type gpuAllocState struct {
	count      *replicaCount // shared reference to this GPU's replicaCount
	pickedFrom int           // slots picked from this GPU during this allocation
	replicas   []string      // remaining annotated-ID candidates for this GPU
}

// replicaComparator decides whether the physical GPU represented by i should
// be preferred over the one represented by j when greedily selecting the next
// device to allocate. Comparators are complete Less functions — they own both
// the primary ordering and the tie-break — so a new policy can freely choose
// which axis of gpuAllocState matters most.
type replicaComparator func(i, j *gpuAllocState) bool

// allocationComparators maps each allocation policy to the comparator that
// implements it. All policies share the same greedy selection loop
// (greedyAlloc) and differ only in how the next best candidate is chosen.
var allocationComparators = map[string]replicaComparator{
	// distributed prefers GPUs with the fewest allocated replicas to spread
	// workload evenly across physical GPUs. Equal allocated counts fall back
	// to pickedFrom so the pod's own picks also rotate.
	spec.AllocationPolicyDistributed: func(i, j *gpuAllocState) bool {
		if i.count.allocated() != j.count.allocated() {
			return i.count.allocated() < j.count.allocated()
		}
		return i.pickedFrom < j.pickedFrom
	},
	// packed prefers GPUs with the most allocated replicas to consolidate
	// workloads onto fewer physical GPUs. Equal allocated counts still
	// rotate by pickedFrom so a single pod that overflows a GPU spreads its
	// overflow rather than concentrating arbitrarily.
	spec.AllocationPolicyPacked: func(i, j *gpuAllocState) bool {
		if i.count.allocated() != j.count.allocated() {
			return i.count.allocated() > j.count.allocated()
		}
		return i.pickedFrom < j.pickedFrom
	},
	// spread maximizes the number of distinct physical GPUs the current
	// allocation touches: it prefers GPUs the pod has not yet picked from,
	// falling back to less-allocated when pickedFrom ties. Useful for
	// multi-GPU workloads (e.g. distributed training / NCCL) that benefit
	// from spanning physical hardware.
	spec.AllocationPolicySpread: func(i, j *gpuAllocState) bool {
		if i.pickedFrom != j.pickedFrom {
			return i.pickedFrom < j.pickedFrom
		}
		return i.count.allocated() < j.count.allocated()
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

// gpuPriorityQueue is a heap of *gpuAllocState whose ordering is fully
// determined by the caller-supplied comparator.
type gpuPriorityQueue struct {
	items     []*gpuAllocState
	preferred replicaComparator
}

func (q *gpuPriorityQueue) Len() int           { return len(q.items) }
func (q *gpuPriorityQueue) Less(i, j int) bool { return q.preferred(q.items[i], q.items[j]) }
func (q *gpuPriorityQueue) Swap(i, j int)      { q.items[i], q.items[j] = q.items[j], q.items[i] }
func (q *gpuPriorityQueue) Push(x any)         { q.items = append(q.items, x.(*gpuAllocState)) }
func (q *gpuPriorityQueue) Pop() any {
	n := len(q.items) - 1
	x := q.items[n]
	q.items = q.items[:n]
	return x
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

	// Bucket candidates by their underlying physical GPU. Each gpuAllocState
	// holds a shared *replicaCount so decrementing its available count also
	// updates the map entry, keeping a single source of truth.
	byGPU := make(map[string]*gpuAllocState)
	for _, c := range candidates {
		id := AnnotatedID(c).GetID()
		item, ok := byGPU[id]
		if !ok {
			item = &gpuAllocState{count: replicas[id]}
			byGPU[id] = item
		}
		item.replicas = append(item.replicas, c)
	}

	// Build the heap once and let the comparator drive ordering.
	pq := &gpuPriorityQueue{
		items:     make([]*gpuAllocState, 0, len(byGPU)),
		preferred: preferred,
	}
	for _, item := range byGPU {
		pq.items = append(pq.items, item)
	}
	heap.Init(pq)

	// Pop the best GPU, take one of its replicas, update counters, push back
	// if any remain. Total cost is O(n log m) where n is `needed` and m is
	// the number of distinct physical devices contributing candidates.
	devices := make([]string, 0, needed)
	for i := 0; i < needed; i++ {
		top := heap.Pop(pq).(*gpuAllocState)
		last := len(top.replicas) - 1
		pick := top.replicas[last]
		top.replicas = top.replicas[:last]
		top.count.available--
		top.pickedFrom++
		if len(top.replicas) > 0 {
			heap.Push(pq, top)
		}
		devices = append(devices, pick)
	}

	return append(required, devices...), nil
}
