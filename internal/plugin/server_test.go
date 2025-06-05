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
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/cdi"
	"github.com/NVIDIA/k8s-device-plugin/internal/imex"
)

func TestCDIAllocateResponse(t *testing.T) {
	testCases := []struct {
		description          string
		deviceIds            []string
		deviceListStrategies []string
		CDIPrefix            string
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
			MOFEDEnabled:         true,
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
			GDSEnabled:           true,
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
			GDSEnabled:           true,
			MOFEDEnabled:         true,
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
		tc := testCases[i]
		t.Run(tc.description, func(t *testing.T) {
			deviceListStrategies, _ := spec.NewDeviceListStrategies(tc.deviceListStrategies)
			plugin := nvidiaDevicePlugin{
				config: &spec.Config{
					Flags: spec.Flags{
						CommandLineFlags: spec.CommandLineFlags{
							GDSEnabled:   &tc.GDSEnabled,
							MOFEDEnabled: &tc.MOFEDEnabled,
						},
					},
				},
				cdiHandler: &cdi.InterfaceMock{
					QualifiedNameFunc: func(c string, s string) string {
						return "nvidia.com/" + c + "=" + s
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

func TestGetPreferredAllocationStrategies(t *testing.T) {
	testCases := []struct {
		description string
		input       *spec.StringOrSliceFlag
		expected    []string
	}{
		{
			description: "nil strategy returns auto",
			input:       nil,
			expected:    []string{"auto"},
		},
		{
			description: "empty strategy returns auto",
			input:       &spec.StringOrSliceFlag{},
			expected:    []string{"auto"},
		},
		{
			description: "strategy with none returns nil",
			input:       &spec.StringOrSliceFlag{"none"},
			expected:    nil,
		},
		{
			description: "single strategy returns that strategy",
			input:       &spec.StringOrSliceFlag{"guaranteed-single"},
			expected:    []string{"guaranteed-single"},
		},
		{
			description: "multiple strategies returns all strategies",
			input:       &spec.StringOrSliceFlag{"guaranteed-single", "best-effort"},
			expected:    []string{"guaranteed-single", "best-effort"},
		},
		{
			description: "none among other strategies still returns nil",
			input:       &spec.StringOrSliceFlag{"guaranteed-single", "none", "best-effort"},
			expected:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			o := &options{
				config: &spec.Config{
					Flags: spec.Flags{
						CommandLineFlags: spec.CommandLineFlags{
							Plugin: &spec.PluginCommandLineFlags{
								PreferredAllocationStrategy: tc.input,
							},
						},
					},
				},
			}
			result := o.getPreferredAllocationStrategies()
			require.Equal(t, tc.expected, result)
		})
	}
}
