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
	"time"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock"
	"github.com/NVIDIA/go-nvml/pkg/nvml/mock/dgxa100"

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

// Helper function to create a test resource manager with mock NVML
func newMockResourceManager(t *testing.T, mockNVML nvml.Interface, deviceCount int) *nvmlResourceManager {
	t.Helper()

	_ = device.New(mockNVML)

	// Create minimal config
	failOnInitError := false
	config := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				FailOnInitError: &failOnInitError,
			},
		},
	}

	// Build device map with UUIDs matching the mock server
	devices := make(Devices)

	// If mockNVML is a dgxa100 server, use its device UUIDs
	if server, ok := mockNVML.(*dgxa100.Server); ok {
		for i := 0; i < deviceCount && i < len(server.Devices); i++ {
			device := server.Devices[i].(*dgxa100.Device)
			deviceID := device.UUID
			devices[deviceID] = &Device{
				Device: pluginapi.Device{
					ID:     deviceID,
					Health: pluginapi.Healthy,
				},
				Index: fmt.Sprintf("%d", i),
			}
		}
	} else {
		// Fallback for non-dgxa100 mocks
		for i := 0; i < deviceCount; i++ {
			deviceID := fmt.Sprintf("GPU-%d", i)
			devices[deviceID] = &Device{
				Device: pluginapi.Device{
					ID:     deviceID,
					Health: pluginapi.Healthy,
				},
				Index: fmt.Sprintf("%d", i),
			}
		}
	}

	return &nvmlResourceManager{
		resourceManager: resourceManager{
			config:   config,
			resource: "nvidia.com/gpu",
			devices:  devices,
		},
		nvml: mockNVML,
	}
}

// mockDGXA100Setup configures the dgxa100 mock with common overrides
func mockDGXA100Setup(server *dgxa100.Server) {
	for i, d := range server.Devices {
		device := d.(*dgxa100.Device)
		device.GetIndexFunc = func(idx int) func() (int, nvml.Return) {
			return func() (int, nvml.Return) {
				return idx, nvml.SUCCESS
			}
		}(i)
		device.GetUUIDFunc = func(uuid string) func() (string, nvml.Return) {
			return func() (string, nvml.Return) {
				return uuid, nvml.SUCCESS
			}
		}(device.UUID)

		// Setup GetSupportedEventTypes - all devices support all event types
		device.GetSupportedEventTypesFunc = func() (uint64, nvml.Return) {
			return uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError), nvml.SUCCESS
		}

		// Setup RegisterEvents - succeed by default
		device.RegisterEventsFunc = func(u uint64, es nvml.EventSet) nvml.Return {
			return nvml.SUCCESS
		}
	}

	// Setup DeviceGetHandleByUUID to return the correct device
	server.DeviceGetHandleByUUIDFunc = func(uuid string) (nvml.Device, nvml.Return) {
		for _, d := range server.Devices {
			device := d.(*dgxa100.Device)
			if device.UUID == uuid {
				return device, nvml.SUCCESS
			}
		}
		return nil, nvml.ERROR_INVALID_ARGUMENT
	}
}

// Test 1: Buffered Channel Capacity
func TestCheckHealth_Phase1_BufferedChannelCapacity(t *testing.T) {
	healthChan := make(chan *Device, 64)
	require.Equal(t, 64, cap(healthChan), "Health channel should have capacity 64")
}

