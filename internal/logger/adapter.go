/**
# Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package logger

import "k8s.io/klog/v2"

// defaultNvInfoVerbosity maps nvinfo debug messages to klog verbosity level 1.
const defaultNvInfoVerbosity = 1

// NvInfoAdapter is an adapter that implements the basicLogger interface using klog.
type NvInfoAdapter struct {
	Verbosity int
}

// NewNvInfoAdapter returns the default klog-backed logger for nvinfo.
func NewNvInfoAdapter() *NvInfoAdapter {
	return &NvInfoAdapter{Verbosity: defaultNvInfoVerbosity}
}

// Debugf logs a debug message with the specified format and arguments.
func (l *NvInfoAdapter) Debugf(format string, args ...any) {
	klog.V(klog.Level(l.Verbosity)).Infof(format, args...)
}

// Infof logs an info message with the specified format and arguments.
func (l *NvInfoAdapter) Infof(format string, args ...any) {
	klog.Infof(format, args...)
}
