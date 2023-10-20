/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package resource

import (
	"fmt"

	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

type nvmlDevice struct {
	device.Device
	devicelib device.Interface
}

var _ Device = (*nvmlDevice)(nil)

// GetMigDevices returns the list of MIG devices configured on this device
func (d nvmlDevice) GetMigDevices() ([]Device, error) {
	migs, err := d.Device.GetMigDevices()
	if err != nil {
		return nil, err
	}

	var devices []Device
	for _, m := range migs {
		device := nvmlMigDevice{
			MigDevice: m,
			devicelib: d.devicelib,
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// GetCudaComputeCapability returns the CUDA major and minor versions.
func (d nvmlDevice) GetCudaComputeCapability() (int, int, error) {
	major, minor, ret := d.Device.GetCudaComputeCapability()
	if ret != nvml.SUCCESS {
		return 0, 0, ret
	}

	return major, minor, nil
}

// GetAttributes is only supported for MIG devices.
func (d nvmlDevice) GetAttributes() (map[string]interface{}, error) {
	return nil, fmt.Errorf("GetAttributes is not supported for non-MIG devices")
}

// GetDeviceHandleFromMigDeviceHandle is only supported for MIG devices
func (d nvmlDevice) GetDeviceHandleFromMigDeviceHandle() (Device, error) {
	return nil, fmt.Errorf("GetDeviceHandleFromMigDeviceHandle is not supported for non-MIG devices")
}

// GetName returns the device name / model.
func (d nvmlDevice) GetName() (string, error) {
	name, ret := d.Device.GetName()
	if ret != nvml.SUCCESS {
		return "", ret
	}
	return name, nil
}

// GetTotalMemoryMB returns the total memory on a device in MB
func (d nvmlDevice) GetTotalMemoryMB() (uint64, error) {
	info, ret := d.Device.GetMemoryInfo()
	if ret != nvml.SUCCESS {
		return 0, ret
	}
	return info.Total / (1024 * 1024), nil
}
