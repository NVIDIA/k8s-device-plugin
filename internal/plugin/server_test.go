/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	v1 "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/cdi"
	"github.com/NVIDIA/k8s-device-plugin/internal/imex"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

func TestAllocate(t *testing.T) {
	testCases := []struct {
		description      string
		request          *pluginapi.AllocateRequest
		expectedError    error
		expectedResponse *pluginapi.AllocateResponse
	}{
		{
			description: "single device",
			request: &pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{
					{
						DevicesIds: []string{"foo"},
					},
				},
			},
			expectedResponse: &pluginapi.AllocateResponse{
				ContainerResponses: []*pluginapi.ContainerAllocateResponse{
					{
						Envs: map[string]string{
							"NVIDIA_VISIBLE_DEVICES": "foo",
						},
					},
				},
			},
		},
		{
			description: "duplicate device IDs",
			request: &pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{
					{
						DevicesIds: []string{"foo", "bar", "foo"},
					},
				},
			},
			expectedResponse: &pluginapi.AllocateResponse{
				ContainerResponses: []*pluginapi.ContainerAllocateResponse{
					{
						Envs: map[string]string{
							"NVIDIA_VISIBLE_DEVICES": "foo,bar",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			plugin := nvidiaDevicePlugin{
				rm: &rm.ResourceManagerMock{
					ValidateRequestFunc: func(annotatedIDs rm.AnnotatedIDs) error {
						return nil
					},
				},
				config: &v1.Config{
					Flags: v1.Flags{
						CommandLineFlags: v1.CommandLineFlags{
							Plugin: &v1.PluginCommandLineFlags{
								DeviceIDStrategy: ptr(v1.DeviceIDStrategyUUID),
							},
						},
					},
				},
				cdiHandler: &cdi.InterfaceMock{
					QualifiedNameFunc: func(c string, s string) string {
						return "nvidia.com/" + c + "=" + s
					},
				},
				deviceListStrategies: v1.DeviceListStrategies{"envvar": true},
			}

			response, err := plugin.Allocate(context.TODO(), tc.request)
			require.EqualValues(t, tc.expectedError, err)
			require.EqualValues(t, tc.expectedResponse, response)
		})
	}
}

func TestCDIAllocateResponse(t *testing.T) {
	testCases := []struct {
		description          string
		deviceIds            []string
		deviceListStrategies []string
		CDIPrefix            string
		AdditionalCDIDevices []string
		GDSEnabled           bool
		MOFEDEnabled         bool
		imexChannels         []*imex.Channel
		expectedResponse     pluginapi.ContainerAllocateResponse
	}{
		{
			description:          "empty device list has empty response",
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
		},
		{
			description:          "single device is added to annotations",
			deviceIds:            []string{"gpu0"},
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0",
				},
			},
		},
		{
			description:          "single device is added to annotations with custom prefix",
			deviceIds:            []string{"gpu0"},
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "custom.cdi.k8s.io/",
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"custom.cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0",
				},
			},
		},
		{
			description:          "multiple devices are added to annotations",
			deviceIds:            []string{"gpu0", "gpu1"},
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gpu=gpu1",
				},
			},
		},
		{
			description:          "multiple devices are added to annotations with custom prefix",
			deviceIds:            []string{"gpu0", "gpu1"},
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "custom.cdi.k8s.io/",
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"custom.cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gpu=gpu1",
				},
			},
		},
		{
			description:          "mofed devices are selected if configured",
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			AdditionalCDIDevices: []string{"nvidia.com/mofed=all"},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/mofed=all",
				},
			},
		},
		{
			description:          "gds devices are selected if configured",
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			AdditionalCDIDevices: []string{"nvidia.com/gds=all"},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gds=all",
				},
			},
		},
		{
			description:          "gds and mofed devices are included with device ids",
			deviceIds:            []string{"gpu0"},
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			AdditionalCDIDevices: []string{"nvidia.com/gds=all", "nvidia.com/mofed=all"},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gds=all,nvidia.com/mofed=all",
				},
			},
		},
		{
			description:          "imex channel is included with devices",
			deviceListStrategies: []string{"cdi-annotations"},
			CDIPrefix:            "cdi.k8s.io/",
			imexChannels:         []*imex.Channel{{ID: "0"}},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/imex-channel=0",
				},
			},
		},
	}

	for i := range testCases {
		tc := &testCases[i]
		t.Run(tc.description, func(t *testing.T) {
			deviceListStrategies, _ := v1.NewDeviceListStrategies(tc.deviceListStrategies)
			plugin := nvidiaDevicePlugin{
				config: &v1.Config{
					Flags: v1.Flags{
						CommandLineFlags: v1.CommandLineFlags{
							GDSEnabled:   &tc.GDSEnabled,
							MOFEDEnabled: &tc.MOFEDEnabled,
						},
					},
				},
				cdiHandler: &cdi.InterfaceMock{
					QualifiedNameFunc: func(c string, s string) string {
						return "nvidia.com/" + c + "=" + s
					},
					AdditionalDevicesFunc: func() []string {
						return tc.AdditionalCDIDevices
					},
				},
				deviceListStrategies: deviceListStrategies,
				cdiAnnotationPrefix:  tc.CDIPrefix,
				imexChannels:         tc.imexChannels,
			}

			response := pluginapi.ContainerAllocateResponse{}
			err := plugin.updateResponseForCDI(&response, "uuid", tc.deviceIds...)

			require.Nil(t, err)
			require.EqualValues(t, &tc.expectedResponse, &response)
		})
	}
}

