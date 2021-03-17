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

// VDevice virtual device
type VDevice struct {
	pluginapi.Device
	dev    *Device
	memory uint64
}

// Device2VDevice device to virtual device
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

// VDevicesByIDs filter vdevices by uuids
func VDevicesByIDs(vdevices []*VDevice, ids []string) ([]*VDevice, error) {
	//var vds []*VDevice
	vds := make([]*VDevice, len(ids))
OUTER:
	for i, id := range ids {
		for _, vd := range vdevices {
			if vd.ID == id {
				vds[i] = vd
				continue OUTER
			}
		}
		return nil, fmt.Errorf("unknown device: %s", id)
	}
	return vds, nil
}

// UniqueDeviceIDs get unique real device ids from vdevices
func UniqueDeviceIDs(vdevices []*VDevice) []string {
	var ids []string
OUTER:
	for _, vd := range vdevices {
		for _, id := range ids {
			if id == vd.dev.ID {
				continue OUTER
			}
		}
		ids = append(ids, vd.dev.ID)
	}
	return ids
}
