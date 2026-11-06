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

	"github.com/NVIDIA/go-gpuallocator/gpuallocator"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// fakeGPUHandle is a minimal nvml.Device test double; only the methods used
// while constructing the device list are implemented.
type fakeGPUHandle struct {
	nvml.Device
	uuid string
}

func (h fakeGPUHandle) GetName() (string, nvml.Return)          { return "Fake GPU", nvml.SUCCESS }
func (h fakeGPUHandle) GetUUID() (string, nvml.Return)          { return h.uuid, nvml.SUCCESS }
func (h fakeGPUHandle) GetPciInfo() (nvml.PciInfo, nvml.Return) { return nvml.PciInfo{}, nvml.SUCCESS }

// fakeLostGPUNvmlLib is a minimal nvml.Interface test double for a node where a
// GPU has fallen off the bus: the device count still includes it, but its handle
// cannot be retrieved. An empty UUID marks such a device.
type fakeLostGPUNvmlLib struct {
	nvml.Interface
	uuids []string
}

func (l fakeLostGPUNvmlLib) Init() nvml.Return                  { return nvml.SUCCESS }
func (l fakeLostGPUNvmlLib) Shutdown() nvml.Return              { return nvml.SUCCESS }
func (l fakeLostGPUNvmlLib) DeviceGetCount() (int, nvml.Return) { return len(l.uuids), nvml.SUCCESS }

func (l fakeLostGPUNvmlLib) DeviceGetHandleByIndex(index int) (nvml.Device, nvml.Return) {
	if index < 0 || index >= len(l.uuids) {
		return nil, nvml.ERROR_INVALID_ARGUMENT
	}
	if l.uuids[index] == "" {
		return nil, nvml.ERROR_GPU_IS_LOST
	}
	return fakeGPUHandle{uuid: l.uuids[index]}, nvml.SUCCESS
}

// fakeLostNVLinkGPUHandle is an nvml.Device test double for a GPU that has
// fallen off the bus while the plugin's NVML session still serves a handle for
// it: its identifying properties are readable, but querying the NVLink state
// fails. Handles for healthy devices report no NVLink support, as consumer GPUs
// do.
type fakeLostNVLinkGPUHandle struct {
	fakeGPUHandle
	lost bool
}

func (h fakeLostNVLinkGPUHandle) GetTopologyCommonAncestor(nvml.Device) (nvml.GpuTopologyLevel, nvml.Return) {
	return nvml.TOPOLOGY_SYSTEM, nvml.SUCCESS
}

func (h fakeLostNVLinkGPUHandle) GetNvLinkState(int) (nvml.EnableState, nvml.Return) {
	if h.lost {
		return nvml.FEATURE_DISABLED, nvml.ERROR_GPU_IS_LOST
	}
	return nvml.FEATURE_DISABLED, nvml.ERROR_NOT_SUPPORTED
}

// fakeLostNVLinkNvmlLib serves a handle for every device, including the one that
// has fallen off the bus. An empty UUID marks such a device.
type fakeLostNVLinkNvmlLib struct {
	fakeLostGPUNvmlLib
}

func (l fakeLostNVLinkNvmlLib) DeviceGetHandleByIndex(index int) (nvml.Device, nvml.Return) {
	if index < 0 || index >= len(l.uuids) {
		return nil, nvml.ERROR_INVALID_ARGUMENT
	}
	uuid, lost := l.uuids[index], false
	if uuid == "" {
		uuid, lost = fmt.Sprintf("GPU-lost-%d", index), true
	}
	return fakeLostNVLinkGPUHandle{fakeGPUHandle: fakeGPUHandle{uuid: uuid}, lost: lost}, nvml.SUCCESS
}

