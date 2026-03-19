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

import (
	"fmt"

	"github.com/go-logr/logr"
)

// defaultNvInfoVerbosity is the klog verbosity level at which nvinfo debug
// messages are emitted. nvinfo has a single debug level, so any threshold of
// 1 or higher enables all of them.
const defaultNvInfoVerbosity = 1

// NvInfoAdapter is an adapter that implements the basicLogger interface using logr.
type NvInfoAdapter struct {
	logger logr.Logger
}

// NewNvInfoAdapter returns a logger adapter for nvinfo.
func NewNvInfoAdapter(logger logr.Logger) *NvInfoAdapter {
	return &NvInfoAdapter{
		logger: logger.WithCallDepth(1),
	}
}

// Debugf logs a debug message with the specified format and arguments.
func (l *NvInfoAdapter) Debugf(format string, args ...any) {
	l.logger.V(defaultNvInfoVerbosity).Info(fmt.Sprintf(format, args...))
}

// Infof logs an info message with the specified format and arguments.
func (l *NvInfoAdapter) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}
