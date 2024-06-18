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

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/info"
	"github.com/NVIDIA/go-nvml/pkg/nvml"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/cdi"
	"github.com/NVIDIA/k8s-device-plugin/internal/plugin/manager"
)

// NewPluginManager creates an NVML-based plugin manager
func NewPluginManager(infolib info.Interface, nvmllib nvml.Interface, devicelib device.Interface, config *spec.Config) (manager.Interface, error) {
	var err error
	switch *config.Flags.MigStrategy {
	case spec.MigStrategyNone:
	case spec.MigStrategySingle:
	case spec.MigStrategyMixed:
	default:
		return nil, fmt.Errorf("unknown strategy: %v", *config.Flags.MigStrategy)
	}

	// TODO: We could consider passing this as an argument since it should already be used to construct nvmllib.
	driverRoot := root(*config.Flags.Plugin.ContainerDriverRoot)

	deviceListStrategies, err := spec.NewDeviceListStrategies(*config.Flags.Plugin.DeviceListStrategy)
	if err != nil {
		return nil, fmt.Errorf("invalid device list strategy: %v", err)
	}

	cdiHandler, err := cdi.New(infolib, nvmllib, devicelib,
		cdi.WithDeviceListStrategies(deviceListStrategies),
		cdi.WithDriverRoot(string(driverRoot)),
		cdi.WithDevRoot(driverRoot.getDevRoot()),
		cdi.WithTargetDriverRoot(*config.Flags.NvidiaDriverRoot),
		cdi.WithTargetDevRoot(*config.Flags.NvidiaDevRoot),
		cdi.WithNvidiaCTKPath(*config.Flags.Plugin.NvidiaCTKPath),
		cdi.WithDeviceIDStrategy(*config.Flags.Plugin.DeviceIDStrategy),
		cdi.WithVendor("k8s.device-plugin.nvidia.com"),
		cdi.WithGdsEnabled(*config.Flags.GDSEnabled),
		cdi.WithMofedEnabled(*config.Flags.MOFEDEnabled),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create cdi handler: %v", err)
	}

	m, err := manager.New(infolib, nvmllib, devicelib,
		manager.WithCDIHandler(cdiHandler),
		manager.WithConfig(config),
		manager.WithFailOnInitError(*config.Flags.FailOnInitError),
		manager.WithMigStrategy(*config.Flags.MigStrategy),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create plugin manager: %v", err)
	}

	if err := m.CreateCDISpecFile(); err != nil {
		return nil, fmt.Errorf("unable to create cdi spec file: %v", err)
	}

	return m, nil
}