// Test 2: sendUnhealthyDevice - Successful Send
func TestSendUnhealthyDevice_Success(t *testing.T) {
	healthChan := make(chan *Device, 64)
	device := &Device{
		Device: pluginapi.Device{
			ID:     "GPU-0",
			Health: pluginapi.Healthy,
		},
	}

	sendUnhealthyDevice(healthChan, device)

	select {
	case d := <-healthChan:
		require.Equal(t, "GPU-0", d.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Device not sent to channel")
	}
}

// Test 3: sendUnhealthyDevice - Channel Full
func TestSendUnhealthyDevice_Phase1_ChannelFull(t *testing.T) {
	healthChan := make(chan *Device, 2)

	// Fill the channel
	healthChan <- &Device{Device: pluginapi.Device{ID: "dummy1"}}
	healthChan <- &Device{Device: pluginapi.Device{ID: "dummy2"}}

	device := &Device{
		Device: pluginapi.Device{
			ID:     "GPU-0",
			Health: pluginapi.Healthy,
		},
	}

	// This should not block
	done := make(chan bool)
	go func() {
		sendUnhealthyDevice(healthChan, device)
		done <- true
	}()

	select {
	case <-done:
		// Good - didn't block
		require.Equal(t, pluginapi.Unhealthy, device.Health,
			"Device health should be updated directly when channel is full")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sendUnhealthyDevice blocked on full channel")
	}
}

// Test 4: Graceful Stop Signal
func TestCheckHealth_Phase1_GracefulStop(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	// Mock EventSet that always times out (quiet system)
	mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		eventSet := &mock.EventSet{
			WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return eventSet, nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 8)

	healthChan := make(chan *Device, 64)
	stopChan := make(chan interface{})

	// Start checkHealth
	errChan := make(chan error, 1)
	go func() {
		errChan <- rm.checkHealth(stopChan, rm.devices, healthChan)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Send stop signal
	stopTime := time.Now()
	close(stopChan)

	// Should stop quickly
	select {
	case err := <-errChan:
		elapsed := time.Since(stopTime)
		require.NoError(t, err, "checkHealth should stop cleanly")
		require.Less(t, elapsed.Milliseconds(), int64(500),
			"Should stop within 500ms, took %v", elapsed)
		t.Logf("✓ Stopped cleanly in %v", elapsed)
	case <-time.After(1 * time.Second):
		t.Fatal("checkHealth did not stop within 1 second")
	}
}

// Test 5: XID Event Processing
func TestCheckHealth_Phase1_XIDEventProcessing(t *testing.T) {
	testCases := []struct {
		name         string
		xid          uint64
		expectMarked bool
		disableXIDs  string
	}{
		{
			name:         "Critical XID 79 marks unhealthy",
			xid:          79, // GPU fallen off bus
			expectMarked: true,
		},
		{
			name:         "Application XID 13 ignored (default)",
			xid:          13, // Graphics engine exception (default ignored)
			expectMarked: false,
		},
		{
			name:         "Critical XID 48 marks unhealthy",
			xid:          48, // DBE error
			expectMarked: true,
		},
		{
			name:         "Disabled XID not marked",
			xid:          79,
			disableXIDs:  "79",
			expectMarked: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.disableXIDs != "" {
				t.Setenv(envDisableHealthChecks, tc.disableXIDs)
			}

			mockNVML := dgxa100.New()
			mockDGXA100Setup(mockNVML)

			eventSent := false
			mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
				eventSet := &mock.EventSet{
					WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
						if !eventSent {
							eventSent = true
							return nvml.EventData{
								EventType: nvml.EventTypeXidCriticalError,
								EventData: tc.xid,
								Device:    mockNVML.Devices[0],
							}, nvml.SUCCESS
						}
						// After one event, just timeout
						return nvml.EventData{}, nvml.ERROR_TIMEOUT
					},
					FreeFunc: func() nvml.Return {
						return nvml.SUCCESS
					},
				}
				return eventSet, nvml.SUCCESS
			}

			rm := newMockResourceManager(t, mockNVML, 8)

			healthChan := make(chan *Device, 64)
			stopChan := make(chan interface{})

			go func() {
				_ = rm.checkHealth(stopChan, rm.devices, healthChan)
			}()

			// Wait for event processing
			time.Sleep(200 * time.Millisecond)
			close(stopChan)

			// Check if device was marked unhealthy
			select {
			case d := <-healthChan:
				if tc.expectMarked {
					require.NotNil(t, d, "Expected device to be marked unhealthy")
					t.Logf("✓ Device %s correctly marked unhealthy for XID-%d", d.ID, tc.xid)
				} else {
					t.Fatalf("Device marked unhealthy but shouldn't be for XID-%d", tc.xid)
				}
			case <-time.After(300 * time.Millisecond):
				if tc.expectMarked {
					t.Fatalf("Expected device to be marked unhealthy for XID-%d", tc.xid)
				} else {
					t.Logf("✓ Correctly ignored XID-%d", tc.xid)
				}
			}
		})
	}
}

