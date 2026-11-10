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

package mps

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

var errInvalidDevice = errors.New("invalid device")

// mpsDevice represents an MPS-specific alias for an rm.Device.
type mpsDevice rm.Device

// assertReplicas checks whether the number of replicas specified is valid.
func (d *mpsDevice) assertReplicas() error {
	maxClients := d.maxClients()
	if d.Replicas > maxClients {
		return fmt.Errorf("%w maximum allowed replicas exceeded: %d > %d", errInvalidDevice, d.Replicas, maxClients)
	}
	return nil
}

// maxClients returns the maximum number of clients supported by an MPS server.
func (d *mpsDevice) maxClients() int {
	if d.isAtLeastVolta() {
		return 48
	}
	return 16
}

// isAtLeastVolta checks whether the specified device is a volta device or newer.
// An unparseable compute capability is treated as pre-Volta rather than an
// error so that devices that do not report one (e.g. WSL) keep the
// conservative client limit.
func (d *mpsDevice) isAtLeastVolta() bool {
	majorStr, minorStr, ok := strings.Cut(strings.TrimPrefix(d.ComputeCapability, "v"), ".")
	if !ok {
		return false
	}
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return false
	}
	if _, err := strconv.Atoi(minorStr); err != nil {
		return false
	}

	return major >= 7
}
