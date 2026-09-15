/*
 * Copyright (c), NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1

import (
	"fmt"
	"strings"
)

// DeviceListStrategies defines which strategies are enabled and should
// be used when passing the device list to the container runtime.
type DeviceListStrategies map[string]bool

// supportedDeviceListStrategies lists every accepted strategy.
var supportedDeviceListStrategies = []string{
	DeviceListStrategyEnvVar,
	DeviceListStrategyVolumeMounts,
	DeviceListStrategyCDIAnnotations,
	DeviceListStrategyCDICRI,
}

// NewDeviceListStrategies constructs a new DeviceListStrategy from the
// requested strategies. At least one strategy is required.
func NewDeviceListStrategies(strategies []string) (DeviceListStrategies, error) {
	if len(strategies) == 0 {
		return nil, fmt.Errorf("no device list strategy specified; at least one of %v is required", supportedDeviceListStrategies)
	}

	ret := make(map[string]bool, len(supportedDeviceListStrategies))
	for _, s := range supportedDeviceListStrategies {
		ret[s] = false
	}
	for _, s := range strategies {
		if _, ok := ret[s]; !ok {
			return nil, fmt.Errorf("invalid strategy: %v", s)
		}
		ret[s] = true
	}

	return DeviceListStrategies(ret), nil
}

// Includes returns whether the given strategy is present in the set of strategies.
func (s DeviceListStrategies) Includes(strategy string) bool {
	return s[strategy]
}

// AnyCDIEnabled returns whether any of the strategies being used require CDI.
func (s DeviceListStrategies) AnyCDIEnabled() bool {
	for k, v := range s {
		if strings.HasPrefix(k, "cdi-") && v {
			return true
		}
	}
	return false
}

// AllCDIEnabled returns whether all strategies being used require CDI.
func (s DeviceListStrategies) AllCDIEnabled() bool {
	for k, v := range s {
		if !strings.HasPrefix(k, "cdi-") && v {
			return false
		}
	}
	return true
}
