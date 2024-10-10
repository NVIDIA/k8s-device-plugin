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

package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

const (
	ImexChannelEnvVar = "NVIDIA_IMEX_CHANNELS"

	DefaultChannelStrategyDisabled = DefaultChannelStrategy("disabled")
	DefaultChannelStrategyAuto     = DefaultChannelStrategy("auto")
	DefaultChannelStrategyEnabled  = DefaultChannelStrategy("enabled")
)

var errInvalidDefaultChannelStrategy = errors.New("invalid default channel strategy")

// Imex stores the configuration options for fabric-attached devices.
type Imex struct {
	DefaultChannelStrategy DefaultChannelStrategy `json:"defaultChannelStrategy,omitempty" yaml:"defaultChannelStrategy,omitempty"`
}

// DefaultChannelStrategy defines the strategy for the injection of the default IMEX channel.
// The following values are applicable:
//   - `disabled` (default): no IMEX channels are added to the allocate response.
//   - `enabled`: the default IMEX channel is added to the allocate response for any requested device.
//   - `auto`: if the device nodes for the default IMEX channel is discoverable by the plugin this
//     behaves the same way as `enabled`. Otherwise `disabled` is selected.
type DefaultChannelStrategy string

// UnmarshalJSON implements the custom unmarshaler for the defaultChannelStrategy type.
// The option allows for the strategy to be set in a a number of ways.
func (f *DefaultChannelStrategy) UnmarshalJSON(b []byte) error {
	var value interface{}
	if len(b) > 0 {
		err := json.Unmarshal(b, &value)
		if err != nil {
			return err
		}
	}

	s, err := asDefaultChannelStrategy(value)
	if err != nil {
		return err
	}

	*f = s

	return nil
}

// asDefaultChannelStrategy converts an input value to a DefaultChannelStrategy.
func asDefaultChannelStrategy(from interface{}) (DefaultChannelStrategy, error) {
	if from == nil {
		return DefaultChannelStrategyDisabled, nil
	}

	switch t := from.(type) {
	case DefaultChannelStrategy:
		return t, nil
	case string:
		switch from {
		case "auto":
			return DefaultChannelStrategyAuto, nil
		case "1", "yes", "on", "enabled":
			return DefaultChannelStrategyEnabled, nil
		case "", "0", "no", "off", "disabled":
			return DefaultChannelStrategyDisabled, nil
		}
	case int, int64:
		return asDefaultChannelStrategy(fmt.Sprintf("%d", t))
	case float64:
		if t == math.Trunc(t) {
			return asDefaultChannelStrategy(fmt.Sprintf("%.0f", t))
		}
	}
	return DefaultChannelStrategyDisabled, fmt.Errorf("%w (%T): %v", errInvalidDefaultChannelStrategy, from, from)
}
