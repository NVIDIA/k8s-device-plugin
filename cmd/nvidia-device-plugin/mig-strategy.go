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
	"log"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/mig"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

// MigStrategyResourceSet holds a set of resource names for a given MIG strategy
type MigStrategyResourceSet map[string]struct{}

// MigStrategy provides an interface for building the set of plugins required to implement a given MIG strategy
type MigStrategy interface {
	GetPlugins() []*NvidiaDevicePlugin
}

// NewMigStrategy returns a reference to a given MigStrategy based on the 'strategy' passed in
func NewMigStrategy(config *spec.Config) (MigStrategy, error) {
	switch config.Flags.MigStrategy {
	case spec.MigStrategyNone:
		return &migStrategyNone{config}, nil
	case spec.MigStrategySingle:
		return &migStrategySingle{config}, nil
	case spec.MigStrategyMixed:
		return &migStrategyMixed{config}, nil
	}
	return nil, fmt.Errorf("Unknown strategy: %v", config.Flags.MigStrategy)
}

type migStrategyNone struct{ config *spec.Config }
type migStrategySingle struct{ config *spec.Config }
type migStrategyMixed struct{ config *spec.Config }

// migStrategyNone
func (s *migStrategyNone) GetPlugins() []*NvidiaDevicePlugin {
	rms, err := rm.NewResourceManagers(s.config)
	if err != nil {
		panic(fmt.Errorf("Unable to load resource managers to manage plugin devices: %v", err))
	}
	return getPlugins(s.config, rms)
}

// migStrategySingle
func (s *migStrategySingle) GetPlugins() []*NvidiaDevicePlugin {
	info := mig.NewDeviceInfo()

	migEnabledDevices, err := info.GetDevicesWithMigEnabled()
	if err != nil {
		panic(fmt.Errorf("Unabled to retrieve list of MIG-enabled devices: %v", err))
	}

	// If no MIG devices are available fallback to "none" strategy
	if len(migEnabledDevices) == 0 {
		none := &migStrategyNone{s.config}
		log.Printf("No MIG devices found. Falling back to mig.strategy=%v", spec.MigStrategyNone)
		return none.GetPlugins()
	}

	migDisabledDevices, err := info.GetDevicesWithMigDisabled()
	if err != nil {
		panic(fmt.Errorf("Unabled to retrieve list of non-MIG-enabled devices: %v", err))
	}
	if len(migDisabledDevices) != 0 {
		panic(fmt.Errorf("For mig.strategy=single all devices on the node must all be configured with the same migEnabled value"))
	}

	if err := info.AssertAllMigEnabledDevicesAreValid(); err != nil {
		panic(fmt.Errorf("At least one device with migEnabled=true was not configured correctly: %v", err))
	}

	rms, err := rm.NewResourceManagers(s.config)
	if err != nil {
		panic(fmt.Errorf("Unable to load resource managers to manage plugin devices: %v", err))
	}

	if len(rms) == 0 {
		panic("No MIG devices present on node")
	}

	if len(rms) != 1 {
		panic("More than one MIG device type present on node")
	}

	return getPlugins(s.config, rms)
}

// migStrategyMixed
func (s *migStrategyMixed) GetPlugins() []*NvidiaDevicePlugin {
	info := mig.NewDeviceInfo()

	if err := info.AssertAllMigEnabledDevicesAreValid(); err != nil {
		panic(fmt.Errorf("At least one device with migEnabled=true was not configured correctly: %v", err))
	}

	rms, err := rm.NewResourceManagers(s.config)
	if err != nil {
		panic(fmt.Errorf("Unable to load resource managers to manage plugin devices: %v", err))
	}

	return getPlugins(s.config, rms)
}

// getPlugins generates the plugins from all ResourceManagers
func getPlugins(config *spec.Config, rms []rm.ResourceManager) []*NvidiaDevicePlugin {
	var plugins []*NvidiaDevicePlugin
	for _, r := range rms {
		plugins = append(plugins, NewNvidiaDevicePlugin(config, r))
	}
	return plugins
}