// Test 6: Error Handling - GPU Lost
func TestCheckHealth_Phase1_ErrorHandling_GPULost(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	errorSent := false
	mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		eventSet := &mock.EventSet{
			WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
				if !errorSent {
					errorSent = true
					// Simulate GPU lost error
					return nvml.EventData{}, nvml.ERROR_GPU_IS_LOST
				}
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return eventSet, nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 8)

	healthChan := make(chan *Device, 64)
	stopChan := make(chan interface{})

	go func() {
		_ = rm.checkHealth(stopChan, rm.devices, healthChan)
	}()

	// Wait for error processing
	time.Sleep(200 * time.Millisecond)

	// Drain channel and count unhealthy devices
	unhealthyCount := 0
	timeout := time.After(500 * time.Millisecond)
drainLoop:
	for {
		select {
		case <-healthChan:
			unhealthyCount++
		case <-timeout:
			break drainLoop
		}
	}

	close(stopChan)

	require.Equal(t, len(rm.devices), unhealthyCount,
		"All %d devices should be marked unhealthy on GPU_LOST error, got %d",
		len(rm.devices), unhealthyCount)
	t.Logf("✓ All %d devices correctly marked unhealthy on GPU_LOST", unhealthyCount)
}

// Test 7: Error Handling - Transient Errors
func TestCheckHealth_Phase1_ErrorHandling_Transient(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	errorSent := false
	mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		eventSet := &mock.EventSet{
			WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
				if !errorSent {
					errorSent = true
					// Simulate transient error
					return nvml.EventData{}, nvml.ERROR_UNKNOWN
				}
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return eventSet, nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 8)

	healthChan := make(chan *Device, 64)
	stopChan := make(chan interface{})

	go func() {
		_ = rm.checkHealth(stopChan, rm.devices, healthChan)
	}()

	// Wait for error processing
	time.Sleep(200 * time.Millisecond)
	close(stopChan)

	// Should not mark devices unhealthy for transient errors
	select {
	case d := <-healthChan:
		t.Fatalf("Device %s marked unhealthy for transient error, but shouldn't be", d.ID)
	case <-time.After(300 * time.Millisecond):
		t.Log("✓ Correctly handled transient error without marking devices unhealthy")
	}
}

// Test 8: Stats Collection
func TestCheckHealth_StatsCollection(t *testing.T) {
	stats := &healthCheckStats{
		startTime: time.Now(),
		xidByType: make(map[uint64]uint64),
	}

	// Record some events
	stats.recordEvent(79)
	stats.recordEvent(48)
	stats.recordEvent(79) // Duplicate
	stats.recordUnhealthy()
	stats.recordUnhealthy()
	stats.recordError()

	require.Equal(t, uint64(3), stats.eventsProcessed)
	require.Equal(t, uint64(2), stats.devicesMarkedUnhealthy)
	require.Equal(t, uint64(1), stats.errorCount)
	require.Equal(t, uint64(2), stats.xidByType[79])
	require.Equal(t, uint64(1), stats.xidByType[48])

	t.Log("✓ Stats correctly collected")
}

// Test 9: Multiple XIDs in Sequence
func TestCheckHealth_MultipleXIDsInSequence(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	xidsToSend := []uint64{79, 48, 64}
	xidIndex := 0

	mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		eventSet := &mock.EventSet{
			WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
				if xidIndex < len(xidsToSend) {
					xid := xidsToSend[xidIndex]
					deviceIdx := xidIndex % 8 // Spread across devices
					xidIndex++
					return nvml.EventData{
						EventType: nvml.EventTypeXidCriticalError,
						EventData: xid,
						Device:    mockNVML.Devices[deviceIdx],
					}, nvml.SUCCESS
				}
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return eventSet, nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 8)

	healthChan := make(chan *Device, 64)
	stopChan := make(chan interface{})

	go func() {
		_ = rm.checkHealth(stopChan, rm.devices, healthChan)
	}()

	time.Sleep(300 * time.Millisecond)
	close(stopChan)

	// Should have received 3 unhealthy notifications
	unhealthyCount := 0
	timeout := time.After(500 * time.Millisecond)
drainLoop:
	for {
		select {
		case <-healthChan:
			unhealthyCount++
		case <-timeout:
			break drainLoop
		}
	}

	require.Equal(t, len(xidsToSend), unhealthyCount,
		"Should have received %d unhealthy notifications, got %d",
		len(xidsToSend), unhealthyCount)
	t.Logf("✓ Correctly processed %d XIDs in sequence", unhealthyCount)
}