// newLostGPUResourceManager returns a resource manager for a node with the
// specified devices, where a device with an empty UUID has fallen off the bus
// and has already been marked unhealthy by the health check. The IDs of the
// healthy devices are returned: these are the devices that kubelet would offer
// as available.
func newLostGPUResourceManager(t *testing.T, uuids []string) (*nvmlResourceManager, []string) {
	t.Helper()

	var healthy []string
	devices := make(Devices)
	for i, uuid := range uuids {
		id, health := uuid, pluginapi.Healthy
		if uuid == "" {
			id, health = fmt.Sprintf("GPU-lost-%d", i), pluginapi.Unhealthy
		} else {
			healthy = append(healthy, id)
		}
		devices[id] = &Device{
			Device: pluginapi.Device{
				ID:     id,
				Health: health,
			},
			Index: fmt.Sprintf("%d", i),
			Paths: []string{fmt.Sprintf("/dev/nvidia%d", i)},
		}
	}

	return &nvmlResourceManager{
		resourceManager: resourceManager{
			config:   &spec.Config{},
			resource: "nvidia.com/gpu",
			devices:  devices,
		},
		nvml: fakeLostGPUNvmlLib{uuids: uuids},
	}, healthy
}

// TestAlignedAllocFallsBackWhenDeviceLinkInfoUnavailable checks that pods are
// still admitted when a device that has fallen off the bus makes the topology
// discovery fail. Before the fallback, every allocation on such a node failed,
// including single-device allocations that the healthy devices could satisfy.
func TestAlignedAllocFallsBackWhenDeviceLinkInfoUnavailable(t *testing.T) {
	// The device at index 2 has fallen off the bus, so kubelet offers the other three.
	uuids := []string{"GPU-aaa", "GPU-bbb", "", "GPU-ddd"}

	t.Run("a single device is allocated from the healthy devices", func(t *testing.T) {
		r, healthy := newLostGPUResourceManager(t, uuids)

		// The aligned allocation path is only taken for devices that support it.
		require.True(t, r.Devices().AlignedAllocationSupported())
		require.False(t, AnnotatedIDs(healthy).AnyHasAnnotations())

		allocated, err := r.getPreferredAllocation(healthy, nil, 1)
		require.NoError(t, err)
		require.Len(t, allocated, 1)
		require.Subset(t, healthy, allocated)
	})

	t.Run("required devices are included", func(t *testing.T) {
		r, healthy := newLostGPUResourceManager(t, uuids)

		allocated, err := r.getPreferredAllocation(healthy, []string{"GPU-ddd"}, 2)
		require.NoError(t, err)
		require.Len(t, allocated, 2)
		require.Contains(t, allocated, "GPU-ddd")
		require.Subset(t, healthy, allocated)
	})

	t.Run("the device that has fallen off the bus is never allocated", func(t *testing.T) {
		r, healthy := newLostGPUResourceManager(t, uuids)

		allocated, err := r.getPreferredAllocation(healthy, nil, len(healthy))
		require.NoError(t, err)
		require.ElementsMatch(t, healthy, allocated)
	})

	t.Run("the first device having fallen off the bus is handled", func(t *testing.T) {
		r, healthy := newLostGPUResourceManager(t, []string{"", "GPU-bbb", "GPU-ccc"})

		allocated, err := r.getPreferredAllocation(healthy, nil, 2)
		require.NoError(t, err)
		require.ElementsMatch(t, healthy, allocated)
	})

	t.Run("allocating more devices than are available still fails", func(t *testing.T) {
		r, healthy := newLostGPUResourceManager(t, uuids)

		_, err := r.getPreferredAllocation(healthy, nil, len(healthy)+1)
		require.Error(t, err)
	})
}

// newLostNVLinkResourceManager is newLostGPUResourceManager for the case where
// the handle for the lost device is still served and link discovery instead
// fails while querying its NVLink state.
func newLostNVLinkResourceManager(t *testing.T, uuids []string) (*nvmlResourceManager, []string) {
	t.Helper()

	r, healthy := newLostGPUResourceManager(t, uuids)
	r.nvml = fakeLostNVLinkNvmlLib{fakeLostGPUNvmlLib{uuids: uuids}}

	return r, healthy
}

