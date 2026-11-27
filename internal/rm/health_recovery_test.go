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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type healthRecoveryTestGPU struct {
	recoveryActionTestDevice
	uuid       string
	uuidRet    nvml.Return
	queries    int
	supported  uint64
	registered []uint64
	onRegister func()
}

func newHealthRecoveryTestGPU(t *testing.T, uuid string, actions ...nvml.DeviceGpuRecoveryAction) *healthRecoveryTestGPU {
	t.Helper()
	gpu := &healthRecoveryTestGPU{
		uuid: uuid,
		supported: nvml.EventTypeXidCriticalError | nvml.EventTypeGpuRecoveryAction |
			nvml.EventTypeSingleBitEccError | nvml.EventTypeDoubleBitEccError,
	}
	gpu.getFieldValues = func(values []nvml.FieldValue) nvml.Return {
		require.LessOrEqual(t, gpu.queries, len(actions), "unexpected recovery-action query for %s", uuid)
		return recoveryActionField(t, actions[gpu.queries-1])(values)
	}
	return gpu
}

func (g *healthRecoveryTestGPU) GetUUID() (string, nvml.Return) {
	return g.uuid, g.uuidRet
}

func (g *healthRecoveryTestGPU) GetFieldValues(values []nvml.FieldValue) nvml.Return {
	g.queries++
	return g.recoveryActionTestDevice.GetFieldValues(values)
}

func (g *healthRecoveryTestGPU) GetSupportedEventTypes() (uint64, nvml.Return) {
	return g.supported, nvml.SUCCESS
}

func (g *healthRecoveryTestGPU) RegisterEvents(mask uint64, _ nvml.EventSet) nvml.Return {
	g.registered = append(g.registered, mask)
	if g.onRegister != nil {
		g.onRegister()
	}
	return nvml.SUCCESS
}

type healthRecoveryTestEventSet struct {
	nvml.EventSet
	wait  func(uint32) (nvml.EventData, nvml.Return)
	freed bool
}

func (s *healthRecoveryTestEventSet) Wait(timeout uint32) (nvml.EventData, nvml.Return) {
	return s.wait(timeout)
}

func (s *healthRecoveryTestEventSet) Free() nvml.Return {
	s.freed = true
	return nvml.SUCCESS
}

type healthRecoveryTestLibrary struct {
	nvml.Interface
	gpus        map[string]*healthRecoveryTestGPU
	getHandle   func(string) (nvml.Device, nvml.Return)
	eventSet    *healthRecoveryTestEventSet
	initialized bool
	shutdown    bool
	created     bool
}

func (l *healthRecoveryTestLibrary) Init() nvml.Return {
	l.initialized = true
	return nvml.SUCCESS
}

func (l *healthRecoveryTestLibrary) Shutdown() nvml.Return {
	l.shutdown = true
	return nvml.SUCCESS
}

func (l *healthRecoveryTestLibrary) EventSetCreate() (nvml.EventSet, nvml.Return) {
	l.created = true
	return l.eventSet, nvml.SUCCESS
}

func (l *healthRecoveryTestLibrary) DeviceGetHandleByUUID(uuid string) (nvml.Device, nvml.Return) {
	if l.getHandle != nil {
		return l.getHandle(uuid)
	}
	if gpu, exists := l.gpus[uuid]; exists {
		return gpu, nvml.SUCCESS
	}
	return nil, nvml.ERROR_NOT_FOUND
}

func runHealthRecoveryCheck(t *testing.T, devices Devices, lib *healthRecoveryTestLibrary, events ...nvml.EventData) map[string][]string {
	t.Helper()
	stop := make(chan any)
	lib.eventSet = &healthRecoveryTestEventSet{
		wait: func(timeout uint32) (nvml.EventData, nvml.Return) {
			require.EqualValues(t, 5000, timeout)
			if len(events) == 0 {
				close(stop)
				return nvml.EventData{}, nvml.ERROR_TIMEOUT
			}
			event := events[0]
			events = events[1:]
			return event, nvml.SUCCESS
		},
	}
	health := make(chan HealthEvent, (len(events)+3)*len(devices)+1)
	manager := &nvmlResourceManager{nvml: lib}
	require.NoError(t, manager.checkHealth(stop, devices, health))
	close(health)
	if lib.created {
		require.True(t, lib.eventSet.freed)
	}
	require.Equal(t, lib.initialized, lib.shutdown)
	result := make(map[string][]string)
	for event := range health {
		result[event.Device.ID] = append(result[event.Device.ID], event.Health)
	}
	return result
}

