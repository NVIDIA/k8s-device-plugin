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

package lm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGerenerateDomainUUID(t *testing.T) {
	testCases := []struct {
		description string
		ips         []string
		expected    string
	}{
		{
			description: "single IP",
			ips:         []string{"10.130.3.24"},
			expected:    "4dbd3d31-fbb3-8a40-33bb-bcc0dd7b68b8",
		},
		{
			description: "multiple IPs",
			ips: []string{
				"10.130.3.24",
				"10.130.3.53",
				"10.130.3.23",
				"10.130.3.31",
				"10.130.3.27",
				"10.130.3.25",
			},
			expected: "42401dd1-8a08-1889-4341-8429de2b6f42",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			id := generateContentUUID(strings.Join(tc.ips, "\n"))
			require.Equal(t, tc.expected, id)
		})
	}
}
