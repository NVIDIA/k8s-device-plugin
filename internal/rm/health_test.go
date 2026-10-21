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
	"sync/atomic"
	"testing"
	"time"

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
			description:         "enabled overrides disabled is all",
			disabled:            "all",
			enabled:             "79",
			expectedAllDisabled: false,
			expectedContents: disabledXIDs{
				0:   true,
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				79:  false,
				109: true,
			},
			expectedDisabled: map[uint64]bool{
				13:  true,
				31:  true,
				43:  true,
				45:  true,
				68:  true,
				79:  false,
				109: true,
				555: true,
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

func TestGroupByParent(t *testing.T) {
	parentA := "GPU-A"
	parentB := "GPU-B"

	deviceA0 := &Device{Device: pluginapi.Device{ID: "GPU-A::0"}}
	deviceA1 := &Device{Device: pluginapi.Device{ID: "GPU-A::1"}}
	deviceB0 := &Device{Device: pluginapi.Device{ID: "GPU-B::0"}}

	grouped := groupByParent([]placedDevice{
		{parentUUID: parentA, device: deviceA0},
		{parentUUID: parentA, device: deviceA1},
		{parentUUID: parentB, device: deviceB0},
	})

	require.Equal(t, []*Device{deviceA0, deviceA1}, grouped[parentA])
	require.Equal(t, []*Device{deviceB0}, grouped[parentB])
}

func TestMatchesMigEvent(t *testing.T) {
	testCases := []struct {
		description string
		deviceGI    uint32
		deviceCI    uint32
		eventGI     uint32
		eventCI     uint32
		expected    bool
	}{
		{
			description: "GI and CI match",
			deviceGI:    3,
			deviceCI:    0,
			eventGI:     3,
			eventCI:     0,
			expected:    true,
		},
		{
			description: "only GI is specified and matches",
			deviceGI:    3,
			deviceCI:    0,
			eventGI:     3,
			eventCI:     0xFFFFFFFF,
			expected:    true,
		},
		{
			description: "only GI is specified and does not match",
			deviceGI:    5,
			deviceCI:    0,
			eventGI:     3,
			eventCI:     0xFFFFFFFF,
			expected:    false,
		},
		{
			description: "only CI is specified and matches",
			deviceGI:    3,
			deviceCI:    0,
			eventGI:     0xFFFFFFFF,
			eventCI:     0,
			expected:    true,
		},
		{
			description: "only CI is specified and does not match",
			deviceGI:    3,
			deviceCI:    1,
			eventGI:     0xFFFFFFFF,
			eventCI:     0,
			expected:    false,
		},
		{
			description: "neither GI nor CI is specified",
			deviceGI:    3,
			deviceCI:    0,
			eventGI:     0xFFFFFFFF,
			eventCI:     0xFFFFFFFF,
			expected:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expected, matchesMigEvent(tc.deviceGI, tc.deviceCI, tc.eventGI, tc.eventCI))
		})
	}
}

const testParentUUID = "GPU-P"

func TestHealthWatchSharesOneWaiterAcrossMixedMIGProfiles(t *testing.T) {
	lib := newHealthNvmlStub(&healthGPUStub{uuid: testParentUUID})
	watch := newHealthWatch(lib)

	stop4g := startHealth(t, &nvmlResourceManager{nvml: lib, watch: watch},
		Devices{"MIG-4g": migDevice("MIG-4g")}, make(chan *Device, 1))
	stop3g := startHealth(t, &nvmlResourceManager{nvml: lib, watch: watch},
		Devices{"MIG-3g": migDevice("MIG-3g")}, make(chan *Device, 1))
	waitForSubscribers(t, watch, 2)
	stop4g()
	stop3g()

	require.Equal(t, int32(1), lib.eventSetCreates.Load(), "mixed MIG plugins must share one event set")
	require.Equal(t, int32(1), lib.gpus[0].registerCount.Load(), "the parent GPU must be registered once")
	require.Equal(t, int32(1), lib.eventSet.freeCount.Load(), "the last subscriber must free the event set")
	require.Equal(t, lib.inits.Load(), lib.shutdowns.Load(), "every NVML Init must be paired with a Shutdown")
}

func TestHealthWatchDispatchesXidToMatchingMIGDevice(t *testing.T) {
	lib := newHealthNvmlStub(&healthGPUStub{uuid: testParentUUID})
	watch := newHealthWatch(lib)

	unhealthy4g := make(chan *Device, 1)
	unhealthy3g := make(chan *Device, 1)
	defer startHealth(t, &nvmlResourceManager{nvml: lib, watch: watch},
		Devices{"MIG-4g": migDevice("MIG-4g")}, unhealthy4g)()
	defer startHealth(t, &nvmlResourceManager{nvml: lib, watch: watch},
		Devices{"MIG-3g": migDevice("MIG-3g")}, unhealthy3g)()
	waitForSubscribers(t, watch, 2)

	lib.eventSet.events <- xidEvent(lib.gpus[0], 79, 1)

	requireUnhealthy(t, unhealthy4g, "MIG-4g")
	select {
	case d := <-unhealthy3g:
		t.Fatalf("non-matching MIG device was marked unhealthy: %s", d.ID)
	case <-time.After(50 * time.Millisecond):
	}
}

