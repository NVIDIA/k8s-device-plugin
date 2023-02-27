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

package main

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
		GDSEnabled       bool
		MOFEDEnabled     bool
		expectedResponse pluginapi.ContainerAllocateResponse
	}{
		{
			description: "empty device list has empty response",
		},
		{
			description: "single device is added to annotations",
			deviceIds:   []string{"gpu0"},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0",
				},
				Envs: map[string]string{
					"NVIDIA_VISIBLE_DEVICES": "void",
				},
			},
		},
		{
			description: "multiple devices are added to annotations",
			deviceIds:   []string{"gpu0", "gpu1"},
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gpu=gpu1",
				},
				Envs: map[string]string{
					"NVIDIA_VISIBLE_DEVICES": "void",
				},
			},
		},
		{
			description:  "mofed devices are selected if configured",
			MOFEDEnabled: true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/mofed=all",
				},
				Envs: map[string]string{
					"NVIDIA_VISIBLE_DEVICES": "void",
				},
			},
		},
		{
			description: "gds devices are selected if configured",
			GDSEnabled:  true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gds=all",
				},
				Envs: map[string]string{
					"NVIDIA_VISIBLE_DEVICES": "void",
				},
			},
		},
		{
			description:  "gds and mofed devices are included with device ids",
			deviceIds:    []string{"gpu0"},
			GDSEnabled:   true,
			MOFEDEnabled: true,
			expectedResponse: pluginapi.ContainerAllocateResponse{
				Annotations: map[string]string{
					"cdi.k8s.io/nvidia-device-plugin_uuid": "nvidia.com/gpu=gpu0,nvidia.com/gds=all,nvidia.com/mofed=all",
				},
				Envs: map[string]string{
					"NVIDIA_VISIBLE_DEVICES": "void",
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
				cdi: &cdi.InterfaceMock{
					QualifiedNameFunc: func(s string) string {
						return "nvidia.com/gpu=" + s
					},
				},
				deviceListEnvvar: "NVIDIA_VISIBLE_DEVICES",
			}

			response := plugin.getAllocateResponseForCDIAnnotations("uuid", tc.deviceIds)

			require.EqualValues(t, &tc.expectedResponse, response)
		})
	}
}
