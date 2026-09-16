/*
 * Copyright (c) 2026, NVIDIA CORPORATION. All rights reserved.
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
	"testing"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// These tests verify notifications sent to the plugin. ListAndWatch is
// responsible for applying those notifications to the advertised device health.
func TestCheckHealthInitializationFailure(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")

	testCases := []struct {
		name            string
		initResult      nvml.Return
		eventSetResult  nvml.Return
		failOnInitError bool
		deviceIDs       []string
		expectedError   string
	}{
		{
			name:            "NVML initialization failure",
			initResult:      nvml.ERROR_UNKNOWN,
			failOnInitError: true,
			deviceIDs:       []string{"GPU-0", "GPU-1", "GPU-2"},
			expectedError:   "failed to initialize NVML",
		},
		{
			name:       "NVML initialization failure with fail on init disabled",
			initResult: nvml.ERROR_UNKNOWN,
			deviceIDs:  []string{"GPU-0", "GPU-1", "GPU-2"},
		},
		{
			name:            "event set creation failure",
			initResult:      nvml.SUCCESS,
			eventSetResult:  nvml.ERROR_UNKNOWN,
			failOnInitError: true,
			deviceIDs:       []string{"GPU-0", "GPU-1", "GPU-2"},
			expectedError:   "failed to create event set",
		},
		{
			name:           "event set creation failure with fail on init disabled",
			initResult:     nvml.SUCCESS,
			eventSetResult: nvml.ERROR_UNKNOWN,
			deviceIDs:      []string{"GPU-0", "GPU-1", "GPU-2"},
			expectedError:  "failed to create event set",
		},
		{
			name:            "NVML initialization failure without devices",
			initResult:      nvml.ERROR_UNKNOWN,
			failOnInitError: true,
			expectedError:   "failed to initialize NVML",
		},
		{
			name:           "event set creation failure without devices",
			initResult:     nvml.SUCCESS,
			eventSetResult: nvml.ERROR_UNKNOWN,
			expectedError:  "failed to create event set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			eventSet := &healthInitEventSet{}
			lib := &healthInitNvmlLib{
				initResult:     tc.initResult,
				eventSetResult: tc.eventSetResult,
				eventSet:       eventSet,
			}
			r := newHealthInitResourceManager(lib, tc.failOnInitError, tc.deviceIDs...)
			unhealthy := make(chan *Device, len(tc.deviceIDs))

			err := r.CheckHealth(make(chan any), unhealthy)
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedError)
			}

			// Drain after CheckHealth returns so a missing notification fails
			// an assertion instead of blocking the test on the original code.
			var notifiedIDs []string
			for len(unhealthy) > 0 {
				d := <-unhealthy
				require.Same(t, r.devices[d.ID], d)
				notifiedIDs = append(notifiedIDs, d.ID)
			}
			require.ElementsMatch(t, tc.deviceIDs, notifiedIDs)
			require.Equal(t, 1, lib.initCalls)
			if tc.initResult == nvml.SUCCESS {
				require.Equal(t, 1, lib.eventSetCreateCalls)
				require.Equal(t, 1, lib.shutdownCalls)
			} else {
				require.Zero(t, lib.eventSetCreateCalls)
				require.Zero(t, lib.shutdownCalls)
			}
			require.Zero(t, eventSet.freeCalls)
		})
	}
}

func TestCheckHealthInitializationDisabled(t *testing.T) {
	for _, disabled := range []string{"all", "xids"} {
		t.Run(disabled, func(t *testing.T) {
			t.Setenv(envDisableHealthChecks, disabled)
			t.Setenv(envEnableHealthChecks, "")
			lib := &healthInitNvmlLib{initResult: nvml.ERROR_UNKNOWN}
			r := newHealthInitResourceManager(lib, true, "GPU-0", "GPU-1")
			unhealthy := make(chan *Device, len(r.devices))

			require.NoError(t, r.CheckHealth(make(chan any), unhealthy))
			require.Empty(t, unhealthy)
			require.Zero(t, lib.initCalls)
			require.Zero(t, lib.eventSetCreateCalls)
			require.Zero(t, lib.shutdownCalls)
		})
	}
}

func TestCheckHealthInitializationSuccessCleanup(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")

	stop := make(chan any)
	waitCalls := 0
	eventSet := &healthInitEventSet{
		waitFunc: func(timeout uint32) (nvml.EventData, nvml.Return) {
			waitCalls++
			require.EqualValues(t, 5000, timeout)
			close(stop)
			return nvml.EventData{}, nvml.ERROR_TIMEOUT
		},
	}
	lib := &healthInitNvmlLib{
		initResult:     nvml.SUCCESS,
		eventSetResult: nvml.SUCCESS,
		eventSet:       eventSet,
	}
	// An empty inventory isolates successful monitor initialization, an
	// ordinary event timeout, and cleanup from per-device event handling.
	r := newHealthInitResourceManager(lib, true)
	require.NoError(t, r.CheckHealth(stop, make(chan *Device)))
	require.Equal(t, 1, lib.initCalls)
	require.Equal(t, 1, lib.eventSetCreateCalls)
	require.Equal(t, 1, waitCalls)
	require.Equal(t, 1, eventSet.freeCalls)
	require.Equal(t, 1, lib.shutdownCalls)
}

func TestCheckHealthInitializationFailureStopsDuringNotification(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")

	for _, initResult := range []nvml.Return{nvml.ERROR_UNKNOWN, nvml.SUCCESS} {
		t.Run(initResult.String(), func(t *testing.T) {
			lib := &healthInitNvmlLib{
				initResult:     initResult,
				eventSetResult: nvml.ERROR_UNKNOWN,
			}
			r := newHealthInitResourceManager(lib, true, "GPU-0", "GPU-1")
			stop := make(chan any)
			t.Cleanup(func() {
				select {
				case <-stop:
				default:
					close(stop)
				}
			})
			unhealthy := make(chan *Device)
			done := make(chan error, 1)
			go func() {
				done <- r.CheckHealth(stop, unhealthy)
			}()

			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			select {
			case d := <-unhealthy:
				require.Same(t, r.devices[d.ID], d)
			case err := <-done:
				t.Fatalf("health check returned without notifying an unhealthy device: %v", err)
			case <-timer.C:
				t.Fatal("health check did not send its first failure notification")
			}

			// No receiver remains for the second device. Closing stop must
			// release the sender even when ListAndWatch is disconnected.
			close(stop)
			select {
			case err := <-done:
				require.Error(t, err)
			case <-timer.C:
				t.Fatal("health check did not stop while sending failure notifications")
			}
			if initResult == nvml.SUCCESS {
				require.Equal(t, 1, lib.shutdownCalls)
			} else {
				require.Zero(t, lib.shutdownCalls)
			}
		})
	}
}

func newHealthInitResourceManager(lib nvml.Interface, failOnInitError bool, ids ...string) *nvmlResourceManager {
	config := &spec.Config{}
	config.Flags.FailOnInitError = &failOnInitError
	devices := make(Devices)
	for _, id := range ids {
		devices[id] = &Device{Device: pluginapi.Device{ID: id, Health: pluginapi.Healthy}}
	}
	return &nvmlResourceManager{
		resourceManager: resourceManager{
			config:   config,
			resource: "nvidia.com/gpu",
			devices:  devices,
		},
		nvml: lib,
	}
}

type healthInitNvmlLib struct {
	nvml.Interface
	initResult          nvml.Return
	eventSetResult      nvml.Return
	eventSet            nvml.EventSet
	initCalls           int
	eventSetCreateCalls int
	shutdownCalls       int
}

func (l *healthInitNvmlLib) Init() nvml.Return {
	l.initCalls++
	return l.initResult
}

func (l *healthInitNvmlLib) EventSetCreate() (nvml.EventSet, nvml.Return) {
	l.eventSetCreateCalls++
	return l.eventSet, l.eventSetResult
}

func (l *healthInitNvmlLib) Shutdown() nvml.Return {
	l.shutdownCalls++
	return nvml.SUCCESS
}

type healthInitEventSet struct {
	nvml.EventSet
	freeCalls int
	waitFunc  func(uint32) (nvml.EventData, nvml.Return)
}

func (e *healthInitEventSet) Wait(timeout uint32) (nvml.EventData, nvml.Return) {
	return e.waitFunc(timeout)
}

func (e *healthInitEventSet) Free() nvml.Return {
	e.freeCalls++
	return nvml.SUCCESS
}
