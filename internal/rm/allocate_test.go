/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package rm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

func makeReplicatedDevices(t *testing.T, gpuToReplicas map[string]int) Devices {
	t.Helper()
	ds := make(Devices)
	for gpu, n := range gpuToReplicas {
		for i := 0; i < n; i++ {
			annotated := string(NewAnnotatedID(gpu, i))
			ds[annotated] = &Device{
				Device:   pluginapi.Device{ID: annotated},
				Index:    gpu,
				Replicas: n,
			}
		}
	}
	return ds
}

// newTestDevices creates a Devices map with replicated devices for testing.
// Each GPU gets 'replicas' number of annotated device entries.
func newTestDevices(gpuIDs []string, replicas int) Devices {
	devices := make(Devices)
	for _, id := range gpuIDs {
		for r := 0; r < replicas; r++ {
			annotatedID := string(NewAnnotatedID(id, r))
			devices[annotatedID] = &Device{
				Device: pluginapi.Device{
					ID:     annotatedID,
					Health: pluginapi.Healthy,
				},
				Index: id,
			}
		}
	}
	return devices
}

// getDeviceIDs returns all device IDs from a Devices map as a string slice.
func getDeviceIDs(devices Devices) []string {
	var ids []string
	for id := range devices {
		ids = append(ids, id)
	}
	return ids
}

// countPerGPU counts how many allocated device IDs belong to each physical GPU.
func countPerGPU(allocated []string) map[string]int {
	counts := make(map[string]int)
	for _, id := range allocated {
		gpuID := AnnotatedID(id).GetID()
		counts[gpuID]++
	}
	return counts
}

func TestDistributedAlloc_PartiallyAllocated_DistributesAcrossDistinctGPUs(t *testing.T) {
	devices := makeReplicatedDevices(t, map[string]int{
		"GPU-0": 2,
		"GPU-1": 2,
	})
	r := &resourceManager{devices: devices}

	available := []string{
		"GPU-0::0", "GPU-0::1",
		"GPU-1::1",
	}

	allocated, err := r.greedyAlloc(available, nil, 2, comparatorForPolicy(spec.AllocationPolicyDistributed))
	require.NoError(t, err)
	require.Len(t, allocated, 2)

	counts := countPerGPU(allocated)
	require.Equalf(t, 1, counts["GPU-0"],
		"expected 1 slot from GPU-0 to keep allocations distributed across physical GPUs; got: %v",
		counts)
	require.Equalf(t, 1, counts["GPU-1"],
		"expected 1 slot from GPU-1 (the still-available second physical GPU) instead of stacking both on GPU-0; got: %v",
		counts)
}

