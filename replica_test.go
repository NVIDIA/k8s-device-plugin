/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
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

package main

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_prioritizeDevices(t *testing.T) {
	type args struct {
		availableDeviceIDs   []string
		mustIncludeDeviceIDs []string
		allocationSize       int
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 error
	}{
		{"Basic",
			args{[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, []string{}, 1},
			[]string{"a-replica-0"}, nil,
		},
		{"Multiple Unique",
			args{[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, []string{}, 2},
			[]string{"a-replica-0", "b-replica-1"}, nil,
		},
		{"NonuniqueError",
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-1"}, []string{}, 3},
			[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, &NonUniqueError{},
		},
		{"Must Include Greater Utilized",
			args{[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, []string{"b-replica-1"}, 1},
			[]string{"b-replica-1"}, nil,
		},
		{"Must Include Least Utilized",
			args{[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, []string{"a-replica-1"}, 1},
			[]string{"a-replica-1"}, nil,
		},
		{"Must Include Two",
			args{[]string{"a-replica-0", "a-replica-1", "b-replica-1"}, []string{"a-replica-1"}, 2},
			[]string{"a-replica-1", "b-replica-1"}, nil,
		},
		{"NonuniqueError Must Include",
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-2", "b-replica-1"}, []string{"a-replica-2"}, 3},
			[]string{"a-replica-0", "a-replica-2", "b-replica-1"}, &NonUniqueError{},
		},
		{"Must Include",
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-1", "c-replica-0"}, []string{"a-replica-2"}, 3},
			[]string{"a-replica-2", "b-replica-1", "c-replica-0"}, nil,
		},
		{"Must Include Entire Allocated",
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-1"}, []string{"a-replica-2", "b-replica-1", "a-replica-1"}, 3},
			[]string{"a-replica-1", "a-replica-2", "b-replica-1"}, &NonUniqueError{},
		},
		{"Deterministic",
			args{[]string{"a-replica-1", "b-replica-1", "c-replica-1", "d-replica-1", "e-replica-1", "f-replica-1", "g-replica-1", "h-replica-1"}, []string{}, 1},
			[]string{"a-replica-1"}, nil,
		},
		{"OversizedRequest", // With the scheduler, this should not happen in the first place.
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-1"}, []string{}, 5},
			nil, fmt.Errorf("no devices left to allocate"),
		},
		{"Undersized", // With the scheduler, this should not happen in the first place.
			args{[]string{"a-replica-0", "a-replica-1", "a-replica-2", "b-replica-1"}, []string{}, 0},
			[]string{}, nil,
		},
		{"NoneAvailable", // With the scheduler, this should not happen in the first place.
			args{[]string{}, []string{}, 1},
			nil, fmt.Errorf("no devices left to allocate"),
		},
		{"SubsetSame", // Should never happen
			args{[]string{"a-replica-0", "a-replica-1"}, []string{"a-replica-2"}, 1},
			nil, fmt.Errorf("device '%s' in mustIncludeDeviceIDs is missing from availableDeviceIDs", "a-replica-2"),
		},
		{"SubsetDifferent", // Should never happen
			args{[]string{"a-replica-0", "a-replica-1"}, []string{"b-replica-2"}, 1},
			nil, fmt.Errorf("device '%s' in mustIncludeDeviceIDs is missing from availableDeviceIDs", "b-replica-2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := prioritizeDevices(tt.args.availableDeviceIDs, tt.args.mustIncludeDeviceIDs, tt.args.allocationSize)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("prioritizeDevices() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("prioritizeDevices() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_stripReplicas(t *testing.T) {
	type args struct {
		deviceReplicaIDs []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"Simple", args{[]string{"b-replica-5", "a-replica-1", "a-replica-0"}}, []string{"a", "b"}},
		{"Simple2", args{[]string{"b-replica-0", "a-replica-1", "a-replica-2", "c-replica-2"}}, []string{"a", "b", "c"}},
		{"Empty", args{[]string{}}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripReplicas(tt.args.deviceReplicaIDs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stripReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}
