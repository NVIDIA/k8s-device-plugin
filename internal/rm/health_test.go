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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
)

func TestNewHealthCheckXIDs(t *testing.T) {
	testCases := []struct {
		input    string
		expected disabledXIDs
	}{
		{
			expected: disabledXIDs{},
		},
		{
			input:    ",",
			expected: disabledXIDs{},
		},
		{
			input:    "not-an-int",
			expected: disabledXIDs{},
		},
		{
			input:    "68",
			expected: disabledXIDs{68: true},
		},
		{
			input:    "-68",
			expected: disabledXIDs{},
		},
		{
			input:    "68  ",
			expected: disabledXIDs{68: true},
		},
		{
			input:    "68,",
			expected: disabledXIDs{68: true},
		},
		{
			input:    ",68",
			expected: disabledXIDs{68: true},
		},
		{
			input:    "68,67",
			expected: disabledXIDs{67: true, 68: true},
		},
		{
			input:    "68,not-an-int,67",
			expected: disabledXIDs{67: true, 68: true},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			xids := newHealthCheckXIDs(strings.Split(tc.input, ",")...)

			require.EqualValues(t, tc.expected, xids)
		})
	}
}

func TestGetDisabledHealthCheckXids(t *testing.T) {
	testCases := []struct {
		description         string
		enabled             string
		disabled            string
		expectedAllDisabled bool
		expectedContents    disabledXIDs
		expectedDisabled    map[uint64]bool
	}{
		{
			description:         "empty envvars are default disabled",
			expectedAllDisabled: false,
			expectedContents: disabledXIDs{
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
		},
		{
			description:         "disabled is all",
			disabled:            "all",
			expectedAllDisabled: true,
			expectedContents: disabledXIDs{
				0:   true,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
				555: true,
			},
		},
		{
			description:         "disabled is xids",
			disabled:            "xids",
			expectedAllDisabled: true,
			expectedContents: disabledXIDs{
				0:   true,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
				555: true,
			},
		},
		{
			description:         "enabled is all",
			enabled:             "all",
			expectedAllDisabled: false,
			expectedContents: disabledXIDs{
				0:   false,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				13:  false,
				31:  false,
				43:  false,
				45:  false,
				68:  false,
				109: false,
				555: false,
			},
		},
		{
			description:         "enabled overrides disabled",
			disabled:            "11",
			enabled:             "11",
			expectedAllDisabled: false,
			expectedContents: disabledXIDs{
				11:  false,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				11:  false,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				109: true,
				555: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Setenv(envDisableHealthChecks, tc.disabled)
			t.Setenv(envEnableHealthChecks, tc.enabled)

			xids := getDisabledHealthCheckXids()
			require.EqualValues(t, tc.expectedContents, xids)
			require.Equal(t, tc.expectedAllDisabled, xids.IsAllDisabled())

			disabled := make(map[uint64]bool)
			for xid := range tc.expectedDisabled {
				disabled[xid] = xids.IsDisabled(xid)
			}
			require.Equal(t, tc.expectedDisabled, disabled)
		})
	}
}

func TestCheckRemappedRows(t *testing.T) {
	rm := &nvmlResourceManager{}

	testCases := []struct {
		description     string
		device          *DeviceMock
		expectedReason  string
		expectedFailure bool
	}{
		{
			description: "not supported returns healthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 0, false, false, nvml.ERROR_NOT_SUPPORTED
				},
			},
			expectedFailure: false,
		},
		{
			description: "nvml error returns healthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 0, false, false, nvml.ERROR_UNKNOWN
				},
			},
			expectedFailure: false,
		},
		{
			description: "no issues returns healthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 0, false, false, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
		{
			description: "uncorrectable rows remapped successfully returns healthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 2, 3, false, false, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
		{
			description: "row remapping failure returns unhealthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 0, false, true, nvml.SUCCESS
				},
			},
			expectedReason:  "row remapping failure occurred (uncorrectable memory error)",
			expectedFailure: true,
		},
		{
			description: "pending row remap returns unhealthy",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 0, true, false, nvml.SUCCESS
				},
			},
			expectedReason:  "row remapping is pending (GPU reset required)",
			expectedFailure: true,
		},
		{
			description: "failure takes precedence over pending",
			device: &DeviceMock{
				GetRemappedRowsFunc: func() (int, int, bool, bool, nvml.Return) {
					return 0, 1, true, true, nvml.SUCCESS
				},
			},
			expectedReason:  "row remapping failure occurred (uncorrectable memory error)",
			expectedFailure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			reason, failed := rm.checkRemappedRows(tc.device)
			require.Equal(t, tc.expectedFailure, failed)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}

