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
	"errors"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
)

// fakeLinkedGPUs is a minimal nvml.Interface test double for the calls that
// gpuallocator makes when linking devices.
type fakeLinkedGPUs struct {
	nvml.Interface
	gpus []*fakeLinkedGPU
}

func (f *fakeLinkedGPUs) Init() nvml.Return     { return nvml.SUCCESS }
func (f *fakeLinkedGPUs) Shutdown() nvml.Return { return nvml.SUCCESS }

func (f *fakeLinkedGPUs) DeviceGetCount() (int, nvml.Return) {
	return len(f.gpus), nvml.SUCCESS
}

func (f *fakeLinkedGPUs) DeviceGetHandleByIndex(i int) (nvml.Device, nvml.Return) {
	return f.gpus[i], nvml.SUCCESS
}

func (f *fakeLinkedGPUs) DeviceGetHandleByUUID(uuid string) (nvml.Device, nvml.Return) {
	for _, gpu := range f.gpus {
		if gpu.uuid == uuid {
			return gpu, nvml.SUCCESS
		}
	}
	return nil, nvml.ERROR_NOT_FOUND
}

// fakeLinkedGPU is a minimal nvml.Device test double. ret is returned by the
// queries about the GPU itself, and nvlinkRet by its NVLink queries, which
// gpuallocator only makes for pairs of GPUs.
type fakeLinkedGPU struct {
	nvml.Device
	uuid      string
	ret       nvml.Return
	nvlinkRet nvml.Return
}

func (g *fakeLinkedGPU) GetUUID() (string, nvml.Return)          { return g.uuid, g.ret }
func (g *fakeLinkedGPU) GetName() (string, nvml.Return)          { return "", g.ret }
func (g *fakeLinkedGPU) GetPciInfo() (nvml.PciInfo, nvml.Return) { return nvml.PciInfo{}, g.ret }

func (g *fakeLinkedGPU) GetTopologyCommonAncestor(nvml.Device) (nvml.GpuTopologyLevel, nvml.Return) {
	return nvml.TOPOLOGY_SYSTEM, nvml.SUCCESS
}

func (g *fakeLinkedGPU) GetNvLinkState(int) (nvml.EnableState, nvml.Return) {
	return nvml.FEATURE_DISABLED, g.nvlinkRet
}

func TestAlignedAlloc(t *testing.T) {
	healthy0 := &fakeLinkedGPU{uuid: "GPU-0"}
	broken0 := &fakeLinkedGPU{uuid: "GPU-0", ret: nvml.ERROR_RESET_REQUIRED}
	brokenNVLink0 := &fakeLinkedGPU{uuid: "GPU-0", nvlinkRet: nvml.ERROR_RESET_REQUIRED}
	healthy1 := &fakeLinkedGPU{uuid: "GPU-1"}
	healthy2 := &fakeLinkedGPU{uuid: "GPU-2"}

	testCases := []struct {
		description string
		gpus        []*fakeLinkedGPU
		available   []string
		required    []string
		size        int
		// expectedDevices is nil when the allocation is expected to fail.
		expectedDevices []string
		expectedBroken  []string
	}{
		{
			description:     "healthy devices are allocated",
			gpus:            []*fakeLinkedGPU{healthy0, healthy1},
			available:       []string{"GPU-0", "GPU-1"},
			size:            2,
			expectedDevices: []string{"GPU-0", "GPU-1"},
		},
		{
			description:     "a broken device that was not offered is not queried",
			gpus:            []*fakeLinkedGPU{broken0, healthy1, healthy2},
			available:       []string{"GPU-1", "GPU-2"},
			size:            2,
			expectedDevices: []string{"GPU-1", "GPU-2"},
		},
		{
			description:     "required devices are allocated",
			gpus:            []*fakeLinkedGPU{healthy0, healthy1},
			available:       []string{"GPU-0", "GPU-1"},
			required:        []string{"GPU-1"},
			size:            1,
			expectedDevices: []string{"GPU-1"},
		},
		{
			description:    "an offered device that cannot be queried is reported once",
			gpus:           []*fakeLinkedGPU{broken0, healthy1},
			available:      []string{"GPU-0", "GPU-1"},
			required:       []string{"GPU-0"},
			size:           1,
			expectedBroken: []string{"GPU-0"},
		},
		{
			description: "a failure in a query between two devices is not attributed",
			gpus:        []*fakeLinkedGPU{brokenNVLink0, healthy1},
			available:   []string{"GPU-0", "GPU-1"},
			size:        1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			r := &nvmlResourceManager{nvml: &fakeLinkedGPUs{gpus: tc.gpus}}

			devices, err := r.alignedAlloc(tc.available, tc.required, tc.size)
			if tc.expectedDevices != nil {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.expectedDevices, devices)
				return
			}

			// Checks if expected broken GPUs reported
			require.Error(t, err)
			var broken []string
			if unhealthy, ok := errors.AsType[*UnhealthyDevicesError](err); ok {
				broken = unhealthy.IDs
			}
			require.Equal(t, tc.expectedBroken, broken)
		})
	}
}
