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

	v1 "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/cdi"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestCDIAllocateResponse(t *testing.T) {
	testCases := []struct {
		description      string
		deviceIds        []string
		CDIEnabled       bool
		GDSEnabled       bool
		MOFEDEnabled     bool
		expectedResponse pluginapi.ContainerAllocateResponse
	}{
		{
			description: "empty device list has empty response",
			CDIEnabled:  true,
		},
		{
			description: "CDI disabled has empty response",
			deviceIds:   []string{"gpu0"},
			CDIEnabled:  false,
		},
		{
			description: "single device is added to annotations",
			deviceIds:   []string{"gpu0"},
			CDIEnabled:  true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0",
				},
			},
		},
		{
			description: "multiple devices are added to annotations",
			deviceIds:   []string{"gpu0", "gpu1"},
			CDIEnabled:  true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gpu=gpu1",
				},
			},
		},
		{
			description:  "mofed devices are selected if configured",
			CDIEnabled:   true,
			MOFEDEnabled: true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/mofed=all",
				},
			},
		},
		{
			description: "gds devices are selected if configured",
			CDIEnabled:  true,
			GDSEnabled:  true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gds=all",
				},
			},
		},
		{
			description:  "gds and mofed devices are included with device ids",
			deviceIds:    []string{"gpu0"},
			CDIEnabled:   true,
			GDSEnabled:   true,
			MOFEDEnabled: true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gds=all,nvidia.com/mofed=all",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			plugin := NvidiaDevicePlugin{
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
				},
				cdiEnabled: tc.CDIEnabled,
			}

			response := plugin.getAllocateResponseForCDI("uuid", tc.deviceIds)

			require.EqualValues(t, &tc.expectedResponse, &response)
		})
	}
}
