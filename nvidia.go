/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
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
	"log"
	"os"
	"strings"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	allHealthChecks        = "xids"
)

// Device couples an underlying pluginapi.Device type with its device node path
type Device struct {
	pluginapi.Device
	Path string
}

// ResourceManager provides an interface for listing a set of Devices and checking health on them
type ResourceManager interface {
	Devices() []*Device
	CheckHealth(stop <-chan interface{}, devices []*Device, unhealthy chan<- *Device)
}

// GpuDeviceManager implements the ResourceManager interface for full GPU devices
type GpuDeviceManager struct {
	skipMigEnabledGPUs bool
}

// MigDeviceManager implements the ResourceManager interface for MIG devices
type MigDeviceManager struct {
	strategy MigStrategy
	resource string
}

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

// NewGpuDeviceManager returns a reference to a new GpuDeviceManager
func NewGpuDeviceManager(skipMigEnabledGPUs bool) *GpuDeviceManager {
	return &GpuDeviceManager{
		skipMigEnabledGPUs: skipMigEnabledGPUs,
	}
}

// NewMigDeviceManager returns a reference to a new MigDeviceManager
func NewMigDeviceManager(strategy MigStrategy, resource string) *MigDeviceManager {
	return &MigDeviceManager{
		strategy: strategy,
		resource: resource,
	}
}

// Devices returns a list of devices from the GpuDeviceManager
func (g *GpuDeviceManager) Devices() []*Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		check(err)

		migEnabled, err := d.IsMigEnabled()
		check(err)

		if migEnabled && g.skipMigEnabledGPUs {
			continue
		}

		devs = append(devs, buildDevice(d))
	}

	return devs
}

// Devices returns a list of devices from the MigDeviceManager
func (m *MigDeviceManager) Devices() []*Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		check(err)

		migEnabled, err := d.IsMigEnabled()
		check(err)

		if !migEnabled {
			continue
		}

		migs, err := d.GetMigDevices()
		check(err)

		for _, mig := range migs {
			if !m.strategy.MatchesResource(mig, m.resource) {
				continue
			}
			devs = append(devs, buildDevice(mig))
		}
	}

	return devs
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (g *GpuDeviceManager) CheckHealth(stop <-chan interface{}, devices []*Device, unhealthy chan<- *Device) {
	checkHealth(stop, devices, unhealthy)
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (m *MigDeviceManager) CheckHealth(stop <-chan interface{}, devices []*Device, unhealthy chan<- *Device) {
	checkHealth(stop, devices, unhealthy)
}

func buildDevice(d *nvml.Device) *Device {
	dev := Device{}
	dev.ID = d.UUID
	dev.Health = pluginapi.Healthy
	dev.Path = d.Path
	if d.CPUAffinity != nil {
		dev.Topology = &pluginapi.TopologyInfo{
			Nodes: []*pluginapi.NUMANode{
				&pluginapi.NUMANode{
					ID: int64(*(d.CPUAffinity)),
				},
			},
		}
	}
	return &dev
}

func checkHealth(stop <-chan interface{}, devices []*Device, unhealthy chan<- *Device) {
	disableHealthChecks := strings.ToLower(os.Getenv(envDisableHealthChecks))
	if disableHealthChecks == "all" {
		disableHealthChecks = allHealthChecks
	}
	if strings.Contains(disableHealthChecks, "xids") {
		return
	}

	eventSet := nvml.NewEventSet()
	defer nvml.DeleteEventSet(eventSet)

	for _, d := range devices {
		gpu, _, _, err := nvml.ParseMigDeviceUUID(d.ID)
		if err != nil {
			gpu = d.ID
		}

		err = nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, gpu)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			log.Printf("Warning: %s is too old to support healthchecking: %s. Marking it unhealthy.", d.ID, err)
			unhealthy <- d
			continue
		}
		check(err)
	}

	for {
		select {
		case <-stop:
			return
		default:
		}

		e, err := nvml.WaitForEvent(eventSet, 5000)
		if err != nil && e.Etype != nvml.XidCriticalError {
			continue
		}

		// FIXME: formalize the full list and document it.
		// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
		// Application errors: the GPU should still be healthy
		if e.Edata == 31 || e.Edata == 43 || e.Edata == 45 {
			continue
		}

		if e.UUID == nil || len(*e.UUID) == 0 {
			// All devices are unhealthy
			log.Printf("XidCriticalError: Xid=%d, All devices will go unhealthy.", e.Edata)
			for _, d := range devices {
				unhealthy <- d
			}
			continue
		}

		for _, d := range devices {
			// Please see https://github.com/NVIDIA/gpu-monitoring-tools/blob/148415f505c96052cb3b7fdf443b34ac853139ec/bindings/go/nvml/nvml.h#L1424
			// for the rationale why gi and ci can be set as such when the UUID is a full GPU UUID and not a MIG device UUID.
			gpu, gi, ci, err := nvml.ParseMigDeviceUUID(d.ID)
			if err != nil {
				gpu = d.ID
				gi = 0xFFFFFFFF
				ci = 0xFFFFFFFF
			}

			if gpu == *e.UUID && gi == *e.GpuInstanceId && ci == *e.ComputeInstanceId {
				log.Printf("XidCriticalError: Xid=%d on Device=%s, the device will go unhealthy.", e.Edata, d.ID)
				unhealthy <- d
			}
		}
	}
}
