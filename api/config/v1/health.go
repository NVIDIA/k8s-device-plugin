/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// EventType represents an NVML event type name that can be parsed from configuration
type EventType string

const (
	EventTypeXidCriticalError  EventType = "EventTypeXidCriticalError"
	EventTypeDoubleBitEccError EventType = "EventTypeDoubleBitEccError"
	EventTypeSingleBitEccError EventType = "EventTypeSingleBitEccError"
)

// ToNVMLEventType converts the string representation to the NVML event type value
func (e EventType) ToNVMLEventType() (uint64, error) {
	switch e {
	case EventTypeXidCriticalError:
		return uint64(nvml.EventTypeXidCriticalError), nil
	case EventTypeDoubleBitEccError:
		return uint64(nvml.EventTypeDoubleBitEccError), nil
	case EventTypeSingleBitEccError:
		return uint64(nvml.EventTypeSingleBitEccError), nil
	default:
		return 0, fmt.Errorf("unknown event type: %s", e)
	}
}

// CriticalXIDs represents either a list of specific XID values or "all"
type CriticalXIDs struct {
	All      bool     `json:"-" yaml:"-"`
	Specific []uint64 `json:"-" yaml:"-"`
}

// UnmarshalJSON implements custom JSON unmarshaling for CriticalXIDs
func (c *CriticalXIDs) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if strings.ToLower(str) == "all" {
			c.All = true
			c.Specific = nil
			return nil
		}
		return fmt.Errorf("invalid criticalXIDs string value: %s (must be 'all' or array of numbers)", str)
	}

	// Try to unmarshal as array of numbers
	var nums []uint64
	if err := json.Unmarshal(data, &nums); err == nil {
		c.All = false
		c.Specific = nums
		return nil
	}

	return fmt.Errorf("criticalXIDs must be 'all' or array of numbers")
}

// MarshalJSON implements custom JSON marshaling for CriticalXIDs
func (c CriticalXIDs) MarshalJSON() ([]byte, error) {
	if c.All {
		return json.Marshal("all")
	}
	return json.Marshal(c.Specific)
}

// UnmarshalYAML implements custom YAML unmarshaling for CriticalXIDs
func (c *CriticalXIDs) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try string first
	var str string
	if err := unmarshal(&str); err == nil {
		if strings.ToLower(str) == "all" {
			c.All = true
			c.Specific = nil
			return nil
		}
		return fmt.Errorf("invalid criticalXIDs string value: %s (must be 'all' or array of numbers)", str)
	}

	// Try array of numbers
	var nums []uint64
	if err := unmarshal(&nums); err == nil {
		c.All = false
		c.Specific = nums
		return nil
	}

	return fmt.Errorf("criticalXIDs must be 'all' or array of numbers")
}

// MarshalYAML implements custom YAML marshaling for CriticalXIDs
func (c CriticalXIDs) MarshalYAML() (interface{}, error) {
	if c.All {
		return "all", nil
	}
	return c.Specific, nil
}

// Health holds configuration options for device health checking
type Health struct {
	// Disabled disables all health checking if true
	Disabled *bool `json:"disabled,omitempty" yaml:"disabled,omitempty"`
	// EventTypes specifies the NVML event types to monitor
	EventTypes []EventType `json:"eventTypes,omitempty" yaml:"eventTypes,omitempty"`
	// IgnoredXIDs specifies XID error codes to ignore (treat as non-fatal)
	IgnoredXIDs []uint64 `json:"ignoredXIDs,omitempty" yaml:"ignoredXIDs,omitempty"`
	// CriticalXIDs specifies XID error codes to treat as critical, or "all" for all XIDs
	CriticalXIDs *CriticalXIDs `json:"criticalXIDs,omitempty" yaml:"criticalXIDs,omitempty"`
}

// GetDisabled returns whether health checks are disabled, with a default of false
func (h *Health) GetDisabled() bool {
	if h.Disabled != nil {
		return *h.Disabled
	}
	return false
}

