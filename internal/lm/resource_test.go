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

package lm

import (
	"testing"

	rt "github.com/NVIDIA/gpu-feature-discovery/internal/resource/testing"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/stretchr/testify/require"
)

func TestGPUResourceLabeler(t *testing.T) {
	device := rt.NewFullGPU()

	testCases := []struct {
		description    string
		count          int
		timeSlicing    spec.TimeSlicing
		expectedLabels Labels
	}{
		{
			description: "zero count returns empty",
		},
		{
			description: "no sharing",
			count:       1,
			expectedLabels: Labels{
				"nvidia.com/gpu.count":         "1",
				"nvidia.com/gpu.replicas":      "1",
				"nvidia.com/gpu.memory":        "300",
				"nvidia.com/gpu.product":       "MOCKMODEL",
				"nvidia.com/gpu.family":        "ampere",
				"nvidia.com/gpu.compute.major": "8",
				"nvidia.com/gpu.compute.minor": "0",
			},
		},
		{
			description: "sharing ignores non-matching resource",
			count:       1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/not-gpu",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/gpu.count":         "1",
				"nvidia.com/gpu.replicas":      "1",
				"nvidia.com/gpu.memory":        "300",
				"nvidia.com/gpu.product":       "MOCKMODEL",
				"nvidia.com/gpu.family":        "ampere",
				"nvidia.com/gpu.compute.major": "8",
				"nvidia.com/gpu.compute.minor": "0",
			},
		},
		{
			description: "shared appends suffix and doubles count",
			count:       1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/gpu",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/gpu.count":         "1",
				"nvidia.com/gpu.replicas":      "2",
				"nvidia.com/gpu.memory":        "300",
				"nvidia.com/gpu.product":       "MOCKMODEL-SHARED",
				"nvidia.com/gpu.family":        "ampere",
				"nvidia.com/gpu.compute.major": "8",
				"nvidia.com/gpu.compute.minor": "0",
			},
		},
		{
			description: "renamed does not append suffix and doubles count",
			count:       1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/gpu",
						Rename:   "nvidia.com/gpu.shared",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/gpu.count":         "1",
				"nvidia.com/gpu.replicas":      "2",
				"nvidia.com/gpu.memory":        "300",
				"nvidia.com/gpu.product":       "MOCKMODEL",
				"nvidia.com/gpu.family":        "ampere",
				"nvidia.com/gpu.compute.major": "8",
				"nvidia.com/gpu.compute.minor": "0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			config := &spec.Config{
				Sharing: spec.Sharing{
					TimeSlicing: tc.timeSlicing,
				},
			}
			l, err := NewGPUResourceLabeler(config, device, tc.count)
			require.NoError(t, err)

			labels, err := l.Labels()
			require.NoError(t, err)

			require.EqualValues(t, tc.expectedLabels, labels)
		})
	}

}

