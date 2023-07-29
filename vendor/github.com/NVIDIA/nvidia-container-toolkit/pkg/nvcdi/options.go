/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
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

package nvcdi

import (
	"github.com/sirupsen/logrus"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

// Option is a function that configures the nvcdilib
type Option func(*nvcdilib)

// WithDeviceLib sets the device library for the library
func WithDeviceLib(devicelib device.Interface) Option {
	return func(l *nvcdilib) {
		l.devicelib = devicelib
	}
}

// WithDeviceNamer sets the device namer for the library
func WithDeviceNamer(namer DeviceNamer) Option {
	return func(l *nvcdilib) {
		l.deviceNamer = namer
	}
}

// WithDriverRoot sets the driver root for the library
func WithDriverRoot(root string) Option {
	return func(l *nvcdilib) {
		l.driverRoot = root
	}
}

// WithLogger sets the logger for the library
func WithLogger(logger *logrus.Logger) Option {
	return func(l *nvcdilib) {
		l.logger = logger
	}
}

// WithNVIDIACTKPath sets the path to the NVIDIA Container Toolkit CLI path for the library
func WithNVIDIACTKPath(path string) Option {
	return func(l *nvcdilib) {
		l.nvidiaCTKPath = path
	}
}

// WithNvmlLib sets the nvml library for the library
func WithNvmlLib(nvmllib nvml.Interface) Option {
	return func(l *nvcdilib) {
		l.nvmllib = nvmllib
	}
}

// WithMode sets the discovery mode for the library
func WithMode(mode string) Option {
	return func(l *nvcdilib) {
		l.mode = mode
	}
}

// WithVendor sets the vendor for the library
func WithVendor(vendor string) Option {
	return func(o *nvcdilib) {
		o.vendor = vendor
	}
}

// WithClass sets the class for the library
func WithClass(class string) Option {
	return func(o *nvcdilib) {
		o.class = class
	}
}
