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
	"encoding/binary"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type recoveryActionTestDevice struct {
	nvml.Device
	getFieldValues func([]nvml.FieldValue) nvml.Return
}

func (d *recoveryActionTestDevice) GetFieldValues(values []nvml.FieldValue) nvml.Return {
	return d.getFieldValues(values)
}

func recoveryActionField(t *testing.T, action nvml.DeviceGpuRecoveryAction) func([]nvml.FieldValue) nvml.Return {
	t.Helper()
	return func(values []nvml.FieldValue) nvml.Return {
		require.Len(t, values, 1)
		require.EqualValues(t, nvml.FI_DEV_GET_GPU_RECOVERY_ACTION, values[0].FieldId)
		values[0].NvmlReturn = uint32(nvml.SUCCESS)
		values[0].ValueType = uint32(nvml.VALUE_TYPE_UNSIGNED_INT)
		// Unused union bytes must not become part of the decoded enum.
		values[0].Value = [8]byte{0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff}
		//nolint:gosec // Recovery actions in these tests are non-negative enum values.
		binary.NativeEndian.PutUint32(values[0].Value[:4], uint32(action))
		return nvml.SUCCESS
	}
}

func TestGetGpuRecoveryAction(t *testing.T) {
	tests := map[string]struct {
		queryRet  nvml.Return
		fieldRet  uint32
		valueType uint32
		wantError string
	}{
		"unsigned int": {valueType: uint32(nvml.VALUE_TYPE_UNSIGNED_INT)},
		"query failure": {
			queryRet: nvml.ERROR_NOT_SUPPORTED, wantError: "failed to get field values",
		},
		"field failure": {
			fieldRet: uint32(nvml.ERROR_NOT_SUPPORTED), wantError: "failed to get GPU recovery action",
		},
		"unsigned long is not the documented field type": {
			valueType: uint32(nvml.VALUE_TYPE_UNSIGNED_LONG), wantError: "unexpected value type",
		},
		"unsigned long long is not the documented field type": {
			valueType: uint32(nvml.VALUE_TYPE_UNSIGNED_LONG_LONG), wantError: "unexpected value type",
		},
		"double is not the documented field type": {
			valueType: uint32(nvml.VALUE_TYPE_DOUBLE), wantError: "unexpected value type",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gpu := &recoveryActionTestDevice{
				getFieldValues: func(values []nvml.FieldValue) nvml.Return {
					_ = recoveryActionField(t, nvml.GPU_RECOVERY_ACTION_GPU_RESET)(values)
					values[0].NvmlReturn = tc.fieldRet
					values[0].ValueType = tc.valueType
					return tc.queryRet
				},
			}
			action, err := getGpuRecoveryAction(gpu)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, nvml.GPU_RECOVERY_ACTION_GPU_RESET, action)
		})
	}
}

func TestSendGpuRecoveryActionHealth(t *testing.T) {
	tests := map[string]struct {
		action   nvml.DeviceGpuRecoveryAction
		queryRet nvml.Return
		want     string
	}{
		"none":                  {action: nvml.GPU_RECOVERY_ACTION_NONE, want: pluginapi.Healthy},
		"IMEX domain":           {action: nvml.GPU_RECOVERY_ACTION_RECOVER_IMEX_DOMAIN, want: pluginapi.Healthy},
		"GPU reset":             {action: nvml.GPU_RECOVERY_ACTION_GPU_RESET, want: pluginapi.Unhealthy},
		"node reboot":           {action: nvml.GPU_RECOVERY_ACTION_NODE_REBOOT, want: pluginapi.Unhealthy},
		"drain P2P":             {action: nvml.GPU_RECOVERY_ACTION_DRAIN_P2P, want: pluginapi.Unhealthy},
		"drain and reset":       {action: nvml.GPU_RECOVERY_ACTION_DRAIN_AND_RESET, want: pluginapi.Unhealthy},
		"unknown action":        {action: nvml.DeviceGpuRecoveryAction(99), want: pluginapi.Unhealthy},
		"unsupported query":     {queryRet: nvml.ERROR_NOT_SUPPORTED, want: pluginapi.Unhealthy},
		"GPU lost on query":     {queryRet: nvml.ERROR_GPU_IS_LOST, want: pluginapi.Unhealthy},
		"unexpected NVML error": {queryRet: nvml.ERROR_UNKNOWN, want: pluginapi.Unhealthy},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gpu := &recoveryActionTestDevice{
				getFieldValues: func(values []nvml.FieldValue) nvml.Return {
					_ = recoveryActionField(t, tc.action)(values)
					return tc.queryRet
				},
			}
			devices := []*Device{
				{Device: pluginapi.Device{ID: "GPU-parent::0"}},
				{Device: pluginapi.Device{ID: "GPU-parent::1"}},
			}
			health := make(chan HealthEvent, len(devices))
			sendGpuRecoveryActionHealth(gpu, "GPU-parent", devices, health)
			require.Len(t, health, len(devices))
			for _, device := range devices {
				require.Equal(t, HealthEvent{Device: device, Health: tc.want}, <-health)
			}
		})
	}
}
