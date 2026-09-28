/**
# Copyright 2026 NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package mps

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

func TestPerDevicePinnedDeviceMemoryLimits(t *testing.T) {
	const totalMemory = 8 * 1024 * 1024 * 1024

	devices := make(rm.Devices)
	for replica := range 4 {
		id := fmt.Sprintf("gpu-%d", replica)
		devices[id] = &rm.Device{
			Index:       "0",
			TotalMemory: totalMemory,
		}
	}

	testCases := []struct {
		name                string
		memoryLimitFactor   float64
		expectedMemoryLimit string
	}{
		{
			name:                "preserves equal share with the default factor",
			memoryLimitFactor:   1,
			expectedMemoryLimit: "2048M",
		},
		{
			name:                "scales the per-replica limit",
			memoryLimitFactor:   1.5,
			expectedMemoryLimit: "3072M",
		},
		{
			name:                "caps the limit at total device memory",
			memoryLimitFactor:   5,
			expectedMemoryLimit: "8192M",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			limits := perDevicePinnedDeviceMemoryLimits(devices, tc.memoryLimitFactor)
			require.Equal(t, tc.expectedMemoryLimit, limits["0"])
		})
	}
}
