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
	"net"
	"os"
	"path/filepath"
	"testing"

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
								DeviceIDStrategy: new(v1.DeviceIDStrategyUUID),
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

func TestNewSessionID(t *testing.T) {
	first, err := NewSessionID()
	require.NoError(t, err)
	require.Regexp(t, "^[0-9a-f]{8}$", first)

	second, err := NewSessionID()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
}

func TestGetPluginSocketPath(t *testing.T) {
	testCases := []struct {
		description string
		resource    v1.ResourceName
		expected    string
	}{
		{
			description: "full GPU",
			resource:    "nvidia.com/gpu",
			expected:    filepath.Join(pluginapi.DevicePluginPath, "nvidia-gpu-3f9a1c07.sock"),
		},
		{
			description: "MIG device",
			resource:    "nvidia.com/mig-1g.10gb.me",
			expected:    filepath.Join(pluginapi.DevicePluginPath, "nvidia-mig-1g.10gb.me-3f9a1c07.sock"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expected, getPluginSocketPath(tc.resource, "3f9a1c07"))
		})
	}
}

func TestRemoveStaleSockets(t *testing.T) {
	testCases := []struct {
		description string
		pluginName  string
		current     string
		stale       []string
		live        []string
		expected    []string
	}{
		{
			description: "stale sockets from earlier sessions are removed",
			pluginName:  "nvidia-gpu",
			current:     "nvidia-gpu-3f9a1c07.sock",
			stale:       []string{"nvidia-gpu-a1b2c3d4.sock", "nvidia-gpu-e81b4f3a.sock"},
		},
		{
			description: "live sockets are kept",
			pluginName:  "nvidia-gpu",
			current:     "nvidia-gpu-3f9a1c07.sock",
			live:        []string{"nvidia-gpu-a1b2c3d4.sock"},
			expected:    []string{"nvidia-gpu-a1b2c3d4.sock"},
		},
		{
			description: "current socket is kept",
			pluginName:  "nvidia-gpu",
			current:     "nvidia-gpu-3f9a1c07.sock",
			stale:       []string{"nvidia-gpu-3f9a1c07.sock"},
			expected:    []string{"nvidia-gpu-3f9a1c07.sock"},
		},
		{
			description: "sockets of other resources are kept",
			pluginName:  "nvidia-mig-1g.10gb",
			current:     "nvidia-mig-1g.10gb-3f9a1c07.sock",
			stale: []string{
				"nvidia-mig-1g.10gb.me-a1b2c3d4.sock",
				"nvidia-mig-1g.10gb-a-a1b2c3d4.sock",
				"nvidia-gpu-a1b2c3d4.sock",
			},
			expected: []string{
				"nvidia-mig-1g.10gb.me-a1b2c3d4.sock",
				"nvidia-mig-1g.10gb-a-a1b2c3d4.sock",
				"nvidia-gpu-a1b2c3d4.sock",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// t.TempDir() can exceed the length limit for unix socket paths on macOS.
			dir, err := os.MkdirTemp("", "dp")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			for _, name := range tc.stale {
				createSocket(t, filepath.Join(dir, name), false)
			}
			for _, name := range tc.live {
				createSocket(t, filepath.Join(dir, name), true)
			}

			removeStaleSockets(dir, tc.pluginName, filepath.Join(dir, tc.current))

			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			var remaining []string
			for _, entry := range entries {
				remaining = append(remaining, entry.Name())
			}
			require.ElementsMatch(t, tc.expected, remaining)
		})
	}
}

// createSocket creates a unix socket at path. A live socket keeps listening
// until the test ends, while a stale one is closed without being unlinked, as
// happens when a plugin exits without calling Stop.
func createSocket(t *testing.T, path string, live bool) {
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	if live {
		t.Cleanup(func() { _ = listener.Close() })
		return
	}
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, listener.Close())
}
