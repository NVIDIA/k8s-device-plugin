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
func NewMigStrategy(strategy string) (MigStrategy, error) {
	switch strategy {
	case MigStrategyNone:
		return &migStrategyNone{}, nil
	case MigStrategySingle:
		return &migStrategySingle{}, nil
	case MigStrategyMixed:
		return &migStrategyMixed{}, nil
	}
	return nil, fmt.Errorf("Unknown strategy: %v", strategy)
}

type migStrategyNone struct{}
type migStrategySingle struct{}
type migStrategyMixed struct{}

// getAllMigDevices() across all full GPUs
func getAllMigDevices() []*nvml.Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var migs []*nvml.Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		check(err)

		migEnabled, err := d.IsMigEnabled()
		check(err)

		if !migEnabled {
			continue
		}

		devs, err := d.GetMigDevices()
		check(err)

		migs = append(migs, devs...)
	}

	return migs
}

// migStrategyNone
func (s *migStrategyNone) GetPlugins() []*NvidiaDevicePlugin {
	return []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/gpu",
			NewGpuDeviceManager(false), // Enumerate device even if MIG enabled
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.NewBestEffortPolicy(),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock"),
	}
}

func (s *migStrategyNone) MatchesResource(mig *nvml.Device, resource string) bool {
	panic("Should never be called")
}

// migStrategySingle
func (s *migStrategySingle) GetPlugins() []*NvidiaDevicePlugin {
	resources := make(MigStrategyResourceSet)
	for _, mig := range getAllMigDevices() {
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

	return []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/gpu",
			NewMigDeviceManager(s, "gpu"),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.Policy(nil),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock"),
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
	resources := make(MigStrategyResourceSet)
	for _, mig := range getAllMigDevices() {
		r := s.getResourceName(mig)
		if !s.validMigDevice(mig) {
			log.Printf("Skipping unsupported MIG device: %v", r)
			continue
		}
		resources[r] = struct{}{}
	}

	plugins := []*NvidiaDevicePlugin{
		NewNvidiaDevicePlugin(
			"nvidia.com/gpu",
			NewGpuDeviceManager(true),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.NewBestEffortPolicy(),
			pluginapi.DevicePluginPath+"nvidia-gpu.sock"),
	}

	for resource := range resources {
		plugin := NewNvidiaDevicePlugin(
			"nvidia.com/"+resource,
			NewMigDeviceManager(s, resource),
			"NVIDIA_VISIBLE_DEVICES",
			gpuallocator.Policy(nil),
			pluginapi.DevicePluginPath+"nvidia-"+resource+".sock")
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
