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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
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

func TestEnvVarsScopeToSharedGPUs(t *testing.T) {
	m := mixedRM(t)
	m.ResourceFunc = func() spec.ResourceName { return "nvidia.com/gpu" }
	d := &Daemon{rm: m, root: Root("/mps")}

	// Only the shared GPU-0 should be visible; the unreplicated GPU-1 must not.
	require.Equal(t, "GPU-0", d.EnvVars()["CUDA_VISIBLE_DEVICES"])
}

func TestPerDeviceMemoryLimits_RemapToVisibleOrdinal(t *testing.T) {
	// A single shared GPU at global index "3". Under CUDA_VISIBLE_DEVICES it
	// becomes ordinal 0, so the limit must be keyed "0", not the global "3".
	devices := rm.Devices{
		"GPU-3::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-3::0"}, Index: "3", TotalMemory: 40 * 1024 * 1024 * 1024},
		"GPU-3::1": &rm.Device{Device: pluginapi.Device{ID: "GPU-3::1"}, Index: "3", TotalMemory: 40 * 1024 * 1024 * 1024},
	}
	d := &Daemon{rm: &rm.ResourceManagerMock{DevicesFunc: func() rm.Devices { return devices }}}

	require.Equal(t, []string{"GPU-3"}, d.sharedVisibleUUIDs())
	require.Equal(t, map[string]string{"0": "20480M"}, d.perDevicePinnedDeviceMemoryLimits())
}

func TestPerDeviceMemoryLimits_MultipleGPUsUseVisibleOrdinals(t *testing.T) {
	// GPU-a: 20 GiB / 2 replicas = 10240M. GPU-b: 80 GiB / 4 replicas = 20480M.
	devices := rm.Devices{
		"GPU-a::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-a::0"}, Index: "0", TotalMemory: 20 * 1024 * 1024 * 1024},
		"GPU-a::1": &rm.Device{Device: pluginapi.Device{ID: "GPU-a::1"}, Index: "0", TotalMemory: 20 * 1024 * 1024 * 1024},
		"GPU-b::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-b::0"}, Index: "1", TotalMemory: 80 * 1024 * 1024 * 1024},
		"GPU-b::1": &rm.Device{Device: pluginapi.Device{ID: "GPU-b::1"}, Index: "1", TotalMemory: 80 * 1024 * 1024 * 1024},
		"GPU-b::2": &rm.Device{Device: pluginapi.Device{ID: "GPU-b::2"}, Index: "1", TotalMemory: 80 * 1024 * 1024 * 1024},
		"GPU-b::3": &rm.Device{Device: pluginapi.Device{ID: "GPU-b::3"}, Index: "1", TotalMemory: 80 * 1024 * 1024 * 1024},
	}
	d := &Daemon{rm: &rm.ResourceManagerMock{DevicesFunc: func() rm.Devices { return devices }}}

	require.Equal(t, []string{"GPU-a", "GPU-b"}, d.sharedVisibleUUIDs())
	limits := d.perDevicePinnedDeviceMemoryLimits()
	require.Equal(t, "10240M", limits["0"], "GPU-a at ordinal 0")
	require.Equal(t, "20480M", limits["1"], "GPU-b at ordinal 1")
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
