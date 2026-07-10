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
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
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

func TestParseMigDeviceUUID(t *testing.T) {
	testCases := []struct {
		description    string
		uuid           string
		expectedParent string
		expectedGi     uint32
		expectedCi     uint32
		expectError    bool
	}{
		{
			description:    "legacy MIG UUID format",
			uuid:           "MIG-GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f/3/0",
			expectedParent: "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f",
			expectedGi:     3,
			expectedCi:     0,
		},
		{
			description: "opaque MIG UUID format carries no placement information",
			uuid:        "MIG-30d00c09-8a98-59b8-8c1a-1d64b4ec3ad2",
			expectError: true,
		},
		{
			description: "full device UUID",
			uuid:        "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f",
			expectError: true,
		},
		{
			description: "legacy format with missing compute instance",
			uuid:        "MIG-GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f/3",
			expectError: true,
		},
		{
			description: "legacy format with non-numeric instance ids",
			uuid:        "MIG-GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f/a/b",
			expectError: true,
		},
		{
			description: "empty string",
			uuid:        "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			parent, gi, ci, err := parseMigDeviceUUID(tc.uuid)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedParent, parent)
			require.Equal(t, tc.expectedGi, gi)
			require.Equal(t, tc.expectedCi, ci)
		})
	}
}

// fakeNvmlLib is a minimal nvml.Interface test double; only
// DeviceGetHandleByUUID is used by getMigDeviceParts.
type fakeNvmlLib struct {
	nvml.Interface
	handle nvml.Device
	ret    nvml.Return
}

func (f *fakeNvmlLib) DeviceGetHandleByUUID(string) (nvml.Device, nvml.Return) {
	return f.handle, f.ret
}

// fakeMigHandle is a minimal nvml.Device test double for a MIG device handle.
type fakeMigHandle struct {
	nvml.Device
	parentUUID string
	gi         int
	ci         int
}

func (f *fakeMigHandle) GetDeviceHandleFromMigDeviceHandle() (nvml.Device, nvml.Return) {
	return &fakeParentHandle{uuid: f.parentUUID}, nvml.SUCCESS
}

func (f *fakeMigHandle) GetGpuInstanceId() (int, nvml.Return) {
	return f.gi, nvml.SUCCESS
}

func (f *fakeMigHandle) GetComputeInstanceId() (int, nvml.Return) {
	return f.ci, nvml.SUCCESS
}

type fakeParentHandle struct {
	nvml.Device
	uuid string
}

func (f *fakeParentHandle) GetUUID() (string, nvml.Return) {
	return f.uuid, nvml.SUCCESS
}

func TestGetMigDeviceParts(t *testing.T) {
	newMigDevice := func(uuid string) *Device {
		return &Device{
			Device: pluginapi.Device{ID: uuid},
			Index:  "0:0",
		}
	}

	testCases := []struct {
		description      string
		device           *Device
		nvmlRet          nvml.Return
		expectedParent   string
		expectedGi       uint32
		expectedCi       uint32
		expectError      bool
		expectedInErrMsg []string
	}{
		{
			description:    "placement resolved via NVML handle",
			device:         newMigDevice("MIG-30d00c09-8a98-59b8-8c1a-1d64b4ec3ad2"),
			nvmlRet:        nvml.SUCCESS,
			expectedParent: "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f",
			expectedGi:     3,
			expectedCi:     0,
		},
		{
			description:    "NVML lookup fails but legacy UUID format is parseable",
			device:         newMigDevice("MIG-GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f/3/0"),
			nvmlRet:        nvml.ERROR_NOT_SUPPORTED,
			expectedParent: "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f",
			expectedGi:     3,
			expectedCi:     0,
		},
		{
			description: "NVML lookup fails for opaque UUID: the NVML error is surfaced",
			device:      newMigDevice("MIG-30d00c09-8a98-59b8-8c1a-1d64b4ec3ad2"),
			nvmlRet:     nvml.ERROR_NO_PERMISSION,
			expectError: true,
			expectedInErrMsg: []string{
				"MIG-30d00c09-8a98-59b8-8c1a-1d64b4ec3ad2",
				nvml.ErrorString(nvml.ERROR_NO_PERMISSION),
			},
		},
		{
			description: "full device is rejected",
			device: &Device{
				Device: pluginapi.Device{ID: "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f"},
				Index:  "0",
			},
			nvmlRet:     nvml.SUCCESS,
			expectError: true,
			expectedInErrMsg: []string{
				"cannot get GI and CI of full device",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			r := &nvmlResourceManager{
				nvml: &fakeNvmlLib{
					handle: &fakeMigHandle{
						parentUUID: "GPU-5c89852c-d268-c3f3-1b07-005d5ae1dc3f",
						gi:         3,
						ci:         0,
					},
					ret: tc.nvmlRet,
				},
			}

			parent, gi, ci, err := r.getMigDeviceParts(tc.device)
			if tc.expectError {
				require.Error(t, err)
				for _, msg := range tc.expectedInErrMsg {
					require.Contains(t, err.Error(), msg)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectedParent, parent)
			require.Equal(t, tc.expectedGi, gi)
			require.Equal(t, tc.expectedCi, ci)
		})
	}
}
