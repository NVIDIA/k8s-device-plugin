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
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
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

func countPerGPU(annotatedIDs []string) map[string]int {
	counts := make(map[string]int)
	for _, id := range annotatedIDs {
		counts[AnnotatedID(id).GetID()]++
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

	allocated, err := r.distributedAlloc(available, nil, 2)
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
