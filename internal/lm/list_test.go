/**
# Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockLabeler is a test double for the Labeler interface
type mockLabeler struct {
	labels Labels
	err    error
}

func (m *mockLabeler) Labels() (Labels, error) {
	return m.labels, m.err
}

func TestMerge(t *testing.T) {
	testCases := []struct {
		description    string
		labelers       []Labeler
		expectedLabels Labels
	}{
		{
			description:    "empty list returns empty labels",
			labelers:       []Labeler{},
			expectedLabels: Labels{},
		},
		{
			description: "single successful labeler",
			labelers: []Labeler{
				&mockLabeler{
					labels: Labels{"key1": "value1"},
				},
			},
			expectedLabels: Labels{"key1": "value1"},
		},
		{
			description: "multiple successful labelers",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{"key1": "value1"}},
				&mockLabeler{labels: Labels{"key2": "value2"}},
				&mockLabeler{labels: Labels{"key3": "value3"}},
			},
			expectedLabels: Labels{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			description: "single failing labeler is skipped",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{"key1": "value1"}},
				&mockLabeler{err: fmt.Errorf("device unhealthy: GPU is lost")},
				&mockLabeler{labels: Labels{"key3": "value3"}},
			},
			expectedLabels: Labels{
				"key1": "value1",
				"key3": "value3",
			},
		},
		{
			description: "multiple failing labelers are skipped",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{"key1": "value1"}},
				&mockLabeler{err: fmt.Errorf("error 1")},
				&mockLabeler{err: fmt.Errorf("error 2")},
				&mockLabeler{labels: Labels{"key4": "value4"}},
			},
			expectedLabels: Labels{
				"key1": "value1",
				"key4": "value4",
			},
		},
		{
			description: "all failing labelers returns empty labels",
			labelers: []Labeler{
				&mockLabeler{err: fmt.Errorf("error 1")},
				&mockLabeler{err: fmt.Errorf("error 2")},
			},
			expectedLabels: Labels{},
		},
		{
			description: "later labeler overwrites earlier labels",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{"key": "value1"}},
				&mockLabeler{labels: Labels{"key": "value2"}},
			},
			expectedLabels: Labels{"key": "value2"},
		},
		{
			description: "empty labels from labeler are merged",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{}},
				&mockLabeler{labels: Labels{"key": "value"}},
			},
			expectedLabels: Labels{"key": "value"},
		},
		{
			description: "failing labeler between successful ones",
			labelers: []Labeler{
				&mockLabeler{labels: Labels{"before": "value"}},
				&mockLabeler{err: fmt.Errorf("GPU XID error")},
				&mockLabeler{labels: Labels{"after": "value"}},
			},
			expectedLabels: Labels{
				"before": "value",
				"after":  "value",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			merged := Merge(tc.labelers...)
			labels, err := merged.Labels()

			require.NoError(t, err, "Merge should never return error")
			require.EqualValues(t, tc.expectedLabels, labels)
		})
	}
}
