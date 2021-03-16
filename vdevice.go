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
	"fmt"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type VDevice struct {
	pluginapi.Device
	dev    *Device
	memory uint64
}

func Device2VDevice(devices []*Device) []*VDevice {
	var vdevices []*VDevice
	for _, d := range devices {
		dev, err := nvml.NewDeviceByUUID(d.ID)
		check(err)
		memory := uint64(float64(*dev.Memory) * deviceMemoryScalingFlag / float64(deviceSplitCountFlag))
		for i := uint(0); i < deviceSplitCountFlag; i++ {
			vd := &VDevice{Device: d.Device, dev: d, memory: memory}
			vd.ID = fmt.Sprintf("%v-%v", d.ID, i)
			vd.memory = memory
			vdevices = append(vdevices, vd)
		}
	}
	return vdevices
}

func VDevicesByIDs(vdevices []*VDevice, ids []string) ([]*VDevice, error) {
	m := make(map[string]*VDevice, len(vdevices))
	for _, vd := range vdevices {
		m[vd.ID] = vd
	}
	var vds []*VDevice
	for _, id := range ids {
		if vd, ok := m[id]; ok {
			vds = append(vds, vd)
		} else {
			return nil, fmt.Errorf("unknown device: %s", id)
		}
	}
	return vds, nil
}

func UniqueDeviceIDs(vdevices []*VDevice) []string {
	m := make(map[string]bool, len(vdevices))
	var ids []string
	for _, vd := range vdevices {
		if _, ok := m[vd.dev.ID]; !ok {
			m[vd.dev.ID] = true
			ids = append(ids, vd.dev.ID)
		}
	}
	return ids
}

//func VDeviceHealth(vdevices []*VDevice, id string, health string) {
//	for _, vd := range vdevices {
//		if vd.dev.ID == id {
//			vd.Health = health
//		}
//	}
//}
