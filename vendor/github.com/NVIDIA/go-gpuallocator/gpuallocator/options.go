/**
# Copyright 2023 NVIDIA CORPORATION
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

package gpuallocator

import (
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// deviceListBuilder stores the options required to build a list of linked devices.
type deviceListBuilder struct {
	nvmllib   nvml.Interface
	devicelib device.Interface
}

// Option defines a type for functional options for constructing device lists.
type Option func(*deviceListBuilder)

// WithNvmlLib provides an option to set the nvml library.
func WithNvmlLib(nvmllib nvml.Interface) Option {
	return func(o *deviceListBuilder) {
		o.nvmllib = nvmllib
	}
}

// WithDeviceLib provides an option to set the library used for device enumeration.
func WithDeviceLib(devicelib device.Interface) Option {
	return func(o *deviceListBuilder) {
		o.devicelib = devicelib
	}
}
