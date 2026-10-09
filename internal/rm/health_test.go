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

func TestCheckHealthSharesEventSetAcrossMixedMIGProfiles(t *testing.T) {
	var inFlight, maxInFlight atomic.Int32
	eventSet := &healthEventSetStub{
		waitFn: func(uint32) (nvml.EventData, nvml.Return) {
			n := inFlight.Add(1)
			for {
				old := maxInFlight.Load()
				if n <= old || maxInFlight.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			inFlight.Add(-1)
			return nvml.EventData{}, nvml.ERROR_TIMEOUT
		},
	}

	parentUUID := "GPU-P"
	var registerCount atomic.Int32
	parent := &healthGPUStub{uuid: parentUUID, registerCount: &registerCount}
	nvmlStub := &healthNvmlStub{
		devices: map[string]nvml.Device{
			parentUUID: parent,
			"MIG-4g":   &healthMigStub{parent: parent, gi: 1, ci: 0},
			"MIG-3g":   &healthMigStub{parent: parent, gi: 2, ci: 0},
		},
		newEventSet: func() nvml.EventSet { return eventSet },
	}
	watch := newHealthWatch()

	stop4g := startHealth(t, newTestHealthResourceManager(nvmlStub, watch, "nvidia.com/mig-4g.47gb"),
		Devices{"MIG-4g": migDevice("MIG-4g", "0:0")}, make(chan *Device, 1))
	stop3g := startHealth(t, newTestHealthResourceManager(nvmlStub, watch, "nvidia.com/mig-3g.47gb"),
		Devices{"MIG-3g": migDevice("MIG-3g", "0:1")}, make(chan *Device, 1))
	time.Sleep(50 * time.Millisecond)
	stop4g()
	stop3g()

	require.Equal(t, int32(1), nvmlStub.eventSetCreates.Load(), "mixed MIG plugins must share one event set")
	require.Equal(t, int32(1), registerCount.Load(), "the parent GPU must be registered once")
	require.Equal(t, int32(1), maxInFlight.Load(), "a shared waiter must not overlap nvmlEventSetWait")
}

func TestCheckHealthDispatchesXidToMatchingMIGDevice(t *testing.T) {
	parentUUID := "GPU-P"
	ready := make(chan struct{})
	var sent atomic.Bool
	eventSet := &healthEventSetStub{
		waitFn: func(uint32) (nvml.EventData, nvml.Return) {
			select {
			case <-ready:
			case <-time.After(10 * time.Millisecond):
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			}
			if sent.CompareAndSwap(false, true) {
				return nvml.EventData{
					Device:            &healthGPUStub{uuid: parentUUID},
					EventType:         nvml.EventTypeXidCriticalError,
					EventData:         79,
					GpuInstanceId:     1,
					ComputeInstanceId: 0,
				}, nvml.SUCCESS
			}
			time.Sleep(10 * time.Millisecond)
			return nvml.EventData{}, nvml.ERROR_TIMEOUT
		},
	}

	parent := &healthGPUStub{uuid: parentUUID}
	nvmlStub := &healthNvmlStub{
		devices: map[string]nvml.Device{
			parentUUID: parent,
			"MIG-4g":   &healthMigStub{parent: parent, gi: 1, ci: 0},
			"MIG-3g":   &healthMigStub{parent: parent, gi: 2, ci: 0},
		},
		newEventSet: func() nvml.EventSet { return eventSet },
	}
	watch := newHealthWatch()

	unhealthy4g := make(chan *Device, 1)
	unhealthy3g := make(chan *Device, 1)
	stop4g := startHealth(t, newTestHealthResourceManager(nvmlStub, watch, "nvidia.com/mig-4g.47gb"),
		Devices{"MIG-4g": migDevice("MIG-4g", "0:0")}, unhealthy4g)
	stop3g := startHealth(t, newTestHealthResourceManager(nvmlStub, watch, "nvidia.com/mig-3g.47gb"),
		Devices{"MIG-3g": migDevice("MIG-3g", "0:1")}, unhealthy3g)
	defer stop4g()
	defer stop3g()

	require.Eventually(t, func() bool {
		watch.mu.Lock()
		defer watch.mu.Unlock()
		return len(watch.subs) == 2
	}, time.Second, 5*time.Millisecond)

	close(ready)

	select {
	case d := <-unhealthy4g:
		require.Equal(t, "MIG-4g", d.ID)
	case <-time.After(time.Second):
		t.Fatal("matching MIG device was not marked unhealthy")
	}
	select {
	case d := <-unhealthy3g:
		t.Fatalf("non-matching MIG device was marked unhealthy: %s", d.ID)
	default:
	}
}

func testHealthConfig() *spec.Config {
	fail := true
	return &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				FailOnInitError: &fail,
			},
		},
	}
}