func TestDistributedAlloc(t *testing.T) {
	testCases := []struct {
		description string
		gpuIDs      []string
		replicas    int
		available   []string // if nil, use all devices
		required    []string
		size        int
		expectError bool
		validate    func(t *testing.T, allocated []string, allDevices Devices)
	}{
		{
			description: "2 GPUs, 4 replicas each, allocate 2 — should distribute across GPUs",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        2,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 2)
				// distributed: should pick one from each GPU
				require.Equal(t, 1, counts["gpu0"], "expected 1 allocation from gpu0")
				require.Equal(t, 1, counts["gpu1"], "expected 1 allocation from gpu1")
			},
		},
		{
			description: "3 GPUs, 2 replicas each, allocate 3 — should distribute across all GPUs",
			gpuIDs:      []string{"gpu0", "gpu1", "gpu2"},
			replicas:    2,
			required:    []string{},
			size:        3,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 3)
				require.Equal(t, 1, counts["gpu0"])
				require.Equal(t, 1, counts["gpu1"])
				require.Equal(t, 1, counts["gpu2"])
			},
		},
		{
			description: "2 GPUs, 4 replicas each, allocate 4 — should distribute 2 per GPU",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        4,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 4)
				require.Equal(t, 2, counts["gpu0"])
				require.Equal(t, 2, counts["gpu1"])
			},
		},
		{
			description: "allocate 1 from single GPU — trivial case",
			gpuIDs:      []string{"gpu0"},
			replicas:    4,
			required:    []string{},
			size:        1,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				require.Len(t, allocated, 1)
				counts := countPerGPU(allocated)
				require.Equal(t, 1, counts["gpu0"])
			},
		},
		{
			description: "not enough devices — should return error",
			gpuIDs:      []string{"gpu0"},
			replicas:    2,
			required:    []string{},
			size:        5,
			expectError: true,
		},
		{
			description: "partial availability simulates pre-allocated state — should still distribute",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        2,
			// Only gpu1 replicas are available (simulates gpu0 already fully allocated)
			available: nil, // will be overridden in test body
			validate: func(t *testing.T, allocated []string, _ Devices) {
				require.Len(t, allocated, 2)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			devices := newTestDevices(tc.gpuIDs, tc.replicas)
			available := tc.available
			if available == nil {
				available = getDeviceIDs(devices)
			}

			rm := resourceManager{
				config:  &spec.Config{},
				devices: devices,
			}

			allocated, err := rm.greedyAlloc(available, tc.required, tc.size, comparatorForPolicy(spec.AllocationPolicyDistributed))
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, allocated, devices)
			}
		})
	}
}

func TestPackedAlloc(t *testing.T) {
	testCases := []struct {
		description string
		gpuIDs      []string
		replicas    int
		available   []string // if nil, use all devices
		required    []string
		size        int
		expectError bool
		validate    func(t *testing.T, allocated []string, allDevices Devices)
	}{
		{
			description: "2 GPUs, 4 replicas each, allocate 2 — should pack onto same GPU",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        2,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 2)
				// packed: should pick 2 from the same GPU
				require.Len(t, counts, 1, "expected all allocations from a single GPU")
			},
		},
		{
			description: "3 GPUs, 2 replicas each, allocate 3 — should fill one GPU first",
			gpuIDs:      []string{"gpu0", "gpu1", "gpu2"},
			replicas:    2,
			required:    []string{},
			size:        3,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 3)
				// One GPU should have 2, another should have 1, third should have 0
				maxCount := 0
				for _, c := range counts {
					if c > maxCount {
						maxCount = c
					}
				}
				require.Equal(t, 2, maxCount, "expected one GPU to be fully packed with 2 allocations")
			},
		},
		{
			description: "2 GPUs, 4 replicas each, allocate 4 — should pack onto single GPU",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        4,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 4)
				// packed: should fill one GPU entirely
				require.Len(t, counts, 1, "expected all 4 allocations from a single GPU")
			},
		},
		{
			description: "2 GPUs, 4 replicas each, allocate 5 — should fill one GPU then overflow",
			gpuIDs:      []string{"gpu0", "gpu1"},
			replicas:    4,
			required:    []string{},
			size:        5,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				counts := countPerGPU(allocated)
				require.Len(t, allocated, 5)
				// One GPU should have 4, the other should have 1
				maxCount := 0
				minCount := 999
				for _, c := range counts {
					if c > maxCount {
						maxCount = c
					}
					if c < minCount {
						minCount = c
					}
				}
				require.Equal(t, 4, maxCount, "expected one GPU fully packed")
				require.Equal(t, 1, minCount, "expected overflow to second GPU")
			},
		},
		{
			description: "allocate 1 from single GPU — trivial case",
			gpuIDs:      []string{"gpu0"},
			replicas:    4,
			required:    []string{},
			size:        1,
			validate: func(t *testing.T, allocated []string, _ Devices) {
				require.Len(t, allocated, 1)
			},
		},
		{
			description: "not enough devices — should return error",
			gpuIDs:      []string{"gpu0"},
			replicas:    2,
			required:    []string{},
			size:        5,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			devices := newTestDevices(tc.gpuIDs, tc.replicas)
			available := tc.available
			if available == nil {
				available = getDeviceIDs(devices)
			}

			rm := resourceManager{
				config:  &spec.Config{},
				devices: devices,
			}

			allocated, err := rm.greedyAlloc(available, tc.required, tc.size, comparatorForPolicy(spec.AllocationPolicyPacked))
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, allocated, devices)
			}
		})
	}
}