// GetEventTypes returns the event types to monitor, with NVIDIA defaults if not specified
func (h *Health) GetEventTypes() []EventType {
	if h.EventTypes != nil {
		return h.EventTypes
	}
	// Default NVIDIA event types
	return []EventType{
		EventTypeXidCriticalError,
		EventTypeDoubleBitEccError,
		EventTypeSingleBitEccError,
	}
}

// GetIgnoredXIDs returns the XIDs to ignore, with NVIDIA defaults if not specified
func (h *Health) GetIgnoredXIDs() []uint64 {
	if h.IgnoredXIDs != nil {
		return h.IgnoredXIDs
	}
	// Default NVIDIA ignored XIDs
	return []uint64{
		13,  // Graphics Engine Exception
		31,  // GPU memory page fault
		43,  // GPU stopped processing
		45,  // Preemptive cleanup, due to previous errors
		68,  // Video processor exception
		109, // Context Switch Timeout Error
	}
}

// GetCriticalXIDs returns the critical XIDs configuration
func (h *Health) GetCriticalXIDs() *CriticalXIDs {
	if h.CriticalXIDs != nil {
		return h.CriticalXIDs
	}
	// Default: all XIDs are critical (except those explicitly ignored)
	return &CriticalXIDs{All: true}
}

// GetEventMask returns the NVML event mask for the configured event types
func (h *Health) GetEventMask() (uint64, error) {
	eventTypes := h.GetEventTypes()
	var mask uint64
	for _, eventType := range eventTypes {
		nvmlType, err := eventType.ToNVMLEventType()
		if err != nil {
			return 0, err
		}
		mask |= nvmlType
	}
	return mask, nil
}

// IsXIDIgnored returns true if the given XID should be ignored (treated as non-fatal)
func (h *Health) IsXIDIgnored(xid uint64) bool {
	ignoredXIDs := h.GetIgnoredXIDs()
	for _, ignored := range ignoredXIDs {
		if ignored == xid {
			return true
		}
	}
	return false
}

// IsXIDCritical returns true if the given XID should be treated as critical
func (h *Health) IsXIDCritical(xid uint64) bool {
	criticalXIDs := h.GetCriticalXIDs()
	
	// If All is true, all XIDs are critical (except those explicitly ignored)
	if criticalXIDs.All {
		return !h.IsXIDIgnored(xid)
	}
	
	// Check if XID is in the specific critical list
	for _, critical := range criticalXIDs.Specific {
		if critical == xid {
			return true
		}
	}
	
	return false
}

// ApplyEnvironmentOverrides applies legacy environment variable overrides for backward compatibility
func (h *Health) ApplyEnvironmentOverrides(disableHealthChecks string) {
	if disableHealthChecks == "" {
		return
	}

	disableHealthChecks = strings.ToLower(disableHealthChecks)
	if disableHealthChecks == "all" {
		disableHealthChecks = "xids"
	}
	
	if strings.Contains(disableHealthChecks, "xids") {
		// Disable health checks entirely
		h.Disabled = ptr(true)
		return
	}

	// Parse as comma-separated list of XIDs to add to ignored list
	additionalIgnored := parseAdditionalXids(disableHealthChecks)
	if len(additionalIgnored) > 0 {
		existing := h.GetIgnoredXIDs()
		// Create a map to avoid duplicates
		ignoredMap := make(map[uint64]bool)
		for _, xid := range existing {
			ignoredMap[xid] = true
		}
		for _, xid := range additionalIgnored {
			ignoredMap[xid] = true
		}
		
		// Convert back to slice
		var finalIgnored []uint64
		for xid := range ignoredMap {
			finalIgnored = append(finalIgnored, xid)
		}
		h.IgnoredXIDs = finalIgnored
	}
}

// parseAdditionalXids parses a comma-separated string of XIDs, same logic as the original getAdditionalXids
func parseAdditionalXids(input string) []uint64 {
	if input == "" {
		return nil
	}

	var additionalXids []uint64
	for _, additionalXid := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(additionalXid)
		if trimmed == "" {
			continue
		}
		xid, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			// Ignore malformed values (same as original logic)
			continue
		}
		additionalXids = append(additionalXids, xid)
	}

	return additionalXids
}