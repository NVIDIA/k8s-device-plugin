/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
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
	"strings"
)

// Health defines the configuration for device health checks
type Health struct {
	// Disabled indicates whether health checks are disabled entirely
	Disabled bool `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	// EventTypes specifies which NVML event types to monitor
	EventTypes []string `json:"eventTypes,omitempty" yaml:"eventTypes,omitempty"`
	// IgnoredXIDs lists XIDs that should be ignored (non-fatal)
	IgnoredXIDs []uint64 `json:"ignoredXIDs,omitempty" yaml:"ignoredXIDs,omitempty"`
	// CriticalXIDs specifies which XIDs are considered critical
	// Can be "all" or a list of specific XIDs
	CriticalXIDs *CriticalXIDsType `json:"criticalXIDs,omitempty" yaml:"criticalXIDs,omitempty"`
}

// CriticalXIDsType represents either "all" XIDs or a specific list
type CriticalXIDsType struct {
	// All indicates if all XIDs should be considered critical
	All bool
	// XIDs contains specific XIDs to treat as critical
	XIDs []uint64
}

// UnmarshalJSON implements custom JSON unmarshaling for CriticalXIDsType
func (c *CriticalXIDsType) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if strings.ToLower(str) == "all" {
			c.All = true
			c.XIDs = nil
			return nil
		}
		return fmt.Errorf("invalid string value for criticalXIDs: %s", str)
	}

	// Try to unmarshal as array of numbers
	var xids []uint64
	if err := json.Unmarshal(data, &xids); err == nil {
		c.All = false
		c.XIDs = xids
		return nil
	}

	return fmt.Errorf("criticalXIDs must be either \"all\" or an array of numbers")
}

// MarshalJSON implements custom JSON marshaling for CriticalXIDsType
func (c CriticalXIDsType) MarshalJSON() ([]byte, error) {
	if c.All {
		return json.Marshal("all")
	}
	return json.Marshal(c.XIDs)
}

// UnmarshalYAML implements custom YAML unmarshaling for CriticalXIDsType
func (c *CriticalXIDsType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as string first
	var str string
	if err := unmarshal(&str); err == nil {
		if strings.ToLower(str) == "all" {
			c.All = true
			c.XIDs = nil
			return nil
		}
		return fmt.Errorf("invalid string value for criticalXIDs: %s", str)
	}

	// Try to unmarshal as array of numbers
	var xids []uint64
	if err := unmarshal(&xids); err == nil {
		c.All = false
		c.XIDs = xids
		return nil
	}

	return fmt.Errorf("criticalXIDs must be either \"all\" or an array of numbers")
}

// MarshalYAML implements custom YAML marshaling for CriticalXIDsType
func (c CriticalXIDsType) MarshalYAML() (interface{}, error) {
	if c.All {
		return "all", nil
	}
	return c.XIDs, nil
}

// DefaultHealth returns the default health configuration for standard deployments
func DefaultHealth() *Health {
	return &Health{
		Disabled: false,
		EventTypes: []string{
			"EventTypeXidCriticalError",
			"EventTypeDoubleBitEccError",
			"EventTypeSingleBitEccError",
		},
		IgnoredXIDs: []uint64{
			13,  // Graphics Engine Exception
			31,  // GPU memory page fault
			43,  // GPU stopped processing
			45,  // Preemptive cleanup, due to previous errors
			68,  // Video processor exception
			109, // Context Switch Timeout Error
		},
		CriticalXIDs: &CriticalXIDsType{
			All: true,
		},
	}
}

// IsCritical checks if a given XID should be treated as critical
func (h *Health) IsCritical(xid uint64) bool {
	// If health checks are disabled, nothing is critical
	if h.Disabled {
		return false
	}

	// Check if XID is in ignored list
	for _, ignoredXID := range h.IgnoredXIDs {
		if xid == ignoredXID {
			return false
		}
	}

	// If no critical XIDs specified, default to all
	if h.CriticalXIDs == nil {
		return true
	}

	// If all XIDs are critical (except ignored ones)
	if h.CriticalXIDs.All {
		return true
	}

	// Check if XID is in critical list
	for _, criticalXID := range h.CriticalXIDs.XIDs {
		if xid == criticalXID {
			return true
		}
	}

	return false
}

// Validate checks if the health configuration is valid
func (h *Health) Validate() error {
	if h == nil {
		return nil
	}

	// Validate event types
	validEventTypes := map[string]bool{
		"EventTypeXidCriticalError":  true,
		"EventTypeDoubleBitEccError": true,
		"EventTypeSingleBitEccError": true,
	}

	for _, eventType := range h.EventTypes {
		if !validEventTypes[eventType] {
			return fmt.Errorf("invalid event type: %s", eventType)
		}
	}

	// Check for XID conflicts
	if h.CriticalXIDs != nil && !h.CriticalXIDs.All && len(h.CriticalXIDs.XIDs) > 0 {
		ignoredMap := make(map[uint64]bool)
		for _, xid := range h.IgnoredXIDs {
			ignoredMap[xid] = true
		}

		for _, xid := range h.CriticalXIDs.XIDs {
			if ignoredMap[xid] {
				return fmt.Errorf("XID %d is in both ignored and critical lists", xid)
			}
		}
	}

	return nil
}

// Merge applies values from another Health config, overriding the current values with
// values from 'other'. This includes the Disabled field - if 'other' explicitly sets
// Disabled to false, it will enable health checks even if they were previously disabled.
// This is intentional behavior for configuration merging where 'other' represents a
// higher-priority configuration source.
//
// For fields that are slices (EventTypes, IgnoredXIDs) or pointers (CriticalXIDs),
// only non-empty/non-nil values from 'other' will override the current values.
func (h *Health) Merge(other *Health) {
	if other == nil {
		return
	}

	h.Disabled = other.Disabled

	if len(other.EventTypes) > 0 {
		h.EventTypes = other.EventTypes
	}

	if len(other.IgnoredXIDs) > 0 {
		h.IgnoredXIDs = other.IgnoredXIDs
	}

	if other.CriticalXIDs != nil {
		h.CriticalXIDs = other.CriticalXIDs
	}
}