func TestCheckHealthQueriesAllParentsBeforeRegisteringXIDs(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")
	gpuA := newHealthRecoveryTestGPU(t, "GPU-A", nvml.GPU_RECOVERY_ACTION_GPU_RESET)
	gpuB := newHealthRecoveryTestGPU(t, "GPU-B", nvml.GPU_RECOVERY_ACTION_NONE)
	// Recovery-action event support must not be required to read the field.
	gpuA.supported = nvml.EventTypeXidCriticalError
	for _, gpu := range []*healthRecoveryTestGPU{gpuA, gpuB} {
		gpu.onRegister = func() {
			require.Equal(t, 1, gpuA.queries)
			require.Equal(t, 1, gpuB.queries)
		}
	}
	devices := Devices{
		"GPU-A::0": {Device: pluginapi.Device{ID: "GPU-A::0"}},
		"GPU-A::1": {Device: pluginapi.Device{ID: "GPU-A::1"}},
		"GPU-B":    {Device: pluginapi.Device{ID: "GPU-B"}},
	}
	lib := &healthRecoveryTestLibrary{gpus: map[string]*healthRecoveryTestGPU{"GPU-A": gpuA, "GPU-B": gpuB}}
	health := runHealthRecoveryCheck(t, devices, lib)
	require.Equal(t, map[string][]string{
		"GPU-A::0": {pluginapi.Unhealthy},
		"GPU-A::1": {pluginapi.Unhealthy},
		"GPU-B":    {pluginapi.Healthy},
	}, health)
	for _, gpu := range []*healthRecoveryTestGPU{gpuA, gpuB} {
		require.Equal(t, []uint64{nvml.EventTypeXidCriticalError}, gpu.registered)
	}
}

func TestCheckHealthXIDFilteringAndRecovery(t *testing.T) {
	tests := map[string]struct {
		disabled  string
		enabled   string
		eventType uint64
		xid       uint64
		action    nvml.DeviceGpuRecoveryAction
		want      []string
	}{
		"built-in ignored XID": {
			eventType: nvml.EventTypeXidCriticalError, xid: 13,
			want: []string{pluginapi.Healthy},
		},
		"explicitly ignored XID": {
			disabled: "79", eventType: nvml.EventTypeXidCriticalError, xid: 79,
			want: []string{pluginapi.Healthy},
		},
		"explicitly enabled application XID": {
			enabled: "13", eventType: nvml.EventTypeXidCriticalError, xid: 13,
			action: nvml.GPU_RECOVERY_ACTION_GPU_RESET,
			want:   []string{pluginapi.Healthy, pluginapi.Unhealthy},
		},
		"explicitly enabled XID with all others disabled": {
			disabled: "all", enabled: "79", eventType: nvml.EventTypeXidCriticalError, xid: 79,
			action: nvml.GPU_RECOVERY_ACTION_NODE_REBOOT,
			want:   []string{pluginapi.Healthy, pluginapi.Unhealthy},
		},
		"recovery-action notifications are not processed": {
			eventType: nvml.EventTypeGpuRecoveryAction,
			want:      []string{pluginapi.Healthy},
		},
		"ECC notifications are not processed": {
			eventType: nvml.EventTypeDoubleBitEccError,
			want:      []string{pluginapi.Healthy},
		},
		"unignored XID with no recovery required remains healthy": {
			eventType: nvml.EventTypeXidCriticalError, xid: 48,
			action: nvml.GPU_RECOVERY_ACTION_NONE,
			want:   []string{pluginapi.Healthy, pluginapi.Healthy},
		},
		"IMEX-domain recovery remains healthy for GPU scheduling": {
			eventType: nvml.EventTypeXidCriticalError, xid: 48,
			action: nvml.GPU_RECOVERY_ACTION_RECOVER_IMEX_DOMAIN,
			want:   []string{pluginapi.Healthy, pluginapi.Healthy},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(envDisableHealthChecks, tc.disabled)
			t.Setenv(envEnableHealthChecks, tc.enabled)
			gpu := newHealthRecoveryTestGPU(t, "GPU-A", nvml.GPU_RECOVERY_ACTION_NONE, tc.action)
			lib := &healthRecoveryTestLibrary{gpus: map[string]*healthRecoveryTestGPU{"GPU-A": gpu}}
			health := runHealthRecoveryCheck(t,
				Devices{"GPU-A": {Device: pluginapi.Device{ID: "GPU-A"}}}, lib,
				nvml.EventData{Device: gpu, EventType: tc.eventType, EventData: tc.xid},
			)
			require.Equal(t, tc.want, health["GPU-A"])
			require.Equal(t, len(tc.want), gpu.queries)
		})
	}
}

func TestCheckHealthRecoveryQueryFailureDoesNotFallBackToXID(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")
	gpu := newHealthRecoveryTestGPU(t, "GPU-A", nvml.GPU_RECOVERY_ACTION_NONE, nvml.GPU_RECOVERY_ACTION_NONE)
	read := gpu.getFieldValues
	gpu.getFieldValues = func(values []nvml.FieldValue) nvml.Return {
		if gpu.queries == 1 {
			return nvml.ERROR_NOT_SUPPORTED
		}
		return read(values)
	}
	lib := &healthRecoveryTestLibrary{gpus: map[string]*healthRecoveryTestGPU{"GPU-A": gpu}}
	health := runHealthRecoveryCheck(t,
		Devices{"GPU-A": {Device: pluginapi.Device{ID: "GPU-A"}}}, lib,
		nvml.EventData{Device: gpu, EventType: nvml.EventTypeXidCriticalError, EventData: 48},
	)
	require.Equal(t, []string{pluginapi.Unhealthy, pluginapi.Healthy}, health["GPU-A"])
	require.Equal(t, []uint64{nvml.EventTypeXidCriticalError}, gpu.registered)
	require.Equal(t, 2, gpu.queries)
}

