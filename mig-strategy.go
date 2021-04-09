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

	"github.com/NVIDIA/go-gpuallocator/gpuallocator"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Constants representing the various MIG strategies
const (
	MigStrategyNone   = "none"
	MigStrategySingle = "single"
	MigStrategyMixed  = "mixed"
)

// MigStrategyResourceSet holds a set of resource names for a given MIG strategy
type MigStrategyResourceSet map[string]struct{}

// MigStrategy provides an interface for building the set of plugins required to implement a given MIG strategy
type MigStrategy interface {
	GetPlugins() []*NvidiaDevicePlugin
	MatchesResource(mig *nvml.Device, resource string) bool
}

// NewMigStrategy returns a reference to a given MigStrategy based on the 'strategy' passed in
func NewMigStrategy(strategy string, resourceConfig resourceConfiguration) (MigStrategy, error) {
	switch strategy {
	case MigStrategyNone:
		return &migStrategyNone{ResourceConfig: resourceConfig}, nil
	case MigStrategySingle:
		return &migStrategySingle{ResourceConfig: resourceConfig}, nil
	case MigStrategyMixed:
		return &migStrategyMixed{ResourceConfig: resourceConfig}, nil
	}
	return nil, fmt.Errorf("Unknown strategy: %v", strategy)
}

type variant struct {
	Name     string
	Replicas uint
}

type resourceConfiguration map[string]variant

func (v *resourceConfiguration) Get(name string) variant {
	x, exists := (*v)[name]
	if exists {
		return x
	}
	return variant{
		Name:     name,
		Replicas: 0,
	}
}

type migStrategyNone struct {
	ResourceConfig resourceConfiguration
}

type migStrategySingle struct {
	ResourceConfig resourceConfiguration
}

type migStrategyMixed struct {
	ResourceConfig resourceConfiguration
}

// migStrategyNone
func (s *migStrategyNone) GetPlugins() []*NvidiaDevicePlugin {
	rc := s.ResourceConfig.Get("gpu")
	return []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/"+rc.Name,
			NewGpuDeviceManager(false), // Enumerate device even if MIG enabled
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.NewBestEffortPolicy(),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock",
			rc.Replicas),
	}
}

func (s *migStrategyNone) MatchesResource(mig *nvml.Device, resource string) bool {
	panic("Should never be called")
}

// migStrategySingle
func (s *migStrategySingle) GetPlugins() []*NvidiaDevicePlugin {
	devices := NewMIGCapableDevices()

	migEnabledDevices, err := devices.GetDevicesWithMigEnabled()
	if err != nil {
		panic(fmt.Errorf("Unabled to retrieve list of MIG-enabled devices: %v", err))
	}

	// If no MIG devices are available fallback to "none" strategy
	if len(migEnabledDevices) == 0 {
		none, _ := NewMigStrategy(MigStrategyNone, s.ResourceConfig)
		log.Printf("No MIG devices found. Falling back to mig.strategy=%v", none)
		return none.GetPlugins()
	}

	migDisabledDevices, err := devices.GetDevicesWithMigDisabled()
	if err != nil {
		panic(fmt.Errorf("Unabled to retrieve list of non-MIG-enabled devices: %v", err))
	}
	if len(migDisabledDevices) != 0 {
		panic(fmt.Errorf("For mig.strategy=single all devices on the node must all be configured with the same migEnabled value"))
	}

	if err := devices.AssertAllMigEnabledDevicesAreValid(); err != nil {
		panic(fmt.Errorf("At least one device with migEnabled=true was not configured correctly: %v", err))
	}

	resources := make(MigStrategyResourceSet)

	migs, err := devices.GetAllMigDevices()
	if err != nil {
		panic(fmt.Errorf("Unable to retrieve list of MIG devices: %v", err))
	}
	for _, mig := range migs {
		r := s.getResourceName(mig)
		if !s.validMigDevice(mig) {
			panic("Unsupported MIG device found: " + r)
		}
		resources[r] = struct{}{}
	}

	if len(resources) == 0 {
		panic("No MIG devices present on node")
	}

	if len(resources) != 1 {
		panic("More than one MIG device type present on node")
	}

	rc := s.ResourceConfig.Get("gpu")
	return []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/"+rc.Name,
			NewMigDeviceManager(s, "gpu"),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.Policy(nil),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock",
			rc.Replicas),
	}
}

func (s *migStrategySingle) validMigDevice(mig *nvml.Device) bool {
	attr, err := mig.GetAttributes()
	check(err)

	return attr.GpuInstanceSliceCount == attr.ComputeInstanceSliceCount
}

func (s *migStrategySingle) getResourceName(mig *nvml.Device) string {
	attr, err := mig.GetAttributes()
	check(err)

	g := attr.GpuInstanceSliceCount
	c := attr.ComputeInstanceSliceCount
	gb := ((attr.MemorySizeMB + 1024 - 1) / 1024)

	var r string
	if g == c {
		r = fmt.Sprintf("mig-%dg.%dgb", g, gb)
	} else {
		r = fmt.Sprintf("mig-%dc.%dg.%dgb", c, g, gb)
	}

	return r
}

func (s *migStrategySingle) MatchesResource(mig *nvml.Device, resource string) bool {
	return true
}

// migStrategyMixed
func (s *migStrategyMixed) GetPlugins() []*NvidiaDevicePlugin {
	devices := NewMIGCapableDevices()

	if err := devices.AssertAllMigEnabledDevicesAreValid(); err != nil {
		panic(fmt.Errorf("At least one device with migEnabled=true was not configured correctly: %v", err))
	}

	resources := make(MigStrategyResourceSet)
	migs, err := devices.GetAllMigDevices()
	if err != nil {
		panic(fmt.Errorf("Unable to retrieve list of MIG devices: %v", err))
	}
	for _, mig := range migs {
		r := s.getResourceName(mig)
		if !s.validMigDevice(mig) {
			log.Printf("Skipping unsupported MIG device: %v", r)
			continue
		}
		resources[r] = struct{}{}
	}

	rc := s.ResourceConfig.Get("gpu")
	plugins := []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/"+rc.Name,
			NewGpuDeviceManager(true),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.NewBestEffortPolicy(),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock",
			rc.Replicas),
	}

	for resource := range resources {
		rc := s.ResourceConfig.Get(resource)
		plugin := NewNvidiaDevicePlugin(
			"nvidia.com/"+resource,
			NewMigDeviceManager(s, resource),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.Policy(nil),
			pluginapi.DevicePluginPath+"nvidia-"+resource+".sock",
			rc.Replicas)
		plugins = append(plugins, plugin)
	}

	return plugins
}

func (s *migStrategyMixed) validMigDevice(mig *nvml.Device) bool {
	attr, err := mig.GetAttributes()
	check(err)

	return attr.GpuInstanceSliceCount == attr.ComputeInstanceSliceCount
}

func (s *migStrategyMixed) getResourceName(mig *nvml.Device) string {
	attr, err := mig.GetAttributes()
	check(err)

	g := attr.GpuInstanceSliceCount
	c := attr.ComputeInstanceSliceCount
	gb := ((attr.MemorySizeMB + 1024 - 1) / 1024)

	var r string
	if g == c {
		r = fmt.Sprintf("mig-%dg.%dgb", g, gb)
	} else {
		r = fmt.Sprintf("mig-%dc.%dg.%dgb", c, g, gb)
	}

	return r
}

func (s *migStrategyMixed) MatchesResource(mig *nvml.Device, resource string) bool {
	return s.getResourceName(mig) == resource
}
