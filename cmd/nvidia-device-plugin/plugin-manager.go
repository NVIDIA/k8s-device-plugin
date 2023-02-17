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
	"github.com/NVIDIA/k8s-device-plugin/internal/cdi"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"

	"k8s.io/klog/v2"
)

// PluginManager provides an interface for building the set of plugins required to implement a given MIG strategy
type PluginManager interface {
	GetPlugins() ([]*NvidiaDevicePlugin, error)
}

// NewPluginManager creates an NVML-based plugin manager
func NewPluginManager(config *spec.Config) (PluginManager, error) {
	var err error
	switch *config.Flags.MigStrategy {
	case spec.MigStrategyNone:
	case spec.MigStrategySingle:
	case spec.MigStrategyMixed:
	default:
		return nil, fmt.Errorf("unknown strategy: %v", *config.Flags.MigStrategy)
	}

	nvmllib := nvml.New()

	cdiHandler := cdi.NewNullHandler()

	if *config.Flags.Plugin.DeviceListStrategy == spec.DeviceListStrategyCDIAnnotations {
		klog.Info("Creating a CDI handler")
		cdiHandler, err = cdi.New(
			cdi.WithDriverRoot(*config.Flags.NvidiaDriverRoot),
			cdi.WithNvidiaCTKPath(*config.Flags.Plugin.NvidiaCTKPath),
			cdi.WithNvml(nvmllib),
			cdi.WithDeviceIDStrategy(*config.Flags.Plugin.DeviceIDStrategy),
			cdi.WithVendor("k8s.device-plugin.nvidia.com"),
			cdi.WithClass("gpu"),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to create cdi handler: %v", err)
		}

		klog.Info("Creating CDI specification")
		if err := cdiHandler.CreateSpecFile(); err != nil {
			return nil, fmt.Errorf("unable to create cdi spec file: %v", err)
		}
	}

	m := pluginManager{
		nvml:   nvmllib,
		config: config,
		cdi:    cdiHandler,
	}
	return &m, nil
}

type pluginManager struct {
	nvml   nvml.Interface
	cdi    cdi.Interface
	config *spec.Config
}

// GetPlugins returns the plugins associated with the NVML resources available on the node
func (s *pluginManager) GetPlugins() ([]*NvidiaDevicePlugin, error) {
	rms, err := rm.NewResourceManagers(s.nvml, s.config)
	if err != nil {
		return nil, fmt.Errorf("unable to load resource managers to manage plugin devices: %v", err)
	}

	var plugins []*NvidiaDevicePlugin
	for _, r := range rms {
		plugins = append(plugins, NewNvidiaDevicePlugin(s.config, r, s.cdi))
	}
	return plugins, nil
}
