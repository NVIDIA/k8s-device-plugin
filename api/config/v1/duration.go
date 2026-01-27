/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Duration wraps a time.Duration function with custom JSON marshaling/unmarshaling
type Duration time.Duration

// IsInfinite returns true if the duration represents an infinite sleep interval.
func (d *Duration) IsInfinite() bool {
	return d != nil && time.Duration(*d) == math.MaxInt64
}

// String returns a human-readable representation of the duration.
func (d Duration) String() string {
	if d.IsInfinite() {
		return "infinite"
	}
	return time.Duration(d).String()
}

// MarshalJSON marshals 'Duration' to its raw bytes representation
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON unmarshals raw bytes into a 'Duration' type.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		return d.parse(value)
	default:
		return fmt.Errorf("invalid duration")
	}
}

// parse parses a duration string, handling the special "infinite" value.
func (d *Duration) parse(value string) error {
	if value == "infinite" {
		*d = Duration(math.MaxInt64)
		return nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// DurationValue implements cli.Generic for parsing duration flags with "infinite" support
type DurationValue struct {
	Value *Duration
}

// NewDurationValue creates a new DurationValue with the given default duration
func NewDurationValue(d time.Duration) *DurationValue {
	duration := Duration(d)
	return &DurationValue{Value: &duration}
}

// Set implements cli.Generic
func (d *DurationValue) Set(value string) error {
	return d.Value.parse(value)
}

// String implements cli.Generic
func (d *DurationValue) String() string {
	if d.Value == nil {
		return ""
	}
	if d.Value.IsInfinite() {
		return "infinite"
	}
	return time.Duration(*d.Value).String()
}
