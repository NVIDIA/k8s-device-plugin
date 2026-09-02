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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// BenchmarkGreedyAlloc measures greedyAlloc across a range of node shapes.
// gpus = physical GPUs, replicas per GPU (candidates n = gpus*replicas),
// request = slots requested.
func BenchmarkGreedyAlloc(b *testing.B) {
	scenarios := []struct {
		gpus, replicas, request int
	}{
		{gpus: 4, replicas: 4, request: 4},   // n=16  — small node
		{gpus: 8, replicas: 8, request: 8},   // n=64  — typical dense (8-GPU) node
		{gpus: 8, replicas: 16, request: 16}, // n=128 — 8-GPU node, aggressive sharing
	}

	cmp := comparatorForPolicy(spec.AllocationPolicyDistributed)
	for _, s := range scenarios {
		ids := make([]string, s.gpus)
		for i := 0; i < s.gpus; i++ {
			ids[i] = fmt.Sprintf("gpu%d", i)
		}
		devices := newTestDevices(ids, s.replicas)
		available := getDeviceIDs(devices)
		r := &resourceManager{config: &spec.Config{}, devices: devices}
		name := fmt.Sprintf("gpus=%d/replicas=%d/req=%d/n=%d", s.gpus, s.replicas, s.request, s.gpus*s.replicas)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := r.greedyAlloc(available, nil, s.request, cmp); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
