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

package utils

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

type dumper struct {
	obj interface{}
}

// String implements the fmt.Stringer interface
func (d *dumper) String() string {
	return Dump(d.obj)
}

// DelayedDumper delays the dumping of an object. Useful in logging to delay
// the processing (JSON marshalling) until (or if) the object is actually
// evaluated.
func DelayedDumper(obj interface{}) fmt.Stringer {
	return &dumper{obj: obj}
}

// Dump dumps an object into YAML textual format
func Dump(obj interface{}) string {
	out, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("<!!! FAILED TO MARSHAL %T (%v) !!!>\n", obj, err)
	}
	return string(out)
}
