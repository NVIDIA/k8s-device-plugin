/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestEventTypeToNVMLEventType(t *testing.T) {
	testCases := []struct {
		eventType EventType
		expected  uint64
		expectErr bool
	}{
		{
			eventType: EventTypeXidCriticalError,
			expected:  uint64(nvml.EventTypeXidCriticalError),
		},
		{
			eventType: EventTypeDoubleBitEccError,
			expected:  uint64(nvml.EventTypeDoubleBitEccError),
		},
		{
			eventType: EventTypeSingleBitEccError,
			expected:  uint64(nvml.EventTypeSingleBitEccError),
		},
		{
			eventType: EventType("invalid"),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.eventType), func(t *testing.T) {
			result, err := tc.eventType.ToNVMLEventType()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestCriticalXIDsJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected CriticalXIDs
	}{
		{
			name:     "all string",
			input:    `"all"`,
			expected: CriticalXIDs{All: true},
		},
		{
			name:     "ALL string (case insensitive)",
			input:    `"ALL"`,
			expected: CriticalXIDs{All: true},
		},
		{
			name:     "array of numbers",
			input:    `[48, 49, 50]`,
			expected: CriticalXIDs{Specific: []uint64{48, 49, 50}},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: CriticalXIDs{Specific: []uint64{}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var criticalXIDs CriticalXIDs
			err := json.Unmarshal([]byte(tc.input), &criticalXIDs)
			require.NoError(t, err)
			require.Equal(t, tc.expected, criticalXIDs)

			// Test marshaling back
			data, err := json.Marshal(criticalXIDs)
			require.NoError(t, err)
			
			var result CriticalXIDs
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestCriticalXIDsYAML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected CriticalXIDs
	}{
		{
			name:     "all string",
			input:    `all`,
			expected: CriticalXIDs{All: true},
		},
		{
			name: "array of numbers",
			input: `
- 48
- 49
- 50`,
			expected: CriticalXIDs{Specific: []uint64{48, 49, 50}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var criticalXIDs CriticalXIDs
			err := yaml.Unmarshal([]byte(tc.input), &criticalXIDs)
			require.NoError(t, err)
			require.Equal(t, tc.expected, criticalXIDs)

			// Test marshaling back
			data, err := yaml.Marshal(criticalXIDs)
			require.NoError(t, err)
			
			var result CriticalXIDs
			err = yaml.Unmarshal(data, &result)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestHealthDefaults(t *testing.T) {
	health := Health{}

	require.False(t, health.GetDisabled())

	expectedEventTypes := []EventType{
		EventTypeXidCriticalError,
		EventTypeDoubleBitEccError,
		EventTypeSingleBitEccError,
	}
	require.Equal(t, expectedEventTypes, health.GetEventTypes())

	expectedIgnoredXIDs := []uint64{13, 31, 43, 45, 68, 109}
	require.Equal(t, expectedIgnoredXIDs, health.GetIgnoredXIDs())

	expectedCriticalXIDs := &CriticalXIDs{All: true}
	require.Equal(t, expectedCriticalXIDs, health.GetCriticalXIDs())
}

func TestHealthGetEventMask(t *testing.T) {
	health := Health{}
	mask, err := health.GetEventMask()
	require.NoError(t, err)

	expected := uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)
	require.Equal(t, expected, mask)
}

func TestHealthXIDChecking(t *testing.T) {
	testCases := []struct {
		name           string
		health         Health
		xid            uint64
		expectIgnored  bool
		expectCritical bool
	}{
		{
			name:           "default - ignored XID",
			health:         Health{},
			xid:            13,
			expectIgnored:  true,
			expectCritical: false,
		},
		{
			name:           "default - critical XID",
			health:         Health{},
			xid:            48,
			expectIgnored:  false,
			expectCritical: true,
		},
		{
			name: "custom ignored XIDs",
			health: Health{
				IgnoredXIDs: []uint64{100, 101},
			},
			xid:           100,
			expectIgnored: true,
			expectCritical: false,
		},
		{
			name: "specific critical XIDs - included",
			health: Health{
				CriticalXIDs: &CriticalXIDs{Specific: []uint64{48, 49}},
			},
			xid:            48,
			expectIgnored:  false,
			expectCritical: true,
		},
		{
			name: "specific critical XIDs - not included",
			health: Health{
				CriticalXIDs: &CriticalXIDs{Specific: []uint64{48, 49}},
			},
			xid:            50,
			expectIgnored:  false,
			expectCritical: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectIgnored, tc.health.IsXIDIgnored(tc.xid))
			require.Equal(t, tc.expectCritical, tc.health.IsXIDCritical(tc.xid))
		})
	}
}

func TestHealthApplyEnvironmentOverrides(t *testing.T) {
	testCases := []struct {
		name            string
		input           string
		initialHealth   Health
		expectDisabled  bool
		expectedIgnored []uint64
	}{
		{
			name:            "empty input - no change",
			input:           "",
			initialHealth:   Health{},
			expectDisabled:  false,
			expectedIgnored: []uint64{13, 31, 43, 45, 68, 109},
		},
		{
			name:           "all - disable health checks",
			input:          "all",
			initialHealth:  Health{},
			expectDisabled: true,
		},
		{
			name:           "xids - disable health checks",
			input:          "xids",
			initialHealth:  Health{},
			expectDisabled: true,
		},
		{
			name:            "additional XIDs",
			input:           "200,201",
			initialHealth:   Health{},
			expectDisabled:  false,
			expectedIgnored: []uint64{13, 31, 43, 45, 68, 109, 200, 201},
		},
		{
			name: "additional XIDs with existing custom ignored",
			input: "200,201",
			initialHealth: Health{
				IgnoredXIDs: []uint64{100},
			},
			expectDisabled:  false,
			expectedIgnored: []uint64{100, 200, 201},
		},
		{
			name:            "invalid XIDs ignored",
			input:           "200,invalid,201",
			initialHealth:   Health{},
			expectDisabled:  false,
			expectedIgnored: []uint64{13, 31, 43, 45, 68, 109, 200, 201},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			health := tc.initialHealth
			health.ApplyEnvironmentOverrides(tc.input)

			require.Equal(t, tc.expectDisabled, health.GetDisabled())
			if !tc.expectDisabled {
				require.ElementsMatch(t, tc.expectedIgnored, health.GetIgnoredXIDs())
			}
		})
	}
}

func TestHealthConfigYAMLParsing(t *testing.T) {
	yamlConfig := `
version: v1
health:
  disabled: false
  eventTypes:
    - EventTypeXidCriticalError
    - EventTypeDoubleBitEccError
  ignoredXIDs: [13, 31, 43]
  criticalXIDs: all
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	require.NoError(t, err)

	require.False(t, config.Health.GetDisabled())
	require.Equal(t, []EventType{EventTypeXidCriticalError, EventTypeDoubleBitEccError}, config.Health.GetEventTypes())
	require.Equal(t, []uint64{13, 31, 43}, config.Health.GetIgnoredXIDs())
	require.True(t, config.Health.GetCriticalXIDs().All)
}

func TestHealthConfigGKEDefaults(t *testing.T) {
	// Test GKE-style configuration
	yamlConfig := `
version: v1
health:
  disabled: false
  eventTypes: [EventTypeXidCriticalError]
  ignoredXIDs: []
  criticalXIDs: [48]
`

	var config Config
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	require.NoError(t, err)

	require.False(t, config.Health.GetDisabled())
	require.Equal(t, []EventType{EventTypeXidCriticalError}, config.Health.GetEventTypes())
	require.Equal(t, []uint64{}, config.Health.GetIgnoredXIDs())
	require.Equal(t, []uint64{48}, config.Health.GetCriticalXIDs().Specific)
}