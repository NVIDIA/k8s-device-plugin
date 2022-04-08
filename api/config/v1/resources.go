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

const (
	productName    = "productName"
	migProfileName = "migProfileName"
	resourceName   = "resourceName"
)

// DeviceSelector is used to pair a selected device to a resource name
type DeviceSelector string

// Resource pairs a device selector with a resource name.
// Only one of ProductName or MigProfileName should ever be set for a given resource.
type Resource struct {
	ProductName    DeviceSelector `json:"productName,omitempty"    yaml:"productName,omitempty"`
	MigProfileName DeviceSelector `json:"migProfileName,omitempty" yaml:"migProfileName,omitempty"`
	ResourceName   string         `json:"resourceName"             yaml:"resourceName"`
}

// AddResourceNamePrefix builds a resource name from a prefix and a name
func AddResourceNamePrefix(name string) string {
	return ResourceNamePrefix + "/" + name
}

// SplitResourceName splits a full resource name into prefix and name
func SplitResourceName(name string) (string, string) {
	split := strings.SplitN(name, "/", 2)
	if len(split) != 2 {
		return "", name
	}
	return split[0], split[1]
}

// UnmarshalJSON unmarshals raw bytes into a 'Resource' struct.
func (r *Resource) UnmarshalJSON(b []byte) error {
	res := make(map[string]string)
	err := json.Unmarshal(b, &res)
	if err != nil {
		return err
	}

	// Verify correct fields set in the resource JSON
	_, resourceNameExists := res[resourceName]
	_, productNameExists := res[productName]
	_, migProfileNameExists := res[migProfileName]

	if !resourceNameExists {
		return fmt.Errorf("resources must have a '%v' field set", resourceName)
	}
	if !productNameExists && !migProfileNameExists {
		return fmt.Errorf("resources must have a '%v' or '%v' field set", productName, migProfileName)
	}
	if len(res) != 2 {
		return fmt.Errorf("resources should have exactly two fields set")
	}

	// Set r.ResourceName from the resource JSON
	prefixedResourceName := AddResourceNamePrefix(res[resourceName])
	if len(prefixedResourceName) > MaxResourceNameLength {
		return fmt.Errorf("fully-qualified resource name must be %v characters or less: %v", MaxResourceNameLength, prefixedResourceName)
	}
	invalid := k8s.NameIsDNSSubdomain(res[resourceName], false)
	if len(invalid) != 0 {
		return fmt.Errorf("incorrect format for resource name '%v': %v", res[resourceName], invalid)
	}
	r.ResourceName = res[resourceName]

	// Set one of r.ProductName or r.MigProfileName from the resource map
	if productNameExists {
		r.ProductName = DeviceSelector(res[productName])
	}
	if migProfileNameExists {
		r.MigProfileName = DeviceSelector(res[migProfileName])
	}

	return nil
}

// IsGPUResource indicates if the resource pairs a full GPU to a resource name
func (r *Resource) IsGPUResource() bool {
	return r.ProductName != ""
}

// IsMigResource indicates if the resource pairs a MIG device to a resource name
func (r *Resource) IsMigResource() bool {
	return r.MigProfileName != ""
}

// Matches checks if the provided string matches the DeviceSelector or not.
func (d DeviceSelector) Matches(s string) bool {
	result, _ := regexp.MatchString(wildCardToRegexp(string(d)), s)
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
