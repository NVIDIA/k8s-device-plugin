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
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// These constants define the possible strategies for striping through GPUs when time-slicing them.
const (
	UnspecifiedTimeSlicingStrategy = ""
	// TODO: Add support for additional strategies below
	//RoundRobinTimeSlicingStrategy = "round-robin"
	//PackedTimeSlicingStrategy     = "packed"
)

// TimeSlicing defines the set of replicas to be made for timeSlicing available resources.
type TimeSlicing struct {
	Strategy  string            `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Resources []ReplicaResource `json:"resources"          yaml:"resources"`
}

// ReplicaResource represents a resource to be replicated.
type ReplicaResource struct {
	Name     ResourceName   `json:"name"             yaml:"name"`
	Rename   ResourceName   `json:"rename,omitempty" yaml:"rename,omitempty"`
	Devices  ReplicaDevices `json:"devices"          yaml:"devices,flow"`
	Replicas int            `json:"replicas"         yaml:"replicas"`
}

// ReplicaDevices encapsulates the set of devices that should be replicated for a given resource.
// This struct should be treated as a 'union' and only one of the fields in this struct should be set at any given time.
type ReplicaDevices struct {
	All   bool
	Count int
	List  []ReplicaDeviceRef
}

// ReplicaDeviceRef can either be a full GPU index, a MIG index, or a UUID (full GPU or MIG)
type ReplicaDeviceRef string

// IsGPUIndex checks if a ReplicaDeviceRef is a full GPU index
func (d ReplicaDeviceRef) IsGPUIndex() bool {
	if _, err := strconv.ParseUint(string(d), 10, 0); err != nil {
		return false
	}
	return true
}

// IsMigIndex checks if a ReplicaDeviceRef is a MIG index
func (d ReplicaDeviceRef) IsMigIndex() bool {
	split := strings.SplitN(string(d), ":", 2)
	if len(split) != 2 {
		return false
	}
	for _, s := range split {
		if _, err := strconv.ParseUint(s, 10, 0); err != nil {
			return false
		}
	}
	return true
}

// IsUUID checks if a ReplicaDeviceRef is a UUID
func (d ReplicaDeviceRef) IsUUID() bool {
	return d.IsGpuUUID() || d.IsMigUUID()
}

// IsGpuUUID checks if a ReplicaDeviceRef is a GPU UUID
// A GPU UUID must be of the form GPU-b1028956-cfa2-0990-bf4a-5da9abb51763
func (d ReplicaDeviceRef) IsGpuUUID() bool {
	if !strings.HasPrefix(string(d), "GPU-") {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(string(d), "GPU-"))
	return err == nil
}

// IsMigUUID checks if a ReplicaDeviceRef is a MIG UUID
// A MIG UUID can be of one of two forms:
//    - MIG-b1028956-cfa2-0990-bf4a-5da9abb51763
//    - MIG-GPU-b1028956-cfa2-0990-bf4a-5da9abb51763/3/0
func (d ReplicaDeviceRef) IsMigUUID() bool {
	if !strings.HasPrefix(string(d), "MIG-") {
		return false
	}
	suffix := strings.TrimPrefix(string(d), "MIG-")
	_, err := uuid.Parse(suffix)
	if err == nil {
		return true
	}
	split := strings.SplitN(suffix, "/", 3)
	if len(split) != 3 {
		return false
	}
	if !ReplicaDeviceRef(split[0]).IsGpuUUID() {
		return false
	}
	for _, s := range split[1:] {
		_, err := strconv.ParseUint(s, 10, 0)
		if err != nil {
			return false
		}
	}
	return true
}

// UnmarshalJSON unmarshals raw bytes into a 'TimeSlicing' struct.
func (s *TimeSlicing) UnmarshalJSON(b []byte) error {
	ts := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &ts)
	if err != nil {
		return err
	}

	strategy, exists := ts["strategy"]
	if !exists {
		strategy = []byte(fmt.Sprintf(`"%s"`, UnspecifiedTimeSlicingStrategy))
	}

	err = json.Unmarshal(strategy, &s.Strategy)
	if err != nil {
		return err
	}

	switch s.Strategy {
	case UnspecifiedTimeSlicingStrategy:
	default:
		return fmt.Errorf("unknown time-slicing strategy: %v", s.Strategy)
	}

	resources, exists := ts["resources"]
	if !exists {
		return fmt.Errorf("no resources specified")
	}

	err = json.Unmarshal(resources, &s.Resources)
	if err != nil {
		return err
	}

	if len(s.Resources) == 0 {
		return fmt.Errorf("no resources specified")
	}

	return nil
}

// UnmarshalJSON unmarshals raw bytes into a 'ReplicaResource' struct.
func (s *ReplicaResource) UnmarshalJSON(b []byte) error {
	rr := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &rr)
	if err != nil {
		return err
	}

	name, exists := rr["name"]
	if !exists {
		return fmt.Errorf("no resource name specified")
	}

	err = json.Unmarshal(name, &s.Name)
	if err != nil {
		return err
	}

	err = s.Name.AssertValid()
	if err != nil {
		return fmt.Errorf("incorrect format for time-sliced resource name '%v'", name)
	}

	devices, exists := rr["devices"]
	if !exists {
		devices = []byte(`"all"`)
	}

	err = json.Unmarshal(devices, &s.Devices)
	if err != nil {
		return err
	}

	replicas, exists := rr["replicas"]
	if !exists {
		return fmt.Errorf("no replicas specified")
	}

	err = json.Unmarshal(replicas, &s.Replicas)
	if err != nil {
		return err
	}

	if s.Replicas <= 0 {
		return fmt.Errorf("number of replicas must be >= 0")
	}

	rename, exists := rr["rename"]
	if !exists {
		return nil
	}

	err = json.Unmarshal(rename, &s.Rename)
	if err != nil {
		return err
	}

	err = s.Rename.AssertValid()
	if err != nil {
		return fmt.Errorf("incorrect format for renamed resource '%v'", s.Rename)
	}

	return nil
}

// UnmarshalJSON unmarshals raw bytes into a 'ReplicaDevices' struct.
func (s *ReplicaDevices) UnmarshalJSON(b []byte) error {
	// Match the string 'all'
	var str string
	err := json.Unmarshal(b, &str)
	if err == nil {
		if str != "all" {
			return fmt.Errorf("devices set as '%v' but the only valid string input is 'all'", str)
		}
		s.All = true
		return nil
	}

	// Match a count
	var count int
	err = json.Unmarshal(b, &count)
	if err == nil {
		if count <= 0 {
			return fmt.Errorf("devices set as '%v' but a count of devices must be > 0", count)
		}
		s.Count = count
		return nil
	}

	// Match a list
	var slice []json.RawMessage
	err = json.Unmarshal(b, &slice)
	if err == nil {
		// For each item in the list check its format and convert it to a string (if necessary)
		result := make([]ReplicaDeviceRef, len(slice))
		for i, s := range slice {
			// Match a uint as a GPU index and convert it to a string
			var index uint
			if err = json.Unmarshal(s, &index); err == nil {
				result[i] = ReplicaDeviceRef(strconv.Itoa(int(index)))
				continue
			}
			// Match strings as valid entries if they are GPU indices, MIG indices, or UUIDs
			var item string
			if err = json.Unmarshal(s, &item); err == nil {
				rd := ReplicaDeviceRef(item)
				if rd.IsGPUIndex() || rd.IsMigIndex() || rd.IsUUID() {
					result[i] = rd
					continue
				}
			}
			// Treat any other entries as errors
			return fmt.Errorf("unsupported type for device in devices list: %v, %T", item, item)
		}
		s.List = result
		return nil
	}

	// No matches found
	return fmt.Errorf("unrecognized type for devices spec: %v", string(b))
}

// MarshalJSON marshals ReplicaDevices to its raw bytes representation
func (s *ReplicaDevices) MarshalJSON() ([]byte, error) {
	if s.All == true {
		return json.Marshal("all")
	}
	if s.Count > 0 {
		return json.Marshal(s.Count)
	}
	if s.List != nil {
		return json.Marshal(s.List)
	}
	return nil, fmt.Errorf("unmarshallable ReplicaDevices struct: %v", s)
}
