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

import "github.com/sirupsen/logrus"

// SetLevel sets the level of the standard logger. The standard logger is shared
// by the libraries used by the plugin components, such as the nvlib platform
// resolution and the CDI spec generation, so enabling debug logging here also
// enables their debug messages.
func SetLevel(debug bool) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
		return
	}
	logrus.SetLevel(logrus.InfoLevel)
}