func ptr[T any](x T) *T {
	return &x
}

func TestTriggerDeviceListUpdate_Phase2(t *testing.T) {
	plugin := &nvidiaDevicePlugin{
		deviceListUpdate: make(chan struct{}, 1),
	}

	// First trigger should send signal
	plugin.triggerDeviceListUpdate()
	select {
	case <-plugin.deviceListUpdate:
		t.Log("✓ Device list update signal sent")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Signal not sent")
	}

	// Second trigger with pending signal should not block
	plugin.triggerDeviceListUpdate()
	plugin.triggerDeviceListUpdate() // Should not block
	t.Log("✓ triggerDeviceListUpdate doesn't block when signal pending")
}

func TestCheckForRecoveredDevices_Phase2(t *testing.T) {
	// Create persistent device map
	devices := rm.Devices{
		"GPU-0": &rm.Device{
			Device: pluginapi.Device{
				ID:     "GPU-0",
				Health: pluginapi.Unhealthy,
			},
			UnhealthyReason: "XID-79",
		},
		"GPU-1": &rm.Device{
			Device: pluginapi.Device{
				ID:     "GPU-1",
				Health: pluginapi.Unhealthy,
			},
			UnhealthyReason: "XID-48",
		},
		"GPU-2": &rm.Device{
			Device: pluginapi.Device{
				ID:     "GPU-2",
				Health: pluginapi.Healthy,
			},
		},
	}

	// Create mock resource manager with persistent devices
	mockRM := &rm.ResourceManagerMock{
		DevicesFunc: func() rm.Devices {
			return devices
		},
		CheckDeviceHealthFunc: func(d *rm.Device) (bool, error) {
			// GPU-0 recovers, GPU-1 stays unhealthy
			if d.ID == "GPU-0" {
				return true, nil
			}
			return false, fmt.Errorf("still unhealthy")
		},
	}

	plugin := &nvidiaDevicePlugin{
		rm:               mockRM,
		deviceListUpdate: make(chan struct{}, 1),
	}

	plugin.checkForRecoveredDevices()

	// Verify GPU-0 recovered
	gpu0 := devices["GPU-0"]
	require.Equal(t, pluginapi.Healthy, gpu0.Health, "GPU-0 should be healthy")
	require.Equal(t, "", gpu0.UnhealthyReason)
	t.Logf("✓ GPU-0 recovered: Health=%s, Reason=%s", gpu0.Health, gpu0.UnhealthyReason)

	// Verify GPU-1 still unhealthy
	gpu1 := devices["GPU-1"]
	require.Equal(t, pluginapi.Unhealthy, gpu1.Health, "GPU-1 should still be unhealthy")
	require.Equal(t, 1, gpu1.RecoveryAttempts, "GPU-1 recovery attempts should increment")
	t.Logf("✓ GPU-1 still unhealthy: attempts=%d", gpu1.RecoveryAttempts)

	// Verify GPU-2 unchanged
	gpu2 := devices["GPU-2"]
	require.Equal(t, pluginapi.Healthy, gpu2.Health)
	require.Equal(t, 0, gpu2.RecoveryAttempts, "Healthy device shouldn't be probed")
	t.Log("✓ GPU-2 unchanged (was already healthy)")

	// Verify deviceListUpdate was triggered
	select {
	case <-plugin.deviceListUpdate:
		t.Log("✓ Device list update triggered for recovery")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Device list update not triggered")
	}
}
