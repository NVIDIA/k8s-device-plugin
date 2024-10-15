/**
# Copyright 2024 NVIDIA CORPORATION
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

package dgpu

import (
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"

	"github.com/NVIDIA/nvidia-container-toolkit/internal/discover"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/logger"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/nvcaps"
)

// NewForDevice creates a discoverer for the specified Device.
func NewForDevice(d device.Device, opts ...Option) (discover.Discover, error) {
	o := new(opts...)

	return o.newNvmlDGPUDiscoverer(&toRequiredInfo{d})
}

// NewForDevice creates a discoverer for the specified device and its associated MIG device.
func NewForMigDevice(d device.Device, mig device.MigDevice, opts ...Option) (discover.Discover, error) {
	o := new(opts...)

	return o.newNvmlMigDiscoverer(
		&toRequiredMigInfo{
			MigDevice: mig,
			parent:    &toRequiredInfo{d},
		},
	)
}

func new(opts ...Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	if o.logger == nil {
		o.logger = logger.New()
	}

	if o.migCaps == nil {
		migCaps, err := nvcaps.NewMigCaps()
		if err != nil {
			o.logger.Debugf("ignoring error getting MIG capability device paths: %v", err)
			o.migCapsError = err
		} else {
			o.migCaps = migCaps
		}
	}

	return o
}
