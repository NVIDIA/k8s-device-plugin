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

	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	allHealthChecks        = "xids"
)

type ResourceManager interface {
	Devices() []*pluginapi.Device
	CheckHealth(ctx context.Context, unhealthy chan<- *pluginapi.Device)
}

type GpuDeviceManager struct {
	cachedDevices []*pluginapi.Device
}

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

func NewGpuDeviceManager() *GpuDeviceManager {
	return &GpuDeviceManager{}
}

func (g *GpuDeviceManager) Devices() []*pluginapi.Device {
	if g.cachedDevices == nil {
		g.cachedDevices = g.getDevices()
	}
	return g.cachedDevices
}

func (g *GpuDeviceManager) getDevices() []*pluginapi.Device {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*pluginapi.Device
	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDeviceLite(i)
		check(err)

		dev := pluginapi.Device{
			ID:     d.UUID,
			Health: pluginapi.Healthy,
		}
		if d.CPUAffinity != nil {
			dev.Topology = &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{
					&pluginapi.NUMANode{
						ID: int64(*(d.CPUAffinity)),
					},
				},
			}
		}
		devs = append(devs, &dev)
	}

	return devs
}

func (g *GpuDeviceManager) CheckHealth(ctx context.Context, unhealthy chan<- *pluginapi.Device) {
	disableHealthChecks := strings.ToLower(os.Getenv(envDisableHealthChecks))
	if disableHealthChecks == "all" {
		disableHealthChecks = allHealthChecks
	}
	if !strings.Contains(disableHealthChecks, "xids") {
		return
	}

	eventSet := nvml.NewEventSet()
	defer nvml.DeleteEventSet(eventSet)

	for _, d := range g.Devices() {
		err := nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, d.ID)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			log.Printf("Warning: %s is too old to support healthchecking: %s. Marking it unhealthy.", d.ID, err)

			unhealthy <- d
			continue
		}

		if err != nil {
			log.Panicln("Fatal:", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
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
			for _, d := range g.Devices() {
				log.Printf("XidCriticalError: Xid=%d, All devices will go unhealthy.", e.Edata)
				unhealthy <- d
			}
			continue
		}

		for _, d := range g.Devices() {
			if d.ID == *e.UUID {
				log.Printf("XidCriticalError: Xid=%d on GPU=%s, the device will go unhealthy.", e.Edata, d.ID)
				unhealthy <- d
			}
		}
	}
}
