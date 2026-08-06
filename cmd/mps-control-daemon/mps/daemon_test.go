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
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

// mixedRM produces a ResourceManager whose Devices() contains BOTH replicated
// (annotated) and unreplicated devices — the state that arises when
// sharing.mps.resources[].devices selects only a subset of physical GPUs.
func mixedRM(t *testing.T) *rm.ResourceManagerMock {
	t.Helper()
	// GPU-0 is MPS-shared (2 annotated replicas). GPU-1 is not shared.
	devices := rm.Devices{
		"GPU-0::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-0::0"}, Index: "0", TotalMemory: 40 * 1024 * 1024 * 1024},
		"GPU-0::1": &rm.Device{Device: pluginapi.Device{ID: "GPU-0::1"}, Index: "0", TotalMemory: 40 * 1024 * 1024 * 1024},
		"GPU-1":    &rm.Device{Device: pluginapi.Device{ID: "GPU-1"}, Index: "1", TotalMemory: 40 * 1024 * 1024 * 1024},
	}
	return &rm.ResourceManagerMock{
		DevicesFunc: func() rm.Devices { return devices },
	}
}

func TestSharedDevices_ExcludesUnreplicatedDevices(t *testing.T) {
	d := &Daemon{rm: mixedRM(t)}

	shared := d.sharedDevices()

	// Only the two annotated GPU-0::* entries should survive.
	require.Len(t, shared, 2, "expected only annotated (MPS-shared) devices; got: %v", shared)
	_, ok0 := shared["GPU-0::0"]
	_, ok1 := shared["GPU-0::1"]
	require.True(t, ok0, "expected GPU-0::0 to be in shared devices")
	require.True(t, ok1, "expected GPU-0::1 to be in shared devices")
	_, hasGpu1 := shared["GPU-1"]
	require.False(t, hasGpu1, "expected unreplicated GPU-1 to be excluded from shared devices")
}

func TestPerDevicePinnedDeviceMemoryLimits_OnlyTouchesSharedDevices(t *testing.T) {
	d := &Daemon{rm: mixedRM(t)}

	limits := d.perDevicePinnedDeviceMemoryLimits()

	// GPU-0 (index "0") is MPS-shared with 2 replicas so its per-replica limit
	// should be 40 GiB / 2 = 20480 MiB. GPU-1 is not shared and must NOT get
	// a limit assigned.
	require.Contains(t, limits, "0", "expected a limit for the shared GPU (index 0)")
	require.Equal(t, "20480M", limits["0"])
	require.NotContains(t, limits, "1", "expected no limit for the unreplicated GPU (index 1)")
}

func TestActiveThreadPercentage_ComputedFromSharedDevicesOnly(t *testing.T) {
	d := &Daemon{rm: mixedRM(t)}

	// 2 shared entries / 1 unique GPU = 2 replicas per shared GPU → 100/2 = 50.
	// If the code counted GPU-1 too, it would compute (3 entries / 2 UUIDs) = 1,
	// giving 100/1 = 100 — an incorrect value that also masks the intended cap.
	require.Equal(t, "50", d.activeThreadPercentage())
}