// Test 10: Device Registration Errors
func TestCheckHealth_Phase1_DeviceRegistrationErrors(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	// Get device 3's UUID before modifying
	device3 := mockNVML.Devices[3].(*dgxa100.Device)
	device3UUID := device3.UUID

	// Make device 3 fail registration
	device3.RegisterEventsFunc = func(u uint64, es nvml.EventSet) nvml.Return {
		return nvml.ERROR_NOT_SUPPORTED
	}

	// Keep EventSet waiting
	mockNVML.EventSetCreateFunc = func() (nvml.EventSet, nvml.Return) {
		eventSet := &mock.EventSet{
			WaitFunc: func(u uint32) (nvml.EventData, nvml.Return) {
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			},
			FreeFunc: func() nvml.Return {
				return nvml.SUCCESS
			},
		}
		return eventSet, nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 8)

	healthChan := make(chan *Device, 64)
	stopChan := make(chan interface{})

	go func() {
		_ = rm.checkHealth(stopChan, rm.devices, healthChan)
	}()

	time.Sleep(200 * time.Millisecond)
	close(stopChan)

	// Device 3 should be marked unhealthy during registration
	unhealthyDevices := []string{}
	timeout := time.After(300 * time.Millisecond)
drainLoop:
	for {
		select {
		case d := <-healthChan:
			unhealthyDevices = append(unhealthyDevices, d.ID)
		case <-timeout:
			break drainLoop
		}
	}

	require.Contains(t, unhealthyDevices, device3UUID,
		"Device 3 (%s) with registration error should be marked unhealthy", device3UUID)
	t.Logf("✓ Device %s with registration error correctly marked unhealthy", device3UUID)
}

// Test 11: handleEventWaitError behavior for different error codes
func TestHandleEventWaitError_Phase1(t *testing.T) {
	testCases := []struct {
		name               string
		errorCode          nvml.Return
		expectContinue     bool
		expectAllUnhealthy bool
	}{
		{
			name:               "GPU_IS_LOST marks all unhealthy and continues",
			errorCode:          nvml.ERROR_GPU_IS_LOST,
			expectContinue:     true,
			expectAllUnhealthy: true,
		},
		{
			name:               "UNINITIALIZED terminates",
			errorCode:          nvml.ERROR_UNINITIALIZED,
			expectContinue:     false,
			expectAllUnhealthy: false,
		},
		{
			name:               "UNKNOWN continues without marking unhealthy",
			errorCode:          nvml.ERROR_UNKNOWN,
			expectContinue:     true,
			expectAllUnhealthy: false,
		},
		{
			name:               "NOT_SUPPORTED continues without marking unhealthy",
			errorCode:          nvml.ERROR_NOT_SUPPORTED,
			expectContinue:     true,
			expectAllUnhealthy: false,
		},
		{
			name:               "Other error marks all unhealthy and continues",
			errorCode:          nvml.ERROR_INSUFFICIENT_POWER,
			expectContinue:     true,
			expectAllUnhealthy: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockNVML := dgxa100.New()
			mockDGXA100Setup(mockNVML)

			rm := newMockResourceManager(t, mockNVML, 3)

			healthChan := make(chan *Device, 64)

			shouldContinue := rm.handleEventWaitError(tc.errorCode, rm.devices, healthChan)

			require.Equal(t, tc.expectContinue, shouldContinue,
				"Error %v should return continue=%v", tc.errorCode, tc.expectContinue)

			// Check if devices were marked unhealthy
			unhealthyCount := 0
			timeout := time.After(100 * time.Millisecond)
		drainLoop:
			for {
				select {
				case <-healthChan:
					unhealthyCount++
				case <-timeout:
					break drainLoop
				}
			}

			if tc.expectAllUnhealthy {
				require.Equal(t, len(rm.devices), unhealthyCount,
					"All devices should be marked unhealthy for error %v", tc.errorCode)
			} else {
				require.Equal(t, 0, unhealthyCount,
					"No devices should be marked unhealthy for error %v", tc.errorCode)
			}

			t.Logf("✓ Error %v: continue=%v, unhealthy=%d",
				tc.errorCode, shouldContinue, unhealthyCount)
		})
	}
}

func TestDevice_Phase2_MarkUnhealthy(t *testing.T) {
	device := &Device{
		Device: pluginapi.Device{
			ID:     "GPU-test",
			Health: pluginapi.Healthy,
		},
	}

	device.MarkUnhealthy("XID-79")

	require.Equal(t, pluginapi.Unhealthy, device.Health)
	require.Equal(t, "XID-79", device.UnhealthyReason)
	require.Equal(t, 0, device.RecoveryAttempts)
	require.False(t, device.LastUnhealthyTime.IsZero(), "LastUnhealthyTime should be set")
	t.Log("✓ MarkUnhealthy correctly updates device state")
}

