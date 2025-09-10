/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestCriticalXIDsType_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected CriticalXIDsType
		hasError bool
	}{
		{
			name:  "all string",
			input: `"all"`,
			expected: CriticalXIDsType{
				All:  true,
				XIDs: nil,
			},
		},
		{
			name:  "ALL string (case insensitive)",
			input: `"ALL"`,
			expected: CriticalXIDsType{
				All:  true,
				XIDs: nil,
			},
		},
		{
			name:  "array of XIDs",
			input: `[48, 79, 94]`,
			expected: CriticalXIDsType{
				All:  false,
				XIDs: []uint64{48, 79, 94},
			},
		},
		{
			name:     "invalid string",
			input:    `"invalid"`,
			hasError: true,
		},
		{
			name:  "empty array",
			input: `[]`,
			expected: CriticalXIDsType{
				All:  false,
				XIDs: []uint64{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var c CriticalXIDsType
			err := json.Unmarshal([]byte(tc.input), &c)

			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, c)
			}
		})
	}
}

func TestCriticalXIDsType_MarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    CriticalXIDsType
		expected string
	}{
		{
			name: "all XIDs",
			input: CriticalXIDsType{
				All: true,
			},
			expected: `"all"`,
		},
		{
			name: "specific XIDs",
			input: CriticalXIDsType{
				All:  false,
				XIDs: []uint64{48, 79},
			},
			expected: `[48,79]`,
		},
		{
			name: "empty XIDs",
			input: CriticalXIDsType{
				All:  false,
				XIDs: []uint64{},
			},
			expected: `[]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.JSONEq(t, tc.expected, string(data))
		})
	}
}

func TestCriticalXIDsType_YAML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected CriticalXIDsType
	}{
		{
			name:  "all string",
			input: "all",
			expected: CriticalXIDsType{
				All:  true,
				XIDs: nil,
			},
		},
		{
			name: "array of XIDs",
			input: `- 48
- 79
- 94`,
			expected: CriticalXIDsType{
				All:  false,
				XIDs: []uint64{48, 79, 94},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var c CriticalXIDsType
			err := yaml.Unmarshal([]byte(tc.input), &c)
			require.NoError(t, err)
			require.Equal(t, tc.expected, c)

			// Test marshaling back
			data, err := yaml.Marshal(c)
			require.NoError(t, err)

			// Unmarshal again to verify roundtrip
			var c2 CriticalXIDsType
			err = yaml.Unmarshal(data, &c2)
			require.NoError(t, err)
			require.Equal(t, c, c2)
		})
	}
}

func TestHealth_IsCritical(t *testing.T) {
	testCases := []struct {
		name     string
		health   Health
		xid      uint64
		expected bool
	}{
		{
			name: "disabled health checks",
			health: Health{
				Disabled: true,
			},
			xid:      48,
			expected: false,
		},
		{
			name: "ignored XID",
			health: Health{
				IgnoredXIDs: []uint64{13, 31, 43},
				CriticalXIDs: &CriticalXIDsType{
					All: true,
				},
			},
			xid:      31,
			expected: false,
		},
		{
			name: "all XIDs critical, not in ignored",
			health: Health{
				IgnoredXIDs: []uint64{13, 31},
				CriticalXIDs: &CriticalXIDsType{
					All: true,
				},
			},
			xid:      48,
			expected: true,
		},
		{
			name: "specific critical XID",
			health: Health{
				CriticalXIDs: &CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48, 79},
				},
			},
			xid:      48,
			expected: true,
		},
		{
			name: "not in critical list",
			health: Health{
				CriticalXIDs: &CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48, 79},
				},
			},
			xid:      94,
			expected: false,
		},
		{
			name: "nil critical XIDs defaults to all",
			health: Health{
				CriticalXIDs: nil,
			},
			xid:      94,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.health.IsCritical(tc.xid)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHealth_Validate(t *testing.T) {
	testCases := []struct {
		name     string
		health   *Health
		hasError bool
		errorMsg string
	}{
		{
			name:     "nil health",
			health:   nil,
			hasError: false,
		},
		{
			name: "valid configuration",
			health: &Health{
				EventTypes:  []string{"EventTypeXidCriticalError", "EventTypeDoubleBitEccError"},
				IgnoredXIDs: []uint64{13, 31},
				CriticalXIDs: &CriticalXIDsType{
					All: true,
				},
			},
			hasError: false,
		},
		{
			name: "invalid event type",
			health: &Health{
				EventTypes: []string{"InvalidEventType"},
			},
			hasError: true,
			errorMsg: "invalid event type: InvalidEventType",
		},
		{
			name: "XID in both ignored and critical",
			health: &Health{
				IgnoredXIDs: []uint64{48, 79},
				CriticalXIDs: &CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48, 94},
				},
			},
			hasError: true,
			errorMsg: "XID 48 is in both ignored and critical lists",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.health.Validate()
			if tc.hasError {
				require.Error(t, err)
				if tc.errorMsg != "" {
					require.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultHealth(t *testing.T) {
	h := DefaultHealth()
	require.NotNil(t, h)

	require.False(t, h.Disabled)
	require.Contains(t, h.EventTypes, "EventTypeXidCriticalError")
	require.Contains(t, h.EventTypes, "EventTypeDoubleBitEccError")
	require.Contains(t, h.EventTypes, "EventTypeSingleBitEccError")

	require.Contains(t, h.IgnoredXIDs, uint64(13))
	require.Contains(t, h.IgnoredXIDs, uint64(31))
	require.Contains(t, h.IgnoredXIDs, uint64(43))
	require.Contains(t, h.IgnoredXIDs, uint64(45))
	require.Contains(t, h.IgnoredXIDs, uint64(68))
	require.Contains(t, h.IgnoredXIDs, uint64(109))

	require.NotNil(t, h.CriticalXIDs)
	require.True(t, h.CriticalXIDs.All)
}

func TestHealth_Merge(t *testing.T) {
	// Note: The Merge function intentionally overwrites the Disabled field
	// even when set to false. This is by design - when merging configurations,
	// the 'other' config represents a higher-priority source that should
	// override the base configuration. If we need to distinguish between
	// "unset" and "explicitly false", we would need to change Disabled
	// from bool to *bool, which would be a breaking API change.
	testCases := []struct {
		name     string
		base     *Health
		other    *Health
		expected *Health
	}{
		{
			name:     "nil other",
			base:     DefaultHealth(),
			other:    nil,
			expected: DefaultHealth(),
		},
		{
			name: "override disabled false to true",
			base: &Health{
				Disabled: false,
			},
			other: &Health{
				Disabled: true,
			},
			expected: &Health{
				Disabled: true,
			},
		},
		{
			name: "override disabled true to false",
			base: &Health{
				Disabled: true,
			},
			other: &Health{
				Disabled: false,
			},
			expected: &Health{
				Disabled: false,
			},
		},
		{
			name: "override event types",
			base: &Health{
				EventTypes: []string{"EventTypeXidCriticalError"},
			},
			other: &Health{
				EventTypes: []string{"EventTypeDoubleBitEccError"},
			},
			expected: &Health{
				EventTypes: []string{"EventTypeDoubleBitEccError"},
			},
		},
		{
			name: "override ignored XIDs",
			base: &Health{
				IgnoredXIDs: []uint64{13, 31},
			},
			other: &Health{
				IgnoredXIDs: []uint64{43, 45},
			},
			expected: &Health{
				IgnoredXIDs: []uint64{43, 45},
			},
		},
		{
			name: "override critical XIDs",
			base: &Health{
				CriticalXIDs: &CriticalXIDsType{
					All: true,
				},
			},
			other: &Health{
				CriticalXIDs: &CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48},
				},
			},
			expected: &Health{
				CriticalXIDs: &CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48},
				},
			},
		},
		{
			name: "merge minimal config with disabled=false overwrites base",
			base: &Health{
				Disabled:    true,
				EventTypes:  []string{"EventTypeXidCriticalError"},
				IgnoredXIDs: []uint64{13, 31},
			},
			other: &Health{
				Disabled: false, // Explicitly enabling health checks
			},
			expected: &Health{
				Disabled:    false,                                 // This is intentional - explicit false overrides
				EventTypes:  []string{"EventTypeXidCriticalError"}, // Unchanged
				IgnoredXIDs: []uint64{13, 31},                      // Unchanged
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.base.Merge(tc.other)
			require.Equal(t, tc.expected, tc.base)
		})
	}
}

func TestHealthConfigSerialization(t *testing.T) {
	// Test that health config can be properly marshaled/unmarshaled
	health := &Health{
		Disabled:    false,
		EventTypes:  []string{"EventTypeXidCriticalError", "EventTypeDoubleBitEccError"},
		IgnoredXIDs: []uint64{13, 31, 43},
		CriticalXIDs: &CriticalXIDsType{
			All: true,
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(health)
	require.NoError(t, err)

	var healthFromJSON Health
	err = json.Unmarshal(jsonData, &healthFromJSON)
	require.NoError(t, err)
	require.Equal(t, health, &healthFromJSON)

	// Test YAML marshaling
	yamlData, err := yaml.Marshal(health)
	require.NoError(t, err)

	var healthFromYAML Health
	err = yaml.Unmarshal(yamlData, &healthFromYAML)
	require.NoError(t, err)
	require.Equal(t, health, &healthFromYAML)
}

func TestHealthConfigExample(t *testing.T) {
	// Test parsing a YAML configuration similar to what would be in a config file
	yamlConfig := `
disabled: false
eventTypes:
  - EventTypeXidCriticalError
  - EventTypeDoubleBitEccError
ignoredXIDs:
  - 13
  - 31
  - 43
criticalXIDs: all
`
	var health Health
	err := yaml.Unmarshal([]byte(yamlConfig), &health)
	require.NoError(t, err)

	require.False(t, health.Disabled)
	require.Len(t, health.EventTypes, 2)
	require.Len(t, health.IgnoredXIDs, 3)
	require.NotNil(t, health.CriticalXIDs)
	require.True(t, health.CriticalXIDs.All)

	// Test GKE-style config
	gkeYamlConfig := `
eventTypes:
  - EventTypeXidCriticalError
criticalXIDs: [48]
`
	var gkeHealth Health
	err = yaml.Unmarshal([]byte(gkeYamlConfig), &gkeHealth)
	require.NoError(t, err)

	require.False(t, gkeHealth.Disabled)
	require.Len(t, gkeHealth.EventTypes, 1)
	require.Empty(t, gkeHealth.IgnoredXIDs)
	require.NotNil(t, gkeHealth.CriticalXIDs)
	require.False(t, gkeHealth.CriticalXIDs.All)
	require.Equal(t, []uint64{48}, gkeHealth.CriticalXIDs.XIDs)
}
