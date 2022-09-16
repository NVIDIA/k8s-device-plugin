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

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

// PluginManager provides an interface for building the set of plugins required to implement a given MIG strategy
type PluginManager interface {
	GetPlugins() ([]*NvidiaDevicePlugin, error)
}

// NewNVMLPluginManager creates an NVML-based plugin manager
func NewNVMLPluginManager(config *spec.Config) (PluginManager, error) {
	switch *config.Flags.MigStrategy {
	case spec.MigStrategyNone:
	case spec.MigStrategySingle:
	case spec.MigStrategyMixed:
	default:
		return nil, fmt.Errorf("unknown strategy: %v", *config.Flags.MigStrategy)
	}

	nvmllib := nvml.New()

	m := nvmlPluginManager{
		nvml:   nvmllib,
		config: config,
	}
	return &m, nil
}

type nvmlPluginManager struct {
	nvml   nvml.Interface
	config *spec.Config
}

// GetPlugins returns the plugins associated with the NVML resources available on the node
func (s *nvmlPluginManager) GetPlugins() ([]*NvidiaDevicePlugin, error) {
	rms, err := rm.NewResourceManagers(s.nvml, s.config)
	if err != nil {
		return nil, fmt.Errorf("unable to load resource managers to manage plugin devices: %v", err)
	}

	var plugins []*NvidiaDevicePlugin
	for _, r := range rms {
		plugins = append(plugins, NewNvidiaDevicePlugin(s.config, r))
	}
	return plugins, nil
}