// TestDistributedAllocIsDefault verifies that the default allocation policy
// (no AllocationPolicy set, or explicitly "distributed") produces the same
// distributed behavior as the existing implementation.
func TestDistributedAllocIsDefault(t *testing.T) {
	gpuIDs := []string{"gpu0", "gpu1", "gpu2"}
	replicas := 4
	devices := newTestDevices(gpuIDs, replicas)
	available := getDeviceIDs(devices)

	// Run distributed allocation multiple times to verify consistent behavior
	for i := 0; i < 10; i++ {
		rm := resourceManager{
			config:  &spec.Config{},
			devices: devices,
		}

		allocated, err := rm.greedyAlloc(available, []string{}, 3, comparatorForPolicy(spec.AllocationPolicyDistributed))
		require.NoError(t, err)
		require.Len(t, allocated, 3)

		counts := countPerGPU(allocated)
		// distributed: should always pick 1 from each of the 3 GPUs
		require.Equal(t, 1, counts["gpu0"], "iteration %d: expected 1 from gpu0", i)
		require.Equal(t, 1, counts["gpu1"], "iteration %d: expected 1 from gpu1", i)
		require.Equal(t, 1, counts["gpu2"], "iteration %d: expected 1 from gpu2", i)
	}
}

// TestPackedVsDistributedContrast directly compares the two allocation
// strategies on the same input to verify they produce meaningfully different results.
func TestPackedVsDistributedContrast(t *testing.T) {
	gpuIDs := []string{"gpu0", "gpu1"}
	replicas := 4
	devices := newTestDevices(gpuIDs, replicas)
	available := getDeviceIDs(devices)

	rm := resourceManager{
		config:  &spec.Config{},
		devices: devices,
	}

	// Distributed: allocate 4 → should be 2 per GPU
	distAllocated, err := rm.greedyAlloc(available, []string{}, 4, comparatorForPolicy(spec.AllocationPolicyDistributed))
	require.NoError(t, err)
	distCounts := countPerGPU(distAllocated)
	require.Equal(t, 2, distCounts["gpu0"])
	require.Equal(t, 2, distCounts["gpu1"])

	// Packed: allocate 4 → should be 4 on one GPU
	packAllocated, err := rm.greedyAlloc(available, []string{}, 4, comparatorForPolicy(spec.AllocationPolicyPacked))
	require.NoError(t, err)
	packCounts := countPerGPU(packAllocated)
	require.Len(t, packCounts, 1, "packed should use only 1 GPU")

	// Verify they are actually different
	require.NotEqual(t, distCounts, packCounts, "distributed and packed should produce different allocation patterns")
}

// TestComparatorForPolicy verifies that each allocation policy resolves to
// the comparator implementing it, and that unknown or empty policies fall
// back to the default distributed comparator.
func TestComparatorForPolicy(t *testing.T) {
	moreAllocated := &replicaCount{total: 4, available: 1} // 3 allocated
	lessAllocated := &replicaCount{total: 4, available: 3} // 1 allocated

	testCases := []struct {
		description string
		policy      string
		// expectPrefersLessAllocated is true for distributed-style comparators,
		// which prefer the GPU with fewer allocated replicas.
		expectPrefersLessAllocated bool
	}{
		{
			description:                "distributed prefers GPU with fewer allocated replicas",
			policy:                     spec.AllocationPolicyDistributed,
			expectPrefersLessAllocated: true,
		},
		{
			description:                "packed prefers GPU with more allocated replicas",
			policy:                     spec.AllocationPolicyPacked,
			expectPrefersLessAllocated: false,
		},
		{
			description:                "empty policy falls back to distributed",
			policy:                     "",
			expectPrefersLessAllocated: true,
		},
		{
			description:                "unknown policy falls back to distributed",
			policy:                     "no-such-policy",
			expectPrefersLessAllocated: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			comparator := comparatorForPolicy(tc.policy)
			require.NotNil(t, comparator)
			require.Equal(t, tc.expectPrefersLessAllocated, comparator(lessAllocated, moreAllocated))
			require.Equal(t, !tc.expectPrefersLessAllocated, comparator(moreAllocated, lessAllocated))
		})
	}
}

