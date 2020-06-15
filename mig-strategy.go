/*
 * Copyright (c) 2020, NVIDIA CORPORATION.  All rights reserved.
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

package main

import (
	"fmt"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	MigStrategyNone = "none"
)

type MigStrategy interface {
	GetPlugins() []*NvidiaDevicePlugin
	MatchesResource(mig *nvml.Device, resource string) bool
}

func NewMigStrategy(strategy string) (MigStrategy, error) {
	switch strategy {
	case MigStrategyNone:
		return &migStrategyNone{}, nil
	}
	return nil, fmt.Errorf("Unknown strategy: %v", strategy)
}

type migStrategyNone struct{}

// migStrategyNone
func (s *migStrategyNone) GetPlugins() []*NvidiaDevicePlugin {
	return []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/gpu",
			NewGpuDeviceManager(false), // Enumerate device even if MIG enabled
			"NVIDIA_VISIBLE_DEVICES",
			pluginapi.DevicePluginPath+"nvidia-gpu.sock"),
	}
}

func (s *migStrategyNone) MatchesResource(mig *nvml.Device, resource string) bool {
	panic("Should never be called")
	return false
}