func TestDevice_Phase2_MarkHealthy(t *testing.T) {
	device := &Device{
		Device: pluginapi.Device{
			ID:     "GPU-test",
			Health: pluginapi.Unhealthy,
		},
		UnhealthyReason:  "XID-79",
		RecoveryAttempts: 5,
	}

	device.MarkHealthy()

	require.Equal(t, pluginapi.Healthy, device.Health)
	require.Equal(t, "", device.UnhealthyReason)
	require.Equal(t, 0, device.RecoveryAttempts)
	require.False(t, device.LastHealthyTime.IsZero(), "LastHealthyTime should be set")
	t.Log("✓ MarkHealthy correctly clears unhealthy state")
}

func TestDevice_Phase2_IsUnhealthy(t *testing.T) {
	healthyDevice := &Device{
		Device: pluginapi.Device{Health: pluginapi.Healthy},
	}
	unhealthyDevice := &Device{
		Device: pluginapi.Device{Health: pluginapi.Unhealthy},
	}

	require.False(t, healthyDevice.IsUnhealthy())
	require.True(t, unhealthyDevice.IsUnhealthy())
	t.Log("✓ IsUnhealthy correctly reports device state")
}

func TestDevice_Phase2_UnhealthyDuration(t *testing.T) {
	device := &Device{
		Device: pluginapi.Device{
			ID:     "GPU-test",
			Health: pluginapi.Unhealthy,
		},
		LastUnhealthyTime: time.Now().Add(-5 * time.Minute),
	}

	duration := device.UnhealthyDuration()
	require.Greater(t, duration, 4*time.Minute,
		"Device should report ~5 minutes unhealthy")
	require.Less(t, duration, 6*time.Minute,
		"Duration should be approximately 5 minutes")

	// Healthy device should return zero duration
	device.MarkHealthy()
	require.Equal(t, time.Duration(0), device.UnhealthyDuration())
	t.Log("✓ UnhealthyDuration correctly calculates time")
}

func TestCheckDeviceHealth_Phase2_DeviceRecovers(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	// Device responds successfully
	mockNVML.Devices[0].(*dgxa100.Device).GetNameFunc = func() (string, nvml.Return) {
		return "Tesla V100", nvml.SUCCESS
	}

	rm := newMockResourceManager(t, mockNVML, 1)
	deviceUUID := mockNVML.Devices[0].(*dgxa100.Device).UUID
	device := rm.devices[deviceUUID]
	device.MarkUnhealthy("XID-79")

	healthy, err := rm.CheckDeviceHealth(device)

	require.NoError(t, err)
	require.True(t, healthy, "Device should be detected as healthy")
	t.Log("✓ CheckDeviceHealth detects recovered device")
}

func TestCheckDeviceHealth_Phase2_DeviceStillFailing(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	// Device not responding
	mockNVML.Devices[0].(*dgxa100.Device).GetNameFunc = func() (string, nvml.Return) {
		return "", nvml.ERROR_GPU_IS_LOST
	}

	rm := newMockResourceManager(t, mockNVML, 1)
	deviceUUID := mockNVML.Devices[0].(*dgxa100.Device).UUID
	device := rm.devices[deviceUUID]
	device.MarkUnhealthy("XID-79")

	healthy, err := rm.CheckDeviceHealth(device)

	require.Error(t, err)
	require.False(t, healthy, "Device should still be unhealthy")
	require.Contains(t, err.Error(), "not responsive")
	t.Log("✓ CheckDeviceHealth detects device still failing")
}

func TestCheckDeviceHealth_Phase2_NVMLInitFailure(t *testing.T) {
	mockNVML := dgxa100.New()
	mockDGXA100Setup(mockNVML)

	// Make NVML Init fail
	mockNVML.InitFunc = func() nvml.Return {
		return nvml.ERROR_UNINITIALIZED
	}

	rm := newMockResourceManager(t, mockNVML, 1)
	deviceUUID := mockNVML.Devices[0].(*dgxa100.Device).UUID
	device := rm.devices[deviceUUID]
	device.MarkUnhealthy("XID-79")

	healthy, err := rm.CheckDeviceHealth(device)

	require.Error(t, err)
	require.False(t, healthy)
	require.Contains(t, err.Error(), "NVML init failed")
	t.Log("✓ CheckDeviceHealth handles NVML init failures")
}
