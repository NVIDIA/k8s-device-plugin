/**
# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
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

package rm

import (
	"testing"

	"github.com/stretchr/testify/require"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

func TestHealthConfigEnvironmentOverrides(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		initialHealth spec.Health
		expectedXIDs  []uint64
		expectDisabled bool
	}{
		{
			name:            "empty input",
			expectedXIDs:    []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:  "disable all with 'all'",
			input: "all",
			expectedXIDs:  []uint64{13, 31, 43, 45, 68, 109},
			expectDisabled: true,
		},
		{
			name:  "disable all with 'xids'",
			input: "xids",
			expectedXIDs:  []uint64{13, 31, 43, 45, 68, 109},
			expectDisabled: true,
		},
		{
			name:         "comma only",
			input:        ",",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "non-numeric value",
			input:        "not-an-int",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "single XID",
			input:        "68",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "negative number ignored",
			input:        "-68",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "XID with spaces",
			input:        "68  ",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "XID with trailing comma",
			input:        "68,",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "XID with leading comma",
			input:        ",68",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:         "multiple XIDs",
			input:        "200,201",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109, 200, 201},
		},
		{
			name:         "mixed valid and invalid XIDs",
			input:        "200,not-an-int,201",
			expectedXIDs: []uint64{13, 31, 43, 45, 68, 109, 200, 201},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			health := tc.initialHealth
			health.ApplyEnvironmentOverrides(tc.input)

			if tc.expectDisabled {
				require.True(t, health.GetDisabled())
			} else {
				require.False(t, health.GetDisabled())
				// Compare ignored XIDs (order doesn't matter)
				actualXIDs := health.GetIgnoredXIDs()
				require.ElementsMatch(t, tc.expectedXIDs, actualXIDs)
			}
		})
	}
}
