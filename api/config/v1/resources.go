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
	"regexp"
	"strings"

	k8s "k8s.io/apimachinery/pkg/api/validation"
)

// ResourcePattern is used to match a resource name to a specific pattern
type ResourcePattern string

// ResourceName represents a valid resource name in Kubernetes
type ResourceName string

// Resource pairs a pattern matcher with a resource name.
type Resource struct {
	Pattern ResourcePattern `json:"pattern" yaml:"pattern"`
	Name    ResourceName    `json:"name"    yaml:"name"`
}

// Resources lists full GPUs and MIG devices separately.
type Resources struct {
	GPUs []Resource `json:"gpus" yaml:"gpus"`
	MIGs []Resource `json:"mig"  yaml:"mig"`
}

// UnmarshalJSON unmarshals raw bytes into a 'Resource' struct.
func (r *Resource) UnmarshalJSON(b []byte) error {
	res := make(map[string]string)
	err := json.Unmarshal(b, &res)
	if err != nil {
		return err
	}

	// Verify correct fields set in the resource JSON
	if _, exists := res["pattern"]; !exists {
		return fmt.Errorf("resources must have a 'pattern' field set")
	}
	if _, exists := res["name"]; !exists {
		return fmt.Errorf("resources must have a 'name' field set")
	}

	// Set r.Pattern from the resource JSON
	r.Pattern = ResourcePattern(res["pattern"])

	// Set r.Name from the resource JSON
	err = ResourceName(res["name"]).AssertValid()
	if err != nil {
		return err
	}
	r.Name = ResourceName(res["name"])

	return nil
}

// AssertValid asserts that the given resource name is a valid Kubernetes resource name
func (n ResourceName) AssertValid() error {
	prefixedResourceName := n.AddPrefix()
	if len(prefixedResourceName) > MaxResourceNameLength {
		return fmt.Errorf("fully-qualified resource name must be %v characters or less: %v", MaxResourceNameLength, prefixedResourceName)
	}
	_, name := n.Split()
	invalid := k8s.NameIsDNSSubdomain(string(name), false)
	if len(invalid) != 0 {
		return fmt.Errorf("incorrect format for resource name '%v': %v", n, invalid)
	}
	return nil
}

// AddPrefix builds a resource name from the standard prefix and a name
func (n ResourceName) AddPrefix() ResourceName {
	_, name := n.Split()
	return ResourceName(ResourceNamePrefix + "/" + name)
}

// Split splits a full resource name into prefix and name
func (n ResourceName) Split() (string, string) {
	split := strings.SplitN(string(n), "/", 2)
	if len(split) != 2 {
		return "", string(n)
	}
	return split[0], split[1]
}

// Matches checks if the provided string matches the ResourcePattern or not.
func (p ResourcePattern) Matches(s string) bool {
	result, _ := regexp.MatchString(wildCardToRegexp(string(p)), s)
	return result
}

// wildCardToRegexp converts a wildcard pattern to a regular expression pattern.
func wildCardToRegexp(pattern string) string {
	var result strings.Builder
	for i, literal := range strings.Split(pattern, "*") {
		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}
		// Quote any regular expression meta characters in the literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return result.String()
}
