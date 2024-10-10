/**
# Copyright 2024 NVIDIA CORPORATION
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

package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultChannelStrategyFlagUnmarshal(t *testing.T) {
	testCases := []struct {
		description   string
		inputs        []string
		expected      DefaultChannelStrategy
		expectedError error
	}{
		{
			description: "auto cases",
			inputs:      []string{`"auto"`},
			expected:    DefaultChannelStrategyAuto,
		},
		{
			description: "disabled cases",
			inputs:      []string{"", `""`, `"0"`, `"off"`, `"no"`, `"disabled"`, `0`},
			expected:    DefaultChannelStrategyDisabled,
		},
		{
			description: "enabled cases",
			inputs:      []string{`"1"`, `"on"`, `"yes"`, `"enabled"`, `1`},
			expected:    DefaultChannelStrategyEnabled,
		},
		{
			description:   "invalid cases",
			inputs:        []string{`"-1"`, `"not"`, `"NO"`, `"YES"`, `"200"`},
			expectedError: errInvalidDefaultChannelStrategy,
		},
	}

	for _, tc := range testCases {
		for _, input := range tc.inputs {
			t.Run(tc.description+input, func(t *testing.T) {
				var output DefaultChannelStrategy
				err := output.UnmarshalJSON([]byte(input))
				require.ErrorIs(t, err, tc.expectedError)
				require.Equal(t, tc.expected, output)
			})
		}
	}
}
