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
	"sync"
	"testing"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"k8s.io/utils/ptr"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
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

// fakeHealthEventSet is a test double for nvml.EventSet that yields mock events sequentially.
type fakeHealthEventSet struct {
	nvml.EventSet
	events []nvml.EventData
	index  int
	mu     sync.Mutex
}

func (f *fakeHealthEventSet) Wait(timeout uint32) (nvml.EventData, nvml.Return) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index < len(f.events) {
		e := f.events[f.index]
		f.index++
		return e, nvml.SUCCESS
	}
	time.Sleep(10 * time.Millisecond)
	return nvml.EventData{}, nvml.ERROR_TIMEOUT
}

func (f *fakeHealthEventSet) Free() nvml.Return {
	return nvml.SUCCESS
}

// fakeHealthDevice is a test double for nvml.Device supporting UUID, supported events, and MIG hierarchy.
type fakeHealthDevice struct {
	nvml.Device
	uuid              string
	parentUUID        string
	gi                int
	ci                int
	supportedEvents   uint64
	registerCallCount int
	mu                sync.Mutex
}

func (f *fakeHealthDevice) GetUUID() (string, nvml.Return) {
	return f.uuid, nvml.SUCCESS
}

func (f *fakeHealthDevice) GetSupportedEventTypes() (uint64, nvml.Return) {
	events := f.supportedEvents
	if events == 0 {
		events = uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)
	}
	return events, nvml.SUCCESS
}

func (f *fakeHealthDevice) RegisterEvents(mask uint64, set nvml.EventSet) nvml.Return {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCallCount++
	return nvml.SUCCESS
}

func (f *fakeHealthDevice) GetDeviceHandleFromMigDeviceHandle() (nvml.Device, nvml.Return) {
	return &fakeHealthDevice{uuid: f.parentUUID}, nvml.SUCCESS
}

func (f *fakeHealthDevice) GetGpuInstanceId() (int, nvml.Return) {
	return f.gi, nvml.SUCCESS
}

func (f *fakeHealthDevice) GetComputeInstanceId() (int, nvml.Return) {
	return f.ci, nvml.SUCCESS
}

// fakeHealthNvmlLib is a test double for nvml.Interface providing handle lookup and event set creation.
type fakeHealthNvmlLib struct {
	nvml.Interface
	devices  map[string]nvml.Device
	eventSet nvml.EventSet
}

func (f *fakeHealthNvmlLib) Init() nvml.Return     { return nvml.SUCCESS }
func (f *fakeHealthNvmlLib) Shutdown() nvml.Return { return nvml.SUCCESS }
func (f *fakeHealthNvmlLib) EventSetCreate() (nvml.EventSet, nvml.Return) {
	return f.eventSet, nvml.SUCCESS
}
func (f *fakeHealthNvmlLib) DeviceGetHandleByUUID(uuid string) (nvml.Device, nvml.Return) {
	if d, ok := f.devices[uuid]; ok {
		return d, nvml.SUCCESS
	}
	return nil, nvml.ERROR_NOT_FOUND
}

