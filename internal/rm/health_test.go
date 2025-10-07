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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHealthCheckXIDs(t *testing.T) {
	testCases := []struct {
		input    string
		expected healthCheckXIDs
	}{
		{
			expected: healthCheckXIDs{},
		},
		{
			input:    ",",
			expected: healthCheckXIDs{},
		},
		{
			input:    "not-an-int",
			expected: healthCheckXIDs{},
		},
		{
			input:    "68",
			expected: healthCheckXIDs{68: true},
		},
		{
			input:    "-68",
			expected: healthCheckXIDs{},
		},
		{
			input:    "68  ",
			expected: healthCheckXIDs{68: true},
		},
		{
			input:    "68,",
			expected: healthCheckXIDs{68: true},
		},
		{
			input:    ",68",
			expected: healthCheckXIDs{68: true},
		},
		{
			input:    "68,67",
			expected: healthCheckXIDs{67: true, 68: true},
		},
		{
			input:    "68,not-an-int,67",
			expected: healthCheckXIDs{67: true, 68: true},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			xids := newHealthCheckXIDs(strings.Split(tc.input, ",")...)

			require.EqualValues(t, tc.expected, xids)
		})
	}
}