func newTestHealthResourceManager(nvmllib nvml.Interface, watch *healthWatch, resource spec.ResourceName) *nvmlResourceManager {
	return &nvmlResourceManager{
		resourceManager: resourceManager{
			config:   testHealthConfig(),
			resource: resource,
		},
		nvml:  nvmllib,
		watch: watch,
	}
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

func migDevice(id, index string) *Device {
	return &Device{
		Device: pluginapi.Device{ID: id},
		Index:  index,
	}
}

type healthEventSetStub struct {
	nvml.EventSet
	waitFn    func(uint32) (nvml.EventData, nvml.Return)
	freeCount atomic.Int32
}

func (e *healthEventSetStub) Wait(timeout uint32) (nvml.EventData, nvml.Return) {
	if e.waitFn != nil {
		return e.waitFn(timeout)
	}
	return nvml.EventData{}, nvml.ERROR_TIMEOUT
}

func (e *healthEventSetStub) Free() nvml.Return {
	e.freeCount.Add(1)
	return nvml.SUCCESS
}

type healthGPUStub struct {
	nvml.Device
	uuid          string
	registerCount *atomic.Int32
	registerRet   nvml.Return
}

func (g *healthGPUStub) GetSupportedEventTypes() (uint64, nvml.Return) {
	return uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError), nvml.SUCCESS
}

func (g *healthGPUStub) RegisterEvents(uint64, nvml.EventSet) nvml.Return {
	if g.registerCount != nil {
		g.registerCount.Add(1)
	}
	if g.registerRet != nvml.SUCCESS {
		return g.registerRet
	}
	return nvml.SUCCESS
}

func (g *healthGPUStub) GetUUID() (string, nvml.Return) {
	return g.uuid, nvml.SUCCESS
}

type healthMigStub struct {
	nvml.Device
	parent *healthGPUStub
	gi     int
	ci     int
}

func (m *healthMigStub) GetDeviceHandleFromMigDeviceHandle() (nvml.Device, nvml.Return) {
	return m.parent, nvml.SUCCESS
}

func (m *healthMigStub) GetGpuInstanceId() (int, nvml.Return) {
	return m.gi, nvml.SUCCESS
}

func (m *healthMigStub) GetComputeInstanceId() (int, nvml.Return) {
	return m.ci, nvml.SUCCESS
}

type healthNvmlStub struct {
	nvml.Interface
	devices         map[string]nvml.Device
	newEventSet     func() nvml.EventSet
	eventSetCreates atomic.Int32
}

func (n *healthNvmlStub) Init() nvml.Return { return nvml.SUCCESS }

func (n *healthNvmlStub) Shutdown() nvml.Return { return nvml.SUCCESS }

func (n *healthNvmlStub) EventSetCreate() (nvml.EventSet, nvml.Return) {
	n.eventSetCreates.Add(1)
	if n.newEventSet != nil {
		return n.newEventSet(), nvml.SUCCESS
	}
	return &healthEventSetStub{}, nvml.SUCCESS
}

func (n *healthNvmlStub) DeviceGetHandleByUUID(uuid string) (nvml.Device, nvml.Return) {
	if d, ok := n.devices[uuid]; ok {
		return d, nvml.SUCCESS
	}
	return nil, nvml.ERROR_NOT_FOUND
}