func TestCheckHealthReplicatedDevicesAllMarkedUnhealthy(t *testing.T) {
	parentUUID := "GPU-12345678-1234-1234-1234-123456789abc"
	dev0 := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::0", Health: pluginapi.Healthy},
		Index:  "0",
	}
	dev1 := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::1", Health: pluginapi.Healthy},
		Index:  "0",
	}
	dev2 := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::2", Health: pluginapi.Healthy},
		Index:  "0",
	}
	dev3 := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::3", Health: pluginapi.Healthy},
		Index:  "0",
	}

	mockDevice := &fakeHealthDevice{uuid: parentUUID}
	eventSet := &fakeHealthEventSet{
		events: []nvml.EventData{
			{
				Device:            mockDevice,
				EventType:         nvml.EventTypeXidCriticalError,
				EventData:         79, // Critical Xid error
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
		},
	}

	nvmllib := &fakeHealthNvmlLib{
		devices: map[string]nvml.Device{
			parentUUID: mockDevice,
		},
		eventSet: eventSet,
	}

	r := &nvmlResourceManager{
		resourceManager: resourceManager{
			config: &spec.Config{
				Flags: spec.Flags{
					CommandLineFlags: spec.CommandLineFlags{
						FailOnInitError: ptr.To(true),
					},
				},
			},
		},
		nvml: nvmllib,
	}

	stop := make(chan interface{})
	unhealthy := make(chan *Device, 10)
	defer close(stop)

	devices := Devices{
		dev0.ID: dev0,
		dev1.ID: dev1,
		dev2.ID: dev2,
		dev3.ID: dev3,
	}

	go func() {
		_ = r.checkHealth(stop, devices, unhealthy)
	}()

	var received []string
	timeout := time.After(2 * time.Second)
	for len(received) < 4 {
		select {
		case d := <-unhealthy:
			received = append(received, d.ID)
		case <-timeout:
			t.Fatalf("timed out waiting for unhealthy devices, received %d of 4: %v", len(received), received)
		}
	}

	require.ElementsMatch(t, []string{dev0.ID, dev1.ID, dev2.ID, dev3.ID}, received)
	require.Equal(t, 1, mockDevice.registerCallCount, "RegisterEvents should be called exactly once per unique parent UUID")
}

