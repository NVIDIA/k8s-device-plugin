/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

// TestGetMPSOptionsSkipsUnsharedResource: the daemon manager creates no daemon
// for a resource whose devices are not replicated (no annotated IDs), so the
// plugin must not enable MPS for it — otherwise it waits for a daemon that never
// exists. A resource with a shared device still enables MPS.
func TestGetMPSOptionsSkipsUnsharedResource(t *testing.T) {
	newOptions := func() *options {
		return &options{
			config: &spec.Config{
				Sharing: spec.Sharing{
					MPS: &spec.ReplicatedResources{
						Resources: []spec.ReplicatedResource{{Name: "nvidia.com/gpu", Replicas: 2}},
					},
				},
			},
		}
	}

	t.Run("unshared resource disables MPS", func(t *testing.T) {
		o := newOptions()
		mgr := &rm.ResourceManagerMock{
			DevicesFunc: func() rm.Devices {
				return rm.Devices{"GPU0": &rm.Device{Device: pluginapi.Device{ID: "GPU0"}}}
			},
		}
		opts, err := o.getMPSOptions(mgr)
		require.NoError(t, err)
		require.False(t, opts.enabled, "MPS must be disabled when no device is shared")
	})

	t.Run("shared resource enables MPS", func(t *testing.T) {
		mpsRoot := "/mps"
		o := newOptions()
		o.config.Flags.MpsRoot = &mpsRoot
		mgr := &rm.ResourceManagerMock{
			ResourceFunc: func() spec.ResourceName { return "nvidia.com/gpu" },
			DevicesFunc: func() rm.Devices {
				id := string(rm.NewAnnotatedID("GPU0", 0))
				return rm.Devices{id: &rm.Device{Device: pluginapi.Device{ID: id}}}
			},
		}
		opts, err := o.getMPSOptions(mgr)
		require.NoError(t, err)
		require.True(t, opts.enabled, "MPS must be enabled when a device is shared")
	})
}

// TestActiveThreadPercentagePerClient checks the per-client thread percentage is
// derived from the allocated devices' replica counts, using the most-shared GPU
// when a container spans different counts.
func TestActiveThreadPercentagePerClient(t *testing.T) {
	devices := rm.Devices{
		"GPU-0::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-0::0"}, Replicas: 2},
		"GPU-0::1": &rm.Device{Device: pluginapi.Device{ID: "GPU-0::1"}, Replicas: 2},
		"GPU-1::0": &rm.Device{Device: pluginapi.Device{ID: "GPU-1::0"}, Replicas: 4},
	}
	mgr := &rm.ResourceManagerMock{DevicesFunc: func() rm.Devices { return devices }}
	m := &mpsOptions{enabled: true, daemon: mps.NewDaemon(mgr, mps.ContainerRoot)}

	require.Equal(t, "50", m.activeThreadPercentage([]string{"GPU-0::0"}), "2-replica GPU → 100/2")
	require.Equal(t, "25", m.activeThreadPercentage([]string{"GPU-1::0"}), "4-replica GPU → 100/4")
	require.Equal(t, "25", m.activeThreadPercentage([]string{"GPU-0::0", "GPU-1::0"}), "mixed → most-shared GPU (100/4)")
	require.Equal(t, "", m.activeThreadPercentage([]string{"unknown"}), "unknown device → no value")
}