func xidEvent(gpu *healthGPUStub, xid uint64, gi uint32) healthEvent {
	return healthEvent{
		data: nvml.EventData{
			Device:            gpu,
			EventType:         nvml.EventTypeXidCriticalError,
			EventData:         xid,
			GpuInstanceId:     gi,
			ComputeInstanceId: 0,
		},
		ret: nvml.SUCCESS,
	}
}

func requireUnhealthy(t *testing.T, unhealthy <-chan *Device, id string) {
	t.Helper()
	select {
	case d := <-unhealthy:
		require.Equal(t, id, d.ID)
	case <-time.After(time.Second):
		t.Fatalf("device %s was not marked unhealthy", id)
	}
}

func waitForSubscribers(t *testing.T, watch *healthWatch, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		watch.mu.Lock()
		defer watch.mu.Unlock()
		return len(watch.subs) == count
	}, time.Second, 5*time.Millisecond)
}

func startHealth(t *testing.T, r *nvmlResourceManager, devices Devices, unhealthy chan<- *Device) func() {
	t.Helper()
	stop := make(chan any)
	done := make(chan error, 1)
	go func() {
		done <- r.checkHealth(stop, devices, unhealthy)
	}()
	return func() {
		close(stop)
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("checkHealth did not return")
		}
	}
}

func migDevice(id string) *Device {
	return &Device{
		Device: pluginapi.Device{ID: id},
		Index:  "0:0",
	}
}

// healthEventSetStub delivers events sent on its channel and otherwise times out quickly.
type healthEventSetStub struct {
	nvml.EventSet
	events    chan healthEvent
	freeCount atomic.Int32
}

func (e *healthEventSetStub) Wait(uint32) (nvml.EventData, nvml.Return) {
	select {
	case event := <-e.events:
		return event.data, event.ret
	case <-time.After(5 * time.Millisecond):
		return nvml.EventData{}, nvml.ERROR_TIMEOUT
	}
}

func (e *healthEventSetStub) Free() nvml.Return {
	e.freeCount.Add(1)
	return nvml.SUCCESS
}

type healthGPUStub struct {
	nvml.Device
	uuid          string
	registerCount atomic.Int32
}

func (g *healthGPUStub) GetSupportedEventTypes() (uint64, nvml.Return) {
	return uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError), nvml.SUCCESS
}

func (g *healthGPUStub) RegisterEvents(uint64, nvml.EventSet) nvml.Return {
	g.registerCount.Add(1)
	return nvml.SUCCESS
}

func (g *healthGPUStub) GetUUID() (string, nvml.Return) {
	return g.uuid, nvml.SUCCESS
}

type healthMigStub struct {
	nvml.Device
	parent *healthGPUStub
	gi     int
}

func (m *healthMigStub) GetDeviceHandleFromMigDeviceHandle() (nvml.Device, nvml.Return) {
	return m.parent, nvml.SUCCESS
}

func (m *healthMigStub) GetGpuInstanceId() (int, nvml.Return) {
	return m.gi, nvml.SUCCESS
}

func (m *healthMigStub) GetComputeInstanceId() (int, nvml.Return) {
	return 0, nvml.SUCCESS
}

type healthNvmlStub struct {
	nvml.Interface
	gpus            []*healthGPUStub
	migs            map[string]nvml.Device
	eventSet        *healthEventSetStub
	eventSetCreates atomic.Int32
	inits           atomic.Int32
	shutdowns       atomic.Int32
}

// newHealthNvmlStub returns an NVML stub with a 4g MIG device on GPU instance 1 and a 3g MIG
// device on GPU instance 2 of the first GPU.
func newHealthNvmlStub(gpus ...*healthGPUStub) *healthNvmlStub {
	return &healthNvmlStub{
		gpus: gpus,
		migs: map[string]nvml.Device{
			"MIG-4g": &healthMigStub{parent: gpus[0], gi: 1},
			"MIG-3g": &healthMigStub{parent: gpus[0], gi: 2},
		},
		eventSet: &healthEventSetStub{events: make(chan healthEvent)},
	}
}

func (n *healthNvmlStub) Init() nvml.Return {
	n.inits.Add(1)
	return nvml.SUCCESS
}

func (n *healthNvmlStub) Shutdown() nvml.Return {
	n.shutdowns.Add(1)
	return nvml.SUCCESS
}

func (n *healthNvmlStub) EventSetCreate() (nvml.EventSet, nvml.Return) {
	n.eventSetCreates.Add(1)
	return n.eventSet, nvml.SUCCESS
}

func (n *healthNvmlStub) DeviceGetCount() (int, nvml.Return) {
	return len(n.gpus), nvml.SUCCESS
}

func (n *healthNvmlStub) DeviceGetHandleByIndex(i int) (nvml.Device, nvml.Return) {
	return n.gpus[i], nvml.SUCCESS
}

func (n *healthNvmlStub) DeviceGetHandleByUUID(uuid string) (nvml.Device, nvml.Return) {
	if d, ok := n.migs[uuid]; ok {
		return d, nvml.SUCCESS
	}
	return nil, nvml.ERROR_NOT_FOUND
}
