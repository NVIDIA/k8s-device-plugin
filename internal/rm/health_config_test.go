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

package rm

import (
	"os"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

func TestGetHealthConfig(t *testing.T) {
	testCases := []struct {
		name           string
		config         *spec.Config
		envVar         string
		expectedResult func(*spec.Health) bool
	}{
		{
			name: "config with health settings",
			config: &spec.Config{
				Health: &spec.Health{
					Disabled:    false,
					EventTypes:  []string{"EventTypeXidCriticalError"},
					IgnoredXIDs: []uint64{13, 31},
					CriticalXIDs: &spec.CriticalXIDsType{
						All: true,
					},
				},
			},
			envVar: "",
			expectedResult: func(h *spec.Health) bool {
				return !h.Disabled && len(h.IgnoredXIDs) == 2
			},
		},
		{
			name: "config with env var override to disable",
			config: &spec.Config{
				Health: &spec.Health{
					Disabled: false,
				},
			},
			envVar: "all",
			expectedResult: func(h *spec.Health) bool {
				return h.Disabled
			},
		},
		{
			name: "config with env var adding ignored XIDs",
			config: &spec.Config{
				Health: &spec.Health{
					Disabled:    false,
					IgnoredXIDs: []uint64{13},
				},
			},
			envVar: "48,79",
			expectedResult: func(h *spec.Health) bool {
				return !h.Disabled && len(h.IgnoredXIDs) == 3
			},
		},
		{
			name:   "no config with default health",
			config: &spec.Config{},
			envVar: "",
			expectedResult: func(h *spec.Health) bool {
				// Should use default health config
				return !h.Disabled && len(h.EventTypes) == 3 && len(h.IgnoredXIDs) == 6
			},
		},
		{
			name:   "no config with env var disable",
			config: &spec.Config{},
			envVar: "xids",
			expectedResult: func(h *spec.Health) bool {
				return h.Disabled
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variable if specified
			if tc.envVar != "" {
				os.Setenv(envDisableHealthChecks, tc.envVar)
				defer os.Unsetenv(envDisableHealthChecks)
			}

			r := &nvmlResourceManager{
				resourceManager: resourceManager{
					config: tc.config,
				},
			}

			health := r.getHealthConfig()
			require.NotNil(t, health)
			require.True(t, tc.expectedResult(health), "Health config did not match expected result")
		})
	}
}

func TestParseXidsFromEnv(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []uint64
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single XID",
			input:    "48",
			expected: []uint64{48},
		},
		{
			name:     "multiple XIDs",
			input:    "48,79,94",
			expected: []uint64{48, 79, 94},
		},
		{
			name:     "XIDs with spaces",
			input:    " 48 , 79 , 94 ",
			expected: []uint64{48, 79, 94},
		},
		{
			name:     "invalid XIDs ignored",
			input:    "48,invalid,79",
			expected: []uint64{48, 79},
		},
		{
			name:     "mixed valid and invalid",
			input:    "48,,79,abc,94",
			expected: []uint64{48, 79, 94},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseXidsFromEnv(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildEventMask(t *testing.T) {
	testCases := []struct {
		name        string
		eventTypes  []string
		expected    uint64
		expectError bool
	}{
		{
			name:        "empty event types returns default",
			eventTypes:  []string{},
			expected:    uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError),
			expectError: false,
		},
		{
			name:        "single event type",
			eventTypes:  []string{"EventTypeXidCriticalError"},
			expected:    uint64(nvml.EventTypeXidCriticalError),
			expectError: false,
		},
		{
			name:        "multiple event types",
			eventTypes:  []string{"EventTypeXidCriticalError", "EventTypeDoubleBitEccError"},
			expected:    uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError),
			expectError: false,
		},
		{
			name: "all event types",
			eventTypes: []string{
				"EventTypeXidCriticalError",
				"EventTypeDoubleBitEccError",
				"EventTypeSingleBitEccError",
			},
			expected:    uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError),
			expectError: false,
		},
		{
			name:        "invalid event type",
			eventTypes:  []string{"InvalidEventType"},
			expected:    0,
			expectError: true,
		},
		{
			name:        "mix of valid and invalid",
			eventTypes:  []string{"EventTypeXidCriticalError", "InvalidEventType"},
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := buildEventMask(tc.eventTypes)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestHealthConfigIntegration(t *testing.T) {
	// Test that health config properly integrates with XID checking
	testCases := []struct {
		name       string
		health     *spec.Health
		xid        uint64
		isCritical bool
	}{
		{
			name: "default config with ignored XID",
			health: &spec.Health{
				IgnoredXIDs: []uint64{13, 31, 43},
				CriticalXIDs: &spec.CriticalXIDsType{
					All: true,
				},
			},
			xid:        31,
			isCritical: false,
		},
		{
			name: "default config with critical XID",
			health: &spec.Health{
				IgnoredXIDs: []uint64{13, 31, 43},
				CriticalXIDs: &spec.CriticalXIDsType{
					All: true,
				},
			},
			xid:        48,
			isCritical: true,
		},
		{
			name: "GKE-style config with specific critical",
			health: &spec.Health{
				IgnoredXIDs: []uint64{},
				CriticalXIDs: &spec.CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48},
				},
			},
			xid:        48,
			isCritical: true,
		},
		{
			name: "GKE-style config with non-critical",
			health: &spec.Health{
				IgnoredXIDs: []uint64{},
				CriticalXIDs: &spec.CriticalXIDsType{
					All:  false,
					XIDs: []uint64{48},
				},
			},
			xid:        79,
			isCritical: false,
		},
		{
			name: "disabled health checks",
			health: &spec.Health{
				Disabled: true,
				CriticalXIDs: &spec.CriticalXIDsType{
					All: true,
				},
			},
			xid:        48,
			isCritical: false, // Nothing is critical when disabled
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.health.IsCritical(tc.xid)
			require.Equal(t, tc.isCritical, result)
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that the new system maintains backward compatibility with DP_DISABLE_HEALTHCHECKS

	t.Run("env var 'all' disables health checks", func(t *testing.T) {
		os.Setenv(envDisableHealthChecks, "all")
		defer os.Unsetenv(envDisableHealthChecks)

		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config: &spec.Config{
					Health: &spec.Health{
						Disabled: false,
					},
				},
			},
		}

		health := r.getHealthConfig()
		require.True(t, health.Disabled)
	})

	t.Run("env var 'xids' disables health checks", func(t *testing.T) {
		os.Setenv(envDisableHealthChecks, "xids")
		defer os.Unsetenv(envDisableHealthChecks)

		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config: &spec.Config{},
			},
		}

		health := r.getHealthConfig()
		require.True(t, health.Disabled)
	})

	t.Run("env var with XIDs adds to ignored list", func(t *testing.T) {
		os.Setenv(envDisableHealthChecks, "48,79")
		defer os.Unsetenv(envDisableHealthChecks)

		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config: &spec.Config{
					Health: &spec.Health{
						IgnoredXIDs: []uint64{13},
					},
				},
			},
		}

		health := r.getHealthConfig()
		require.False(t, health.Disabled)
		require.Contains(t, health.IgnoredXIDs, uint64(13))
		require.Contains(t, health.IgnoredXIDs, uint64(48))
		require.Contains(t, health.IgnoredXIDs, uint64(79))
	})
}
