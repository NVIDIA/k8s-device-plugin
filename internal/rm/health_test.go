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
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock/dgxa100"
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

func TestCheckHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	unhealthy := make(chan *Device)

	server := dgxa100.New()

	deviceMock := server.Devices[0].(*dgxa100.Device)
	deviceMock.GetSupportedEventTypesFunc = func() (uint64, nvml.Return) {
		return nvml.EventTypeXidCriticalError, nvml.SUCCESS
	}
	// TODO: Should this be more dynamic?
	deviceMock.RegisterEventsFunc = func(v uint64, eventSet nvml.EventSet) nvml.Return {
		return nvml.SUCCESS
	}

	var count int
	eventData := []nvml.EventData{
		{
			EventData: 109,
			EventType: nvml.EventTypeXidCriticalError,
			Device:    server.Devices[0],
		},
		{
			EventData: 48,
			EventType: nvml.EventTypeXidCriticalError,
			Device:    server.Devices[0],
		},
	}

	server.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		es := &mock.EventSet{
			WaitFunc: func(v uint32) (nvml.EventData, nvml.Return) {
				ed := eventData[count%len(eventData)]
				count++
				if count == len(eventData) {
					// Cancel the context to signal the health checker to terminate
					// after the predefined events have been triggered.
					cancel()
				}
				return ed, nvml.SUCCESS
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return es, nvml.SUCCESS
	}

	r := &nvmlResourceManager{
		nvml: server,
	}

	var unhealthyDevices []*Device
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for d := range unhealthy {
			unhealthyDevices = append(unhealthyDevices, d)
		}
	}()

	var expectedDevices []*Device

	devices := make(Devices)
	for i, d := range server.Devices {
		device, err := BuildDevice(newNvmlGPUDevice(i, d))
		require.NoError(t, err)
		devices[device.GetUUID()] = device
		expectedDevices = append(expectedDevices, device)
		// TODO: We only expect a single unhealthy event for the first device.
		break
	}

	err := r.checkHealth(ctx, devices, unhealthy)
	require.ErrorIs(t, err, context.Canceled)

	// Close the channel and wait for the goroutine to finish collecting unhealthy devices
	close(unhealthy)
	wg.Wait()

	require.EqualValues(t, expectedDevices, unhealthyDevices)
}

func TestRegisterDeviceEventsNotSupported(t *testing.T) {
	// This test verifies that devices which return ERROR_NOT_SUPPORTED from
	// RegisterEvents are NOT marked as unhealthy. This is the expected behavior
	// for old GPUs that don't support event registration.
	// See: commit 8cd14472a "Fix healthchecking on old devices"

	unhealthy := make(chan *Device, 10)

	server := dgxa100.New()

	deviceMock := server.Devices[0].(*dgxa100.Device)
	deviceMock.GetSupportedEventTypesFunc = func() (uint64, nvml.Return) {
		return nvml.EventTypeXidCriticalError, nvml.SUCCESS
	}
	// Simulate an old device that doesn't support event registration
	deviceMock.RegisterEventsFunc = func(v uint64, eventSet nvml.EventSet) nvml.Return {
		return nvml.ERROR_NOT_SUPPORTED
	}

	eventSet := &mock.EventSet{
		FreeFunc: func() nvml.Return {
			return nvml.SUCCESS
		},
	}

	devices := make(Devices)
	parentToDeviceMap := make(map[string]*Device)

	for i, d := range server.Devices {
		device, err := BuildDevice(newNvmlGPUDevice(i, d))
		require.NoError(t, err)
		devices[device.GetUUID()] = device
		parentToDeviceMap[device.GetUUID()] = device
		break
	}

	p := nvmlHealthProvider{
		nvmllib:           server,
		devices:           devices,
		unhealthy:         unhealthy,
		parentToDeviceMap: parentToDeviceMap,
		deviceIDToGiMap:   make(map[string]uint32),
		deviceIDToCiMap:   make(map[string]uint32),
	}

	p.registerDeviceEvents(eventSet)

	// Close the channel so we can drain it
	close(unhealthy)

	// Verify that NO devices were marked as unhealthy
	var unhealthyDevices []*Device
	for d := range unhealthy {
		unhealthyDevices = append(unhealthyDevices, d)
	}

	require.Empty(t, unhealthyDevices, "Devices returning ERROR_NOT_SUPPORTED should NOT be marked as unhealthy")
}
