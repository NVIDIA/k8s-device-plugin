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
	"github.com/NVIDIA/nvidia-container-toolkit/internal/logger"
)

type options struct {
	logger            logger.Interface
	devRoot           string
	nvidiaCDIHookPath string
}

type Option func(*options)

// WithDevRoot sets the root where /dev is located.
func WithDevRoot(root string) Option {
	return func(l *options) {
		l.devRoot = root
	}
}

// WithLogger sets the logger for the library
func WithLogger(logger logger.Interface) Option {
	return func(l *options) {
		l.logger = logger
	}
}

// WithNVIDIACDIHookPath sets the path to the NVIDIA Container Toolkit CLI path for the library
func WithNVIDIACDIHookPath(path string) Option {
	return func(l *options) {
		l.nvidiaCDIHookPath = path
	}
}