func TestCheckHealthMIGUsesParentRecoveryActionForAllSiblings(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")
	gpuA := newHealthRecoveryTestGPU(t, "GPU-A",
		nvml.GPU_RECOVERY_ACTION_NONE, nvml.GPU_RECOVERY_ACTION_GPU_RESET, nvml.GPU_RECOVERY_ACTION_NONE)
	gpuB := newHealthRecoveryTestGPU(t, "GPU-B", nvml.GPU_RECOVERY_ACTION_NONE)
	lib := &healthRecoveryTestLibrary{gpus: map[string]*healthRecoveryTestGPU{"GPU-A": gpuA, "GPU-B": gpuB}}
	// Only the physical GPU handles can answer recovery-action queries.
	eventDevice := &fakeParentHandle{uuid: "GPU-A"}
	xid := nvml.EventData{
		Device: eventDevice, EventType: nvml.EventTypeXidCriticalError, EventData: 48,
		GpuInstanceId: 1, ComputeInstanceId: 0,
	}
	health := runHealthRecoveryCheck(t, Devices{
		"MIG-GPU-A/1/0": {Device: pluginapi.Device{ID: "MIG-GPU-A/1/0"}, Index: "0:0"},
		"MIG-GPU-A/2/0": {Device: pluginapi.Device{ID: "MIG-GPU-A/2/0"}, Index: "0:1"},
		"GPU-B":         {Device: pluginapi.Device{ID: "GPU-B"}},
	}, lib, xid, xid)
	for _, id := range []string{"MIG-GPU-A/1/0", "MIG-GPU-A/2/0"} {
		require.Equal(t, []string{pluginapi.Healthy, pluginapi.Unhealthy, pluginapi.Healthy}, health[id])
	}
	require.Equal(t, []string{pluginapi.Healthy}, health["GPU-B"])
	require.Equal(t, 3, gpuA.queries)
	require.Equal(t, 1, gpuB.queries)
}

func TestCheckHealthXIDQueryFailureAndRecovery(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")
	gpu := newHealthRecoveryTestGPU(t, "GPU-A",
		nvml.GPU_RECOVERY_ACTION_NONE, nvml.GPU_RECOVERY_ACTION_NONE, nvml.GPU_RECOVERY_ACTION_NONE)
	read := gpu.getFieldValues
	gpu.getFieldValues = func(values []nvml.FieldValue) nvml.Return {
		if gpu.queries == 2 {
			return nvml.ERROR_NOT_SUPPORTED
		}
		return read(values)
	}
	lib := &healthRecoveryTestLibrary{gpus: map[string]*healthRecoveryTestGPU{"GPU-A": gpu}}
	xid := nvml.EventData{Device: gpu, EventType: nvml.EventTypeXidCriticalError, EventData: 48}
	health := runHealthRecoveryCheck(t,
		Devices{"GPU-A": {Device: pluginapi.Device{ID: "GPU-A"}}}, lib, xid, xid)
	require.Equal(t, []string{pluginapi.Healthy, pluginapi.Unhealthy, pluginapi.Healthy}, health["GPU-A"])
	require.Equal(t, 3, gpu.queries)
}

func TestCheckHealthXIDParentLookupFailure(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "")
	t.Setenv(envEnableHealthChecks, "")
	gpu := newHealthRecoveryTestGPU(t, "GPU-A", nvml.GPU_RECOVERY_ACTION_NONE)
	lookups := 0
	lib := &healthRecoveryTestLibrary{
		getHandle: func(uuid string) (nvml.Device, nvml.Return) {
			require.Equal(t, "GPU-A", uuid)
			lookups++
			if lookups == 1 {
				return gpu, nvml.SUCCESS
			}
			return nil, nvml.ERROR_GPU_IS_LOST
		},
	}
	health := runHealthRecoveryCheck(t,
		Devices{"GPU-A": {Device: pluginapi.Device{ID: "GPU-A"}}}, lib,
		nvml.EventData{Device: &fakeParentHandle{uuid: "GPU-A"}, EventType: nvml.EventTypeXidCriticalError, EventData: 48})
	require.Equal(t, []string{pluginapi.Healthy, pluginapi.Unhealthy}, health["GPU-A"])
	require.Equal(t, 2, lookups)
	require.Equal(t, 1, gpu.queries)
}

func TestCheckHealthAllDisabledSkipsMonitoring(t *testing.T) {
	t.Setenv(envDisableHealthChecks, "all")
	t.Setenv(envEnableHealthChecks, "")
	lib := &healthRecoveryTestLibrary{}
	require.Empty(t, runHealthRecoveryCheck(t, Devices{"GPU-A": {Device: pluginapi.Device{ID: "GPU-A"}}}, lib))
	require.False(t, lib.initialized)
	require.False(t, lib.created)
}