// TestAlignedAllocFallsBackWhenNVLinkStateUnavailable covers the variant seen in
// the field, where the handle lookup for the lost device succeeds and discovery
// fails later, on a pairwise NVLink state query. The allocation must be
// preserved there too.
func TestAlignedAllocFallsBackWhenNVLinkStateUnavailable(t *testing.T) {
	// The device at index 1 has fallen off the bus, as in the reported trace.
	uuids := []string{"GPU-aaa", "", "GPU-ccc", "GPU-ddd"}

	// The fallback covers every error from the discovery call, so pin down which
	// one this case produces: the allocations below would also pass if discovery
	// failed at an earlier step, such as the handle lookup.
	t.Run("discovery fails while querying the NVLink state of a pair", func(t *testing.T) {
		r, _ := newLostNVLinkResourceManager(t, uuids)

		_, err := gpuallocator.NewDevices(gpuallocator.WithNvmlLib(r.nvml))
		require.ErrorContains(t, err, "error getting NVLink for devices (1, 0)")
		require.ErrorContains(t, err, nvml.ERROR_GPU_IS_LOST.Error())
	})

	t.Run("a single device is allocated from the healthy devices", func(t *testing.T) {
		r, healthy := newLostNVLinkResourceManager(t, uuids)

		require.True(t, r.Devices().AlignedAllocationSupported())
		require.False(t, AnnotatedIDs(healthy).AnyHasAnnotations())

		allocated, err := r.getPreferredAllocation(healthy, nil, 1)
		require.NoError(t, err)
		require.Len(t, allocated, 1)
		require.Subset(t, healthy, allocated)
	})

	t.Run("the device that has fallen off the bus is never allocated", func(t *testing.T) {
		r, healthy := newLostNVLinkResourceManager(t, uuids)

		allocated, err := r.getPreferredAllocation(healthy, nil, len(healthy))
		require.NoError(t, err)
		require.ElementsMatch(t, healthy, allocated)
	})
}

func TestUnalignedAlloc(t *testing.T) {
	testCases := []struct {
		description string
		available   []string
		required    []string
		size        int
		expected    []string
		expectedErr bool
	}{
		{
			description: "required devices come first, the remainder in order",
			available:   []string{"GPU-aaa", "GPU-bbb", "GPU-ccc"},
			required:    []string{"GPU-ccc"},
			size:        2,
			expected:    []string{"GPU-ccc", "GPU-aaa"},
		},
		{
			description: "devices are selected in order when none are required",
			available:   []string{"GPU-aaa", "GPU-bbb"},
			size:        2,
			expected:    []string{"GPU-aaa", "GPU-bbb"},
		},
		{
			description: "nothing is selected for a zero-sized allocation",
			size:        0,
			expected:    []string{},
		},
		{
			description: "a required device is only selected once",
			available:   []string{"GPU-aaa", "GPU-bbb"},
			required:    []string{"GPU-aaa", "GPU-aaa"},
			size:        1,
			expected:    []string{"GPU-aaa"},
		},
		{
			description: "an available device is only selected once",
			available:   []string{"GPU-aaa", "GPU-aaa", "GPU-bbb"},
			size:        2,
			expected:    []string{"GPU-aaa", "GPU-bbb"},
		},
		{
			description: "a required device that is not available is an error",
			available:   []string{"GPU-aaa"},
			required:    []string{"GPU-bbb"},
			size:        1,
			expectedErr: true,
		},
		{
			description: "more required devices than the allocation size is an error",
			available:   []string{"GPU-aaa", "GPU-bbb"},
			required:    []string{"GPU-aaa", "GPU-bbb"},
			size:        1,
			expectedErr: true,
		},
		{
			description: "not enough available devices is an error",
			available:   []string{"GPU-aaa"},
			size:        2,
			expectedErr: true,
		},
		{
			description: "not enough distinct available devices is an error",
			available:   []string{"GPU-aaa", "GPU-aaa"},
			size:        2,
			expectedErr: true,
		},
		{
			description: "a negative allocation size is an error",
			available:   []string{"GPU-aaa"},
			size:        -1,
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			allocated, err := unalignedAlloc(tc.available, tc.required, tc.size)
			if tc.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, allocated)
		})
	}
}
