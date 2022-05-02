/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplicaDeviceRef(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "0",
			expected: "gpuIndex",
		},
		{
			input:    "0:0",
			expected: "migIndex",
		},
		{
			input:    "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c",
			expected: "uuid",
		},
		{
			input:    "MIG-3eb87630-93d5-b2b6-b8ff-9b359caf4ee2",
			expected: "uuid",
		},
		{
			input:    "MIG-GPU-662077db-fa3f-0d8f-9502-21ab0ef058a2/10/0",
			expected: "uuid",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			switch tc.expected {
			case "gpuIndex":
				require.True(t, ReplicaDeviceRef(tc.input).IsGPUIndex())
				require.False(t, ReplicaDeviceRef(tc.input).IsMigIndex())
				require.False(t, ReplicaDeviceRef(tc.input).IsUUID())
			case "migIndex":
				require.False(t, ReplicaDeviceRef(tc.input).IsGPUIndex())
				require.True(t, ReplicaDeviceRef(tc.input).IsMigIndex())
				require.False(t, ReplicaDeviceRef(tc.input).IsUUID())
			case "uuid":
				require.False(t, ReplicaDeviceRef(tc.input).IsGPUIndex())
				require.False(t, ReplicaDeviceRef(tc.input).IsMigIndex())
				require.True(t, ReplicaDeviceRef(tc.input).IsUUID())
			}
		})
	}
}

