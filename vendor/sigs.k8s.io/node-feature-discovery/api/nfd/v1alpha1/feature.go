/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import "maps"

// NewNodeFeatureSpec creates a new emprty instance of NodeFeatureSpec type,
// initializing all fields to proper empty values.
func NewNodeFeatureSpec() *NodeFeatureSpec {
	return &NodeFeatureSpec{
		Features: *NewFeatures(),
		Labels:   make(map[string]string),
	}
}

// NewFeatures creates a new instance of Features, initializing all feature
// types (flags, attributes and instances) to empty values.
func NewFeatures() *Features {
	return &Features{
		Flags:      make(map[string]FlagFeatureSet),
		Attributes: make(map[string]AttributeFeatureSet),
		Instances:  make(map[string]InstanceFeatureSet)}
}

// NewFlagFeatures creates a new instance of KeyFeatureSet.
func NewFlagFeatures(keys ...string) FlagFeatureSet {
	e := make(map[string]Nil, len(keys))
	for _, k := range keys {
		e[k] = Nil{}
	}
	return FlagFeatureSet{Elements: e}
}

// NewAttributeFeatures creates a new instance of ValueFeatureSet.
func NewAttributeFeatures(values map[string]string) AttributeFeatureSet {
	if values == nil {
		values = make(map[string]string)
	}
	return AttributeFeatureSet{Elements: values}
}

// NewInstanceFeatures creates a new instance of InstanceFeatureSet.
func NewInstanceFeatures(instances ...InstanceFeature) InstanceFeatureSet {
	return InstanceFeatureSet{Elements: instances}
}

// NewInstanceFeature creates a new InstanceFeature instance.
func NewInstanceFeature(attrs map[string]string) *InstanceFeature {
	if attrs == nil {
		attrs = make(map[string]string)
	}
	return &InstanceFeature{Attributes: attrs}
}

// InsertAttributeFeatures inserts new values into a specific feature.
func (f *Features) InsertAttributeFeatures(domain, feature string, values map[string]string) {
	if f.Attributes == nil {
		f.Attributes = make(map[string]AttributeFeatureSet)
	}
	key := domain + "." + feature
	if _, ok := f.Attributes[key]; !ok {
		f.Attributes[key] = NewAttributeFeatures(values)
		return
	}

	maps.Copy(f.Attributes[key].Elements, values)
}

// MergeInto merges two FeatureSpecs into one. Data in the input object takes
// precedence (overwrite) over data of the existing object we're merging into.
func (in *NodeFeatureSpec) MergeInto(out *NodeFeatureSpec) {
	in.Features.MergeInto(&out.Features)
	if in.Labels != nil {
		if out.Labels == nil {
			out.Labels = make(map[string]string, len(in.Labels))
		}
		maps.Copy(out.Labels, in.Labels)
	}
}

// MergeInto merges two sets of features into one. Features from the input set
// take precedence (overwrite) features from the existing features of the set
// we're merging into.
func (in *Features) MergeInto(out *Features) {
	if in.Flags != nil {
		if out.Flags == nil {
			out.Flags = make(map[string]FlagFeatureSet, len(in.Flags))
		}
		for key, val := range in.Flags {
			outVal := out.Flags[key]
			val.MergeInto(&outVal)
			out.Flags[key] = outVal
		}
	}
	if in.Attributes != nil {
		if out.Attributes == nil {
			out.Attributes = make(map[string]AttributeFeatureSet, len(in.Attributes))
		}
		for key, val := range in.Attributes {
			outVal := out.Attributes[key]
			val.MergeInto(&outVal)
			out.Attributes[key] = outVal
		}
	}
	if in.Instances != nil {
		if out.Instances == nil {
			out.Instances = make(map[string]InstanceFeatureSet, len(in.Instances))
		}
		for key, val := range in.Instances {
			outVal := out.Instances[key]
			val.MergeInto(&outVal)
			out.Instances[key] = outVal
		}
	}
}

// MergeInto merges two sets of flag featues.
func (in *FlagFeatureSet) MergeInto(out *FlagFeatureSet) {
	if in.Elements != nil {
		if out.Elements == nil {
			out.Elements = make(map[string]Nil, len(in.Elements))
		}
		maps.Copy(out.Elements, in.Elements)
	}
}

// MergeInto merges two sets of attribute featues.
func (in *AttributeFeatureSet) MergeInto(out *AttributeFeatureSet) {
	if in.Elements != nil {
		if out.Elements == nil {
			out.Elements = make(map[string]string, len(in.Elements))
		}
		maps.Copy(out.Elements, in.Elements)
	}
}

// MergeInto merges two sets of instance featues.
func (in *InstanceFeatureSet) MergeInto(out *InstanceFeatureSet) {
	if in.Elements != nil {
		if out.Elements == nil {
			out.Elements = make([]InstanceFeature, 0, len(in.Elements))
		}
		for _, e := range in.Elements {
			out.Elements = append(out.Elements, *e.DeepCopy())
		}
	}
}
