/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourcePatternMatches(t *testing.T) {
	testCases := []struct {
		description string
		pattern     ResourcePattern
		input       string
		expected    bool
	}{
		{
			description: "exact match",
			pattern:     "1g.24gb",
			input:       "1g.24gb",
			expected:    true,
		},
		{
			description: "no match on different string",
			pattern:     "1g.24gb",
			input:       "2g.48gb",
			expected:    false,
		},
		{
			description: "does not substring match with suffix",
			pattern:     "1g.24gb",
			input:       "1g.24gb-me",
			expected:    false,
		},
		{
			description: "does not substring match with plus suffix",
			pattern:     "1g.24gb",
			input:       "1g.24gb+me.all",
			expected:    false,
		},
		{
			description: "does not substring match with prefix",
			pattern:     "1g.24gb",
			input:       "x1g.24gb",
			expected:    false,
		},
		{
			description: "dash-me suffix matches its own pattern",
			pattern:     "1g.24gb-me",
			input:       "1g.24gb-me",
			expected:    true,
		},
		{
			description: "plus-me.all suffix matches its own pattern",
			pattern:     "1g.24gb+me.all",
			input:       "1g.24gb+me.all",
			expected:    true,
		},
		{
			description: "wildcard matches everything",
			pattern:     "*",
			input:       "1g.24gb-me",
			expected:    true,
		},
		{
			description: "wildcard prefix match",
			pattern:     "1g.*",
			input:       "1g.24gb",
			expected:    true,
		},
		{
			description: "wildcard prefix does not match different prefix",
			pattern:     "1g.*",
			input:       "2g.48gb",
			expected:    false,
		},
		{
			description: "gfx suffix matches its own pattern",
			pattern:     "1g.24gb+gfx",
			input:       "1g.24gb+gfx",
			expected:    true,
		},
		{
			description: "base pattern does not match +gfx suffix",
			pattern:     "1g.24gb",
			input:       "1g.24gb+gfx",
			expected:    false,
		},
		{
			description: "gfx pattern does not match base",
			pattern:     "1g.24gb+gfx",
			input:       "1g.24gb",
			expected:    false,
		},
		{
			description: "gfx pattern does not match -me suffix",
			pattern:     "1g.24gb+gfx",
			input:       "1g.24gb-me",
			expected:    false,
		},
		{
			description: "2g gfx suffix matches its own pattern",
			pattern:     "2g.48gb+gfx",
			input:       "2g.48gb+gfx",
			expected:    true,
		},
		{
			description: "2g base does not match +gfx suffix",
			pattern:     "2g.48gb",
			input:       "2g.48gb+gfx",
			expected:    false,
		},
		{
			description: "wildcard suffix match",
			pattern:     "*gb",
			input:       "1g.24gb",
			expected:    true,
		},
		{
			description: "wildcard suffix does not match -me suffix",
			pattern:     "*gb",
			input:       "1g.24gb-me",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := tc.pattern.Matches(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
