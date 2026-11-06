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
	"errors"
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

func TestGetPreferredAllocationMarksUnhealthyDevices(t *testing.T) {
	testCases := []struct {
		description       string
		err               error
		expectedError     string
		expectedUnhealthy []string
	}{
		{
			description:       "devices that cannot be queried are marked unhealthy",
			err:               fmt.Errorf("unable to get device link information: %w", &rm.UnhealthyDevicesError{IDs: []string{"GPU-0"}, Err: errors.New("GPU requires reset")}),
			expectedError:     "error getting list of preferred allocation devices: unable to get device link information: GPU requires reset; devices that cannot be queried: [GPU-0]; marking them unhealthy",
			expectedUnhealthy: []string{"GPU-0"},
		},
		{
			description:   "other errors do not mark devices unhealthy",
			err:           errors.New("not enough available devices to satisfy allocation"),
			expectedError: "error getting list of preferred allocation devices: not enough available devices to satisfy allocation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			plugin := nvidiaDevicePlugin{
				rm: &rm.ResourceManagerMock{
					DevicesFunc: func() rm.Devices {
						return rm.Devices{
							"GPU-0": {Device: pluginapi.Device{ID: "GPU-0"}},
							"GPU-1": {Device: pluginapi.Device{ID: "GPU-1"}},
						}
					},
					GetPreferredAllocationFunc: func([]string, []string, int) ([]string, error) {
						return nil, tc.err
					},
				},
				health: make(chan *rm.Device),
				stop:   make(chan any),
			}
			defer close(plugin.stop)

			_, err := plugin.GetPreferredAllocation(context.TODO(), &pluginapi.PreferredAllocationRequest{
				ContainerRequests: []*pluginapi.ContainerPreferredAllocationRequest{{}},
			})
			require.EqualError(t, err, tc.expectedError)

			// The test stands in for ListAndWatch, the only receiver on the channel.
			var unhealthy []string
			for range tc.expectedUnhealthy {
				select {
				case d := <-plugin.health:
					unhealthy = append(unhealthy, d.ID)
				case <-time.After(time.Second):
					t.Fatal("timed out waiting for an unhealthy device")
				}
			}
			select {
			case d := <-plugin.health:
				unhealthy = append(unhealthy, d.ID)
			case <-time.After(100 * time.Millisecond):
			}
			require.Equal(t, tc.expectedUnhealthy, unhealthy)
		})
	}
}
