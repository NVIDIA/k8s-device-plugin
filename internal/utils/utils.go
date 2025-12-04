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

package utils

import (
	"bufio"
	"os"
	"slices"
	"strings"
)

// Supported driver module constants.
const (
	GDRCOPY_DRIVER_MODULE = "gdrdrv"
)

// IsGdrdrvLoaded checks if the "gdrdrv" driver module is loaded.
func IsGdrdrvLoaded() bool {
	return isDriverLoaded(GDRCOPY_DRIVER_MODULE)
}

// isDriverLoaded checks if the specified driver module is loaded by reading the modules file.
// It first checks "/host/proc/modules" (for containerized environments) and falls back to "/proc/modules" if not found.
func isDriverLoaded(module string) bool {
	paths := []string{"/host/proc/modules", "/proc/modules"}
	return slices.ContainsFunc(paths, func(path string) bool {
		return isDriverLoadedWithPath(module, path)
	})
}

// isDriverLoadedWithPath checks if the specified driver module is loaded by reading the specified modules file.
func isDriverLoadedWithPath(module, modulesPath string) bool {
	f, err := os.Open(modulesPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, module+" ") {
			return true
		}
	}
	return false
}
