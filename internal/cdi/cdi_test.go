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

package cdi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"tags.cncf.io/container-device-interface/specs-go"
)

func TestRootTransformPreservesDevNullMounts(t *testing.T) {
	testCases := []struct {
		description      string
		handler          cdiHandler
		expectedLibMount string
	}{
		{
			description: "container driver root is /",
			handler: cdiHandler{
				driverRoot:       "/",
				targetDriverRoot: "/run/nvidia/driver",
				devRoot:          "/",
				targetDevRoot:    "/run/nvidia/driver",
			},
			expectedLibMount: "/run/nvidia/driver/usr/lib/libcuda.so.1",
		},
		{
			description: "container dev root is / with distinct target dev root",
			handler: cdiHandler{
				driverRoot:       "/driver-root",
				targetDriverRoot: "/run/nvidia/driver",
				devRoot:          "/",
				targetDevRoot:    "/run/nvidia/dev-root",
			},
			expectedLibMount: "/run/nvidia/driver/usr/lib/libcuda.so.1",
		},
		{
			description: "host driver root mounted at /driver-root",
			handler: cdiHandler{
				driverRoot:       "/driver-root",
				targetDriverRoot: "/",
				devRoot:          "/driver-root",
				targetDevRoot:    "/",
			},
			expectedLibMount: "/usr/lib/libcuda.so.1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			libMount := "/usr/lib/libcuda.so.1"
			if tc.handler.driverRoot != "/" {
				libMount = tc.handler.driverRoot + libMount
			}
			spec := &specs.Spec{
				Devices: []specs.Device{
					{
						Name: "0",
						ContainerEdits: specs.ContainerEdits{
							Mounts: []*specs.Mount{
								{HostPath: "/dev/null", ContainerPath: "/var/run/nvidia-container-devices/void"},
							},
						},
					},
				},
				ContainerEdits: specs.ContainerEdits{
					Mounts: []*specs.Mount{
						{HostPath: libMount, ContainerPath: "/usr/lib/libcuda.so.1"},
						{HostPath: "/dev/null", ContainerPath: "/var/run/nvidia-container-devices/void"},
					},
				},
			}

			transformer := devNullPreservingTransformer{tc.handler.getRootTransformer()}
			require.NoError(t, transformer.Transform(spec))

			require.Equal(t, tc.expectedLibMount, spec.ContainerEdits.Mounts[0].HostPath)
			require.Equal(t, "/dev/null", spec.ContainerEdits.Mounts[1].HostPath)
			require.Equal(t, "/dev/null", spec.Devices[0].ContainerEdits.Mounts[0].HostPath)
		})
	}
}