func TestCheckHealthMultiplePhysicalGPUs(t *testing.T) {
	parentUUID1 := "GPU-aaaa-1111"
	parentUUID2 := "GPU-bbbb-2222"

	dev1a := &Device{
		Device: pluginapi.Device{ID: parentUUID1 + "::0", Health: pluginapi.Healthy},
		Index:  "0",
	}
	dev1b := &Device{
		Device: pluginapi.Device{ID: parentUUID1 + "::1", Health: pluginapi.Healthy},
		Index:  "0",
	}
	dev2a := &Device{
		Device: pluginapi.Device{ID: parentUUID2 + "::0", Health: pluginapi.Healthy},
		Index:  "1",
	}
	dev2b := &Device{
		Device: pluginapi.Device{ID: parentUUID2 + "::1", Health: pluginapi.Healthy},
		Index:  "1",
	}

	mockDevice1 := &fakeHealthDevice{uuid: parentUUID1}
	mockDevice2 := &fakeHealthDevice{uuid: parentUUID2}

	eventSet := &fakeHealthEventSet{
		events: []nvml.EventData{
			{
				Device:            mockDevice1,
				EventType:         nvml.EventTypeXidCriticalError,
				EventData:         79,
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
		},
	}

	nvmllib := &fakeHealthNvmlLib{
		devices: map[string]nvml.Device{
			parentUUID1: mockDevice1,
			parentUUID2: mockDevice2,
		},
		eventSet: eventSet,
	}

	r := &nvmlResourceManager{
		resourceManager: resourceManager{
			config: &spec.Config{
				Flags: spec.Flags{
					CommandLineFlags: spec.CommandLineFlags{
						FailOnInitError: ptr.To(true),
					},
				},
			},
		},
		nvml: nvmllib,
	}

	stop := make(chan interface{})
	unhealthy := make(chan *Device, 10)
	defer close(stop)

	devices := Devices{
		dev1a.ID: dev1a,
		dev1b.ID: dev1b,
		dev2a.ID: dev2a,
		dev2b.ID: dev2b,
	}

	go func() {
		_ = r.checkHealth(stop, devices, unhealthy)
	}()

	var received []string
	timeout := time.After(2 * time.Second)
	for len(received) < 2 {
		select {
		case d := <-unhealthy:
			received = append(received, d.ID)
		case <-timeout:
			t.Fatalf("timed out waiting for unhealthy devices, received %d of 2: %v", len(received), received)
		}
	}

	require.ElementsMatch(t, []string{dev1a.ID, dev1b.ID}, received)

	// Ensure no unexpected devices from GPU-2 are sent
	select {
	case unexpected := <-unhealthy:
		t.Fatalf("unexpected device received on unhealthy channel: %v", unexpected.ID)
	case <-time.After(100 * time.Millisecond):
		// Success
	}
}

func TestCheckHealthMigPlacementAndGlobalError(t *testing.T) {
	parentUUID := "GPU-MIG-PARENT-1234"
	mig1 := &Device{
		Device: pluginapi.Device{ID: "MIG-GPU-MIG-PARENT-1234/1/0", Health: pluginapi.Healthy},
		Index:  "0:0",
	}
	mig2 := &Device{
		Device: pluginapi.Device{ID: "MIG-GPU-MIG-PARENT-1234/2/0", Health: pluginapi.Healthy},
		Index:  "0:1",
	}

	mockParent := &fakeHealthDevice{uuid: parentUUID}

	t.Run("GI-specific error marks only affected MIG instance unhealthy", func(t *testing.T) {
		eventSet := &fakeHealthEventSet{
			events: []nvml.EventData{
				{
					Device:            mockParent,
					EventType:         nvml.EventTypeXidCriticalError,
					EventData:         79,
					GpuInstanceId:     1,
					ComputeInstanceId: 0,
				},
			},
		}

		nvmllib := &fakeHealthNvmlLib{
			devices: map[string]nvml.Device{
				parentUUID: mockParent,
			},
			eventSet: eventSet,
		}

		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config: &spec.Config{
					Flags: spec.Flags{
						CommandLineFlags: spec.CommandLineFlags{
							FailOnInitError: ptr.To(true),
						},
					},
				},
			},
			nvml: nvmllib,
		}

		stop := make(chan interface{})
		unhealthy := make(chan *Device, 10)
		defer close(stop)

		devices := Devices{
			mig1.ID: mig1,
			mig2.ID: mig2,
		}

		go func() {
			_ = r.checkHealth(stop, devices, unhealthy)
		}()

		select {
		case d := <-unhealthy:
			require.Equal(t, mig1.ID, d.ID)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for unhealthy MIG device")
		}

		// Ensure mig2 was not marked unhealthy
		select {
		case unexpected := <-unhealthy:
			t.Fatalf("unexpected device marked unhealthy: %v", unexpected.ID)
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("Global GPU error marks all MIG instances on parent GPU unhealthy", func(t *testing.T) {
		eventSet := &fakeHealthEventSet{
			events: []nvml.EventData{
				{
					Device:            mockParent,
					EventType:         nvml.EventTypeXidCriticalError,
					EventData:         79,
					GpuInstanceId:     0xFFFFFFFF,
					ComputeInstanceId: 0xFFFFFFFF,
				},
			},
		}

		nvmllib := &fakeHealthNvmlLib{
			devices: map[string]nvml.Device{
				parentUUID: mockParent,
			},
			eventSet: eventSet,
		}

		r := &nvmlResourceManager{
			resourceManager: resourceManager{
				config: &spec.Config{
					Flags: spec.Flags{
						CommandLineFlags: spec.CommandLineFlags{
							FailOnInitError: ptr.To(true),
						},
					},
				},
			},
			nvml: nvmllib,
		}

		stop := make(chan interface{})
		unhealthy := make(chan *Device, 10)
		defer close(stop)

		devices := Devices{
			mig1.ID: mig1,
			mig2.ID: mig2,
		}

		go func() {
			_ = r.checkHealth(stop, devices, unhealthy)
		}()

		var received []string
		timeout := time.After(2 * time.Second)
		for len(received) < 2 {
			select {
			case d := <-unhealthy:
				received = append(received, d.ID)
			case <-timeout:
				t.Fatalf("timed out waiting for unhealthy MIG devices, received %d of 2: %v", len(received), received)
			}
		}

		require.ElementsMatch(t, []string{mig1.ID, mig2.ID}, received)
	})
}

func TestCheckHealthIgnoredXids(t *testing.T) {
	parentUUID := "GPU-IGNORED-TEST"
	dev := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::0", Health: pluginapi.Healthy},
		Index:  "0",
	}

	mockDevice := &fakeHealthDevice{uuid: parentUUID}
	eventSet := &fakeHealthEventSet{
		events: []nvml.EventData{
			{
				Device:            mockDevice,
				EventType:         nvml.EventTypeXidCriticalError,
				EventData:         31, // Application error (memory page fault) - default ignored
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
			{
				Device:            mockDevice,
				EventType:         nvml.EventTypeXidCriticalError,
				EventData:         43, // Application error (GPU stopped processing) - default ignored
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
		},
	}

	nvmllib := &fakeHealthNvmlLib{
		devices: map[string]nvml.Device{
			parentUUID: mockDevice,
		},
		eventSet: eventSet,
	}

	r := &nvmlResourceManager{
		resourceManager: resourceManager{
			config: &spec.Config{
				Flags: spec.Flags{
					CommandLineFlags: spec.CommandLineFlags{
						FailOnInitError: ptr.To(true),
					},
				},
			},
		},
		nvml: nvmllib,
	}

	stop := make(chan interface{})
	unhealthy := make(chan *Device, 10)
	defer close(stop)

	go func() {
		_ = r.checkHealth(stop, Devices{dev.ID: dev}, unhealthy)
	}()

	select {
	case unexpected := <-unhealthy:
		t.Fatalf("ignored Xid should not mark device unhealthy: %v", unexpected.ID)
	case <-time.After(200 * time.Millisecond):
		// Success
	}
}

func TestCheckHealthUnexpectedDevice(t *testing.T) {
	parentUUID := "GPU-EXPECTED"
	dev := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::0", Health: pluginapi.Healthy},
		Index:  "0",
	}

	mockDevice := &fakeHealthDevice{uuid: parentUUID}
	mockUnknownDevice := &fakeHealthDevice{uuid: "GPU-UNKNOWN"}

	eventSet := &fakeHealthEventSet{
		events: []nvml.EventData{
			{
				Device:            mockUnknownDevice,
				EventType:         nvml.EventTypeXidCriticalError,
				EventData:         79,
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
		},
	}

	nvmllib := &fakeHealthNvmlLib{
		devices: map[string]nvml.Device{
			parentUUID: mockDevice,
		},
		eventSet: eventSet,
	}

	r := &nvmlResourceManager{
		resourceManager: resourceManager{
			config: &spec.Config{
				Flags: spec.Flags{
					CommandLineFlags: spec.CommandLineFlags{
						FailOnInitError: ptr.To(true),
					},
				},
			},
		},
		nvml: nvmllib,
	}

	stop := make(chan interface{})
	unhealthy := make(chan *Device, 10)
	defer close(stop)

	go func() {
		_ = r.checkHealth(stop, Devices{dev.ID: dev}, unhealthy)
	}()

	select {
	case unexpected := <-unhealthy:
		t.Fatalf("unexpected device should not mark device unhealthy: %v", unexpected.ID)
	case <-time.After(200 * time.Millisecond):
		// Success
	}
}

func TestCheckHealthNonCriticalEventType(t *testing.T) {
	parentUUID := "GPU-NON-CRITICAL"
	dev := &Device{
		Device: pluginapi.Device{ID: parentUUID + "::0", Health: pluginapi.Healthy},
		Index:  "0",
	}

	mockDevice := &fakeHealthDevice{uuid: parentUUID}
	eventSet := &fakeHealthEventSet{
		events: []nvml.EventData{
			{
				Device:            mockDevice,
				EventType:         nvml.EventTypeSingleBitEccError,
				EventData:         0,
				GpuInstanceId:     0xFFFFFFFF,
				ComputeInstanceId: 0xFFFFFFFF,
			},
		},
	}

	nvmllib := &fakeHealthNvmlLib{
		devices: map[string]nvml.Device{
			parentUUID: mockDevice,
		},
		eventSet: eventSet,
	}

	r := &nvmlResourceManager{
		resourceManager: resourceManager{
			config: &spec.Config{
				Flags: spec.Flags{
					CommandLineFlags: spec.CommandLineFlags{
						FailOnInitError: ptr.To(true),
					},
				},
			},
		},
		nvml: nvmllib,
	}

	stop := make(chan interface{})
	unhealthy := make(chan *Device, 10)
	defer close(stop)

	go func() {
		_ = r.checkHealth(stop, Devices{dev.ID: dev}, unhealthy)
	}()

	select {
	case unexpected := <-unhealthy:
		t.Fatalf("non-critical event type should not mark device unhealthy: %v", unexpected.ID)
	case <-time.After(200 * time.Millisecond):
		// Success
	}
}