func TestMigResourceLabeler(t *testing.T) {

	device := rt.NewMigDevice(1, 2, 300)
	rt.NewMigEnabledDevice(device)

	testCases := []struct {
		description    string
		resourceName   spec.ResourceName
		count          int
		timeSlicing    spec.TimeSlicing
		expectedLabels Labels
	}{
		{
			description: "zero count returns empty",
		},
		{
			description:  "no sharing",
			resourceName: "nvidia.com/gpu",
			count:        1,
			expectedLabels: Labels{
				"nvidia.com/gpu.count":           "1",
				"nvidia.com/gpu.replicas":        "1",
				"nvidia.com/gpu.memory":          "300",
				"nvidia.com/gpu.product":         "MOCKMODEL-MIG-1g.300gb",
				"nvidia.com/gpu.multiprocessors": "0",
				"nvidia.com/gpu.slices.gi":       "1",
				"nvidia.com/gpu.slices.ci":       "2",
				"nvidia.com/gpu.engines.copy":    "0",
				"nvidia.com/gpu.engines.decoder": "0",
				"nvidia.com/gpu.engines.encoder": "0",
				"nvidia.com/gpu.engines.jpeg":    "0",
				"nvidia.com/gpu.engines.ofa":     "0",
			},
		},
		{
			description:  "shared appends suffix and doubles count",
			resourceName: "nvidia.com/gpu",
			count:        1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/gpu",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/gpu.count":           "1",
				"nvidia.com/gpu.replicas":        "2",
				"nvidia.com/gpu.memory":          "300",
				"nvidia.com/gpu.product":         "MOCKMODEL-MIG-1g.300gb-SHARED",
				"nvidia.com/gpu.multiprocessors": "0",
				"nvidia.com/gpu.slices.gi":       "1",
				"nvidia.com/gpu.slices.ci":       "2",
				"nvidia.com/gpu.engines.copy":    "0",
				"nvidia.com/gpu.engines.decoder": "0",
				"nvidia.com/gpu.engines.encoder": "0",
				"nvidia.com/gpu.engines.jpeg":    "0",
				"nvidia.com/gpu.engines.ofa":     "0",
			},
		},
		{
			description:  "renamed does not append suffix and doubles count",
			resourceName: "nvidia.com/gpu",
			count:        1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/gpu",
						Rename:   "nvidia.com/gpu.shared",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/gpu.count":           "1",
				"nvidia.com/gpu.replicas":        "2",
				"nvidia.com/gpu.memory":          "300",
				"nvidia.com/gpu.product":         "MOCKMODEL-MIG-1g.300gb",
				"nvidia.com/gpu.multiprocessors": "0",
				"nvidia.com/gpu.slices.gi":       "1",
				"nvidia.com/gpu.slices.ci":       "2",
				"nvidia.com/gpu.engines.copy":    "0",
				"nvidia.com/gpu.engines.decoder": "0",
				"nvidia.com/gpu.engines.encoder": "0",
				"nvidia.com/gpu.engines.jpeg":    "0",
				"nvidia.com/gpu.engines.ofa":     "0",
			},
		},
		{
			description:  "mig mixed appends shared",
			resourceName: "nvidia.com/mig-1g.1gb",
			count:        1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/gpu",
						Rename:   "nvidia.com/gpu.shared",
						Replicas: 2,
					},
					{
						Name:     "nvidia.com/mig-1g.1gb",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/mig-1g.1gb.count":           "1",
				"nvidia.com/mig-1g.1gb.replicas":        "2",
				"nvidia.com/mig-1g.1gb.memory":          "300",
				"nvidia.com/mig-1g.1gb.product":         "MOCKMODEL-MIG-1g.300gb-SHARED",
				"nvidia.com/mig-1g.1gb.multiprocessors": "0",
				"nvidia.com/mig-1g.1gb.slices.gi":       "1",
				"nvidia.com/mig-1g.1gb.slices.ci":       "2",
				"nvidia.com/mig-1g.1gb.engines.copy":    "0",
				"nvidia.com/mig-1g.1gb.engines.decoder": "0",
				"nvidia.com/mig-1g.1gb.engines.encoder": "0",
				"nvidia.com/mig-1g.1gb.engines.jpeg":    "0",
				"nvidia.com/mig-1g.1gb.engines.ofa":     "0",
			},
		},
		{
			description:  "mig mixed rename does not append",
			resourceName: "nvidia.com/mig-1g.1gb",
			count:        1,
			timeSlicing: spec.TimeSlicing{
				Resources: []spec.ReplicatedResource{
					{
						Name:     "nvidia.com/mig-1g.1gb",
						Rename:   "nvidia.com/mig-1g.1gb.shared",
						Replicas: 2,
					},
				},
			},
			expectedLabels: Labels{
				"nvidia.com/mig-1g.1gb.count":           "1",
				"nvidia.com/mig-1g.1gb.replicas":        "2",
				"nvidia.com/mig-1g.1gb.memory":          "300",
				"nvidia.com/mig-1g.1gb.product":         "MOCKMODEL-MIG-1g.300gb",
				"nvidia.com/mig-1g.1gb.multiprocessors": "0",
				"nvidia.com/mig-1g.1gb.slices.gi":       "1",
				"nvidia.com/mig-1g.1gb.slices.ci":       "2",
				"nvidia.com/mig-1g.1gb.engines.copy":    "0",
				"nvidia.com/mig-1g.1gb.engines.decoder": "0",
				"nvidia.com/mig-1g.1gb.engines.encoder": "0",
				"nvidia.com/mig-1g.1gb.engines.jpeg":    "0",
				"nvidia.com/mig-1g.1gb.engines.ofa":     "0",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			config := &spec.Config{
				Sharing: spec.Sharing{
					TimeSlicing: tc.timeSlicing,
				},
			}
			l, err := NewMIGResourceLabeler(tc.resourceName, config, device, tc.count)
			require.NoError(t, err)

			labels, err := l.Labels()
			require.NoError(t, err)

			require.EqualValues(t, tc.expectedLabels, labels)
		})
	}
}