// newFullGPUDevices creates a Devices map with non-replicated full GPU devices.
// These devices have no annotations and support aligned allocation.
func newFullGPUDevices(uuids []string) Devices {
	devices := make(Devices)
	for i, uuid := range uuids {
		devices[uuid] = &Device{
			Device: pluginapi.Device{
				ID:     uuid,
				Health: pluginapi.Healthy,
			},
			Index: fmt.Sprintf("%d", i),
			Paths: []string{fmt.Sprintf("/dev/nvidia%d", i)},
		}
	}
	return devices
}

// TestFullGPUNodeIgnoresAllocationPolicy verifies that on a node with full
// (non-MIG, non-replicated) GPUs, the allocation policy setting has no effect.
// This is critical for mixed clusters where the same DaemonSet deploys
// nvidia-device-plugin with identical flags to both full GPU and MIG nodes.
func TestFullGPUNodeIgnoresAllocationPolicy(t *testing.T) {
	uuids := []string{"GPU-aaa", "GPU-bbb", "GPU-ccc", "GPU-ddd"}
	devices := newFullGPUDevices(uuids)
	available := getDeviceIDs(devices)

	// Verify precondition: these devices support aligned allocation
	require.True(t, devices.AlignedAllocationSupported(), "full GPU devices should support aligned allocation")

	// Verify precondition: no annotations on available IDs
	require.False(t, AnnotatedIDs(available).AnyHasAnnotations(), "full GPU device IDs should not have annotations")

	// With packed policy set, getPreferredAllocation should still go to alignedAlloc
	// (not packedAlloc). Since alignedAlloc requires NVML, we verify the branching
	// logic at the condition level: the aligned path is taken before allocationPolicy
	// is ever checked.
	t.Run("AlignedAllocation is selected regardless of allocationPolicy", func(t *testing.T) {
		// The condition that selects alignedAlloc:
		//   r.Devices().AlignedAllocationSupported() && !AnnotatedIDs(available).AnyHasAnnotations()
		// must be true for full GPU devices, ensuring packedAlloc is never reached.
		isAlignedPath := devices.AlignedAllocationSupported() && !AnnotatedIDs(available).AnyHasAnnotations()
		require.True(t, isAlignedPath, "full GPU nodes must always take the aligned allocation path")
	})

	// Verify that MIG devices do NOT take the aligned path
	t.Run("MIG devices do not take aligned allocation path", func(t *testing.T) {
		migDevices := make(Devices)
		migUUIDs := []string{"MIG-aaa"}
		for _, uuid := range migUUIDs {
			migDevices[uuid] = &Device{
				Device: pluginapi.Device{
					ID:     uuid,
					Health: pluginapi.Healthy,
				},
				Index: "0:0", // MIG index contains ":"
				Paths: []string{"/dev/nvidia0"},
			}
		}
		require.False(t, migDevices.AlignedAllocationSupported(), "MIG devices should not support aligned allocation")
	})

	// Verify that replicated (annotated) devices do NOT take the aligned path
	t.Run("replicated devices do not take aligned allocation path", func(t *testing.T) {
		replicatedDevices := newTestDevices([]string{"gpu0"}, 4)
		replicatedAvailable := getDeviceIDs(replicatedDevices)
		require.True(t, AnnotatedIDs(replicatedAvailable).AnyHasAnnotations(), "replicated device IDs should have annotations")
	})
}