func TestMarshalReplicaDevices(t *testing.T) {
	testCases := []struct {
		input  ReplicaDevices
		output string
		err    bool
	}{
		{
			input: ReplicaDevices{},
			err:   true,
		},
		{
			input: ReplicaDevices{
				All: true,
			},
			output: `"all"`,
		},
		{
			input: ReplicaDevices{
				Count: 2,
			},
			output: `2`,
		},
		{
			input: ReplicaDevices{
				List: []ReplicaDeviceRef{"0", "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"},
			},
			output: `["0", "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output, err := tc.input.MarshalJSON()
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.output, string(output))
		})
	}
}

func TestUnmarshalReplicaDevices(t *testing.T) {
	testCases := []struct {
		input  string
		output ReplicaDevices
		err    bool
	}{
		{
			input: ``,
			err:   true,
		},
		{
			input: `"not-all"`,
			err:   true,
		},
		{
			input: `-2`,
			err:   true,
		},
		{
			input: `2.0`,
			err:   true,
		},
		{
			input: `[-1]`,
			err:   true,
		},
		{
			input: `["-1"]`,
			err:   true,
		},
		{
			input: `["invalid-UUID"]`,
			err:   true,
		},
		{
			input: `["GPU-UUID"]`,
			err:   true,
		},
		{
			input: `["MIG-UUID"]`,
			err:   true,
		},
		{
			input: `["MIG-GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
			err:   true,
		},
		{
			input: `"all"`,
			output: ReplicaDevices{
				All: true,
			},
		},
		{
			input: `2`,
			output: ReplicaDevices{
				Count: 2,
			},
		},
		{
			input: `[0]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"0"},
			},
		},
		{
			input: `["0"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"0"},
			},
		},
		{
			input: `["0:0"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"0:0"},
			},
		},
		{
			input: `["GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"},
			},
		},
		{
			input: `["MIG-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"MIG-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"},
			},
		},
		{
			input: `["MIG-GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c/0/0"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"MIG-GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c/0/0"},
			},
		},
		{
			input: `[0, "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"0", "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"},
			},
		},
		{
			input: `["0", "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"]`,
			output: ReplicaDevices{
				List: []ReplicaDeviceRef{"0", "0:0", "GPU-4cf8db2d-06c0-7d70-1a51-e59b25b2c16c"},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var output ReplicaDevices
			err := output.UnmarshalJSON([]byte(tc.input))
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestUnmarshalReplicaResource(t *testing.T) {
	testCases := []struct {
		input  string
		output ReplicaResource
		err    bool
	}{
		{
			input: ``,
			err:   true,
		},
		{
			input: `{}`,
			err:   true,
		},
		{
			input: `{
				"name": "valid",
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"devices": "all",
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"devices": "all",
				"rename": "valid-shared",
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"devices": "all",
				"replicas": 2
			}`,
			output: ReplicaResource{
				Name:     ResourceName("valid"),
				Devices:  ReplicaDevices{All: true},
				Replicas: 2,
			},
		},
		{
			input: `{
				"name": "valid",
				"devices": "all",
				"replicas": 2,
				"rename": "valid"
			}`,
			output: ReplicaResource{
				Name:     ResourceName("valid"),
				Devices:  ReplicaDevices{All: true},
				Replicas: 2,
				Rename:   "valid",
			},
		},
		{
			input: `{
				"name": "valid",
				"replicas": -1,
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"replicas": 0,
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"replicas": 2
			}`,
			output: ReplicaResource{
				Name:     ResourceName("valid"),
				Devices:  ReplicaDevices{All: true},
				Replicas: 2,
			},
		},
		{
			input: `{
				"name": "valid",
				"replicas": 2,
				"rename": "valid-shared"
			}`,
			output: ReplicaResource{
				Name:     ResourceName("valid"),
				Devices:  ReplicaDevices{All: true},
				Replicas: 2,
				Rename:   "valid-shared",
			},
		},
		{
			input: `{
				"name": "$invalid$",
				"replicas": 2,
				"rename": "valid-shared"
			}`,
			err: true,
		},
		{
			input: `{
				"name": "valid",
				"replicas": 2,
				"rename": "$invalid$"
			}`,
			err: true,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var output ReplicaResource
			err := output.UnmarshalJSON([]byte(tc.input))
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestUnmarshalTimeSlicing(t *testing.T) {
	testCases := []struct {
		input  string
		output TimeSlicing
		err    bool
	}{
		{
			input: ``,
			err:   true,
		},
		{
			input: `{}`,
			err:   true,
		},
		{
			input: `{
				"strategy,": "",
			}`,
			err: true,
		},
		{
			input: `{
				"strategy": "",
				"resources": []
			}`,
			err: true,
		},
		{
			input: `{
				"strategy": "",
				"resources": [
					{
						"name": "valid",
						"replicas": 2
					}
				]
			}`,
			output: TimeSlicing{
				Strategy: UnspecifiedTimeSlicingStrategy,
				Resources: []ReplicaResource{
					{
						Name:     "valid",
						Devices:  ReplicaDevices{All: true},
						Replicas: 2,
					},
				},
			},
		},
		{
			input: `{
				"resources": [
					{
						"name": "valid",
						"replicas": 2
					}
				]
			}`,
			output: TimeSlicing{
				Strategy: UnspecifiedTimeSlicingStrategy,
				Resources: []ReplicaResource{
					{
						Name:     "valid",
						Devices:  ReplicaDevices{All: true},
						Replicas: 2,
					},
				},
			},
		},
		{
			input: `{
				"resources": [
					{
						"name": "valid1",
						"replicas": 2
					},
					{
						"name": "valid2",
						"replicas": 2
					}
				]
			}`,
			output: TimeSlicing{
				Strategy: UnspecifiedTimeSlicingStrategy,
				Resources: []ReplicaResource{
					{
						Name:     "valid1",
						Devices:  ReplicaDevices{All: true},
						Replicas: 2,
					},
					{
						Name:     "valid2",
						Devices:  ReplicaDevices{All: true},
						Replicas: 2,
					},
				},
			},
		},
		{
			input: `{
				"strategy": "bogus",
				"resources": [
					{
						"name": "valid",
						"replicas": 2
					}
				]
			}`,
			err: true,
		},
		{
			input: `{
				"resources": [
					{
						"name": "$invalid$",
						"replicas": 2
					}
				]
			}`,
			err: true,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var output TimeSlicing
			err := output.UnmarshalJSON([]byte(tc.input))
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.output, output)
		})
	}
}
