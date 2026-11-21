/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package plugin

import (
	"testing"

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/info"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	v1 "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

func TestListAndWatchHealthInitializationFailure(t *testing.T) {
	t.Setenv("DP_DISABLE_HEALTHCHECKS", "")
	t.Setenv("DP_ENABLE_HEALTHCHECKS", "")

	for _, tc := range []struct {
		name            string
		initResult      nvml.Return
		failOnInitError bool
		expectError     bool
	}{
		{
			name:            "NVML initialization failure",
			initResult:      nvml.ERROR_UNKNOWN,
			failOnInitError: true,
			expectError:     true,
		},
		{
			name:       "NVML initialization failure with fail-on-init-error disabled",
			initResult: nvml.ERROR_UNKNOWN,
		},
		{
			name:            "event set creation failure",
			initResult:      nvml.SUCCESS,
			failOnInitError: true,
			expectError:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &v1.Config{
				Flags: v1.Flags{
					CommandLineFlags: v1.CommandLineFlags{
						MigStrategy:     new(v1.MigStrategyNone),
						FailOnInitError: &tc.failOnInitError,
					},
				},
				Resources: v1.Resources{
					GPUs: []v1.Resource{{Pattern: "*", Name: "nvidia.com/gpu"}},
				},
			}
			nvmllib := &healthInitializationNVML{initResult: nvml.SUCCESS}
			managers, err := rm.NewNVMLResourceManagers(healthInitializationInfo{}, nvmllib, healthInitializationDevices{}, config)
			require.NoError(t, err)
			require.Len(t, managers, 1)

			// Discovery succeeds, but initializing health monitoring subsequently fails.
			nvmllib.initResult = tc.initResult
			plugin := &nvidiaDevicePlugin{
				rm:     managers[0],
				health: make(chan *rm.Device),
				stop:   make(chan any),
			}
			stream := &healthInitializationStream{started: make(chan struct{})}
			watchDone := make(chan error, 1)
			go func() {
				watchDone <- plugin.ListAndWatch(&pluginapi.Empty{}, stream)
			}()
			<-stream.started

			err = plugin.rm.CheckHealth(plugin.stop, plugin.health)
			close(plugin.stop)
			require.NoError(t, <-watchDone)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// CheckHealth returns only after ListAndWatch receives both notifications.
			// ListAndWatch must publish the last update before observing the stop signal.
			require.Len(t, stream.snapshots, 3)
			require.Equal(t, map[string]string{
				"GPU-0": pluginapi.Healthy,
				"GPU-1": pluginapi.Healthy,
			}, stream.snapshots[0])
			unhealthy := map[string]string{
				"GPU-0": pluginapi.Unhealthy,
				"GPU-1": pluginapi.Unhealthy,
			}
			require.Equal(t, unhealthy, stream.snapshots[2])

			// A new stream must retain the updated state even after CheckHealth exits.
			// The closed stop channel lets it return immediately after its first report.
			reconnected := &healthInitializationStream{started: make(chan struct{})}
			require.NoError(t, plugin.ListAndWatch(&pluginapi.Empty{}, reconnected))
			require.Equal(t, []map[string]string{unhealthy}, reconnected.snapshots)
		})
	}
}

type healthInitializationStream struct {
	grpc.ServerStream
	started   chan struct{}
	snapshots []map[string]string
}

func (s *healthInitializationStream) Send(response *pluginapi.ListAndWatchResponse) error {
	// Copy health values because subsequent notifications mutate the same devices.
	health := make(map[string]string)
	for _, d := range response.Devices {
		health[d.ID] = d.Health
	}
	s.snapshots = append(s.snapshots, health)
	if len(s.snapshots) == 1 {
		close(s.started)
	}
	return nil
}

type healthInitializationNVML struct {
	nvml.Interface
	initResult nvml.Return
}

func (n *healthInitializationNVML) Init() nvml.Return {
	return n.initResult
}

func (n *healthInitializationNVML) Shutdown() nvml.Return {
	return nvml.SUCCESS
}

func (n *healthInitializationNVML) EventSetCreate() (nvml.EventSet, nvml.Return) {
	return nil, nvml.ERROR_UNKNOWN
}

type healthInitializationInfo struct {
	info.Interface
}

func (healthInitializationInfo) ResolvePlatform() info.Platform {
	return info.PlatformNVML
}

type healthInitializationDevices struct {
	device.Interface
}

func (healthInitializationDevices) VisitDevices(visit func(int, device.Device) error) error {
	for i, uuid := range []string{"GPU-0", "GPU-1"} {
		if err := visit(i, healthInitializationDevice{uuid: uuid}); err != nil {
			return err
		}
	}
	return nil
}

type healthInitializationDevice struct {
	device.Device
	uuid string
}

func (healthInitializationDevice) GetName() (string, nvml.Return) {
	return "test GPU", nvml.SUCCESS
}

func (healthInitializationDevice) IsMigEnabled() (bool, error) {
	return false, nil
}

func (d healthInitializationDevice) GetUUID() (string, nvml.Return) {
	return d.uuid, nvml.SUCCESS
}

func (healthInitializationDevice) GetMinorNumber() (int, nvml.Return) {
	return 0, nvml.SUCCESS
}

func (healthInitializationDevice) GetPciInfo() (nvml.PciInfo, nvml.Return) {
	return nvml.PciInfo{}, nvml.SUCCESS
}

func (healthInitializationDevice) GetMemoryInfo() (nvml.Memory, nvml.Return) {
	return nvml.Memory{}, nvml.SUCCESS
}

func (healthInitializationDevice) GetCudaComputeCapability() (int, int, nvml.Return) {
	return 9, 0, nvml.SUCCESS
}