func TestCheckRetiredPages(t *testing.T) {
	rm := &nvmlResourceManager{}

	testCases := []struct {
		description     string
		device          *DeviceMock
		expectedReason  string
		expectedFailure bool
	}{
		{
			description: "not supported returns healthy",
			device: &DeviceMock{
				GetRetiredPagesPendingStatusFunc: func() (nvml.EnableState, nvml.Return) {
					return nvml.FEATURE_DISABLED, nvml.ERROR_NOT_SUPPORTED
				},
			},
			expectedFailure: false,
		},
		{
			description: "nvml error returns healthy",
			device: &DeviceMock{
				GetRetiredPagesPendingStatusFunc: func() (nvml.EnableState, nvml.Return) {
					return nvml.FEATURE_DISABLED, nvml.ERROR_UNKNOWN
				},
			},
			expectedFailure: false,
		},
		{
			description: "no pending retired pages returns healthy",
			device: &DeviceMock{
				GetRetiredPagesPendingStatusFunc: func() (nvml.EnableState, nvml.Return) {
					return nvml.FEATURE_DISABLED, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
		{
			description: "pending retired pages returns unhealthy",
			device: &DeviceMock{
				GetRetiredPagesPendingStatusFunc: func() (nvml.EnableState, nvml.Return) {
					return nvml.FEATURE_ENABLED, nvml.SUCCESS
				},
			},
			expectedReason:  "pages are pending retirement (reboot required)",
			expectedFailure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			reason, failed := rm.checkRetiredPages(tc.device)
			require.Equal(t, tc.expectedFailure, failed)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}

func TestCheckTemperature(t *testing.T) {
	rm := &nvmlResourceManager{}

	testCases := []struct {
		description     string
		device          *DeviceMock
		expectedReason  string
		expectedFailure bool
	}{
		{
			description: "shutdown threshold not supported returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
			},
			expectedFailure: false,
		},
		{
			description: "shutdown threshold error returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					return 0, nvml.ERROR_UNKNOWN
				},
			},
			expectedFailure: false,
		},
		{
			description: "temperature sensor not supported returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					return 100, nvml.SUCCESS
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
			},
			expectedFailure: false,
		},
		{
			description: "temperature sensor error returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					return 100, nvml.SUCCESS
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 0, nvml.ERROR_UNKNOWN
				},
			},
			expectedFailure: false,
		},
		{
			description: "temperature below shutdown returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					switch tt {
					case nvml.TEMPERATURE_THRESHOLD_SHUTDOWN:
						return 100, nvml.SUCCESS
					case nvml.TEMPERATURE_THRESHOLD_SLOWDOWN:
						return 90, nvml.SUCCESS
					}
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 50, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
		{
			description: "temperature at slowdown but below shutdown returns healthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					switch tt {
					case nvml.TEMPERATURE_THRESHOLD_SHUTDOWN:
						return 100, nvml.SUCCESS
					case nvml.TEMPERATURE_THRESHOLD_SLOWDOWN:
						return 90, nvml.SUCCESS
					}
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 92, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
		{
			description: "temperature at shutdown threshold returns unhealthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					switch tt {
					case nvml.TEMPERATURE_THRESHOLD_SHUTDOWN:
						return 100, nvml.SUCCESS
					case nvml.TEMPERATURE_THRESHOLD_SLOWDOWN:
						return 90, nvml.SUCCESS
					}
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 100, nvml.SUCCESS
				},
			},
			expectedReason:  "GPU temperature (100°C) has reached shutdown threshold (100°C)",
			expectedFailure: true,
		},
		{
			description: "temperature above shutdown threshold returns unhealthy",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					switch tt {
					case nvml.TEMPERATURE_THRESHOLD_SHUTDOWN:
						return 100, nvml.SUCCESS
					case nvml.TEMPERATURE_THRESHOLD_SLOWDOWN:
						return 90, nvml.SUCCESS
					}
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 105, nvml.SUCCESS
				},
			},
			expectedReason:  "GPU temperature (105°C) has reached shutdown threshold (100°C)",
			expectedFailure: true,
		},
		{
			description: "slowdown threshold error does not affect result",
			device: &DeviceMock{
				GetTemperatureThresholdFunc: func(tt nvml.TemperatureThresholds) (uint32, nvml.Return) {
					switch tt {
					case nvml.TEMPERATURE_THRESHOLD_SHUTDOWN:
						return 100, nvml.SUCCESS
					case nvml.TEMPERATURE_THRESHOLD_SLOWDOWN:
						return 0, nvml.ERROR_NOT_SUPPORTED
					}
					return 0, nvml.ERROR_NOT_SUPPORTED
				},
				GetTemperatureFunc: func(ts nvml.TemperatureSensors) (uint32, nvml.Return) {
					return 85, nvml.SUCCESS
				},
			},
			expectedFailure: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			reason, failed := rm.checkTemperature(tc.device)
			require.Equal(t, tc.expectedFailure, failed)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}
