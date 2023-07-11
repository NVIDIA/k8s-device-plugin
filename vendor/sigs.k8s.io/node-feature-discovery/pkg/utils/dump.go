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
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func KlogDump(v klog.Level, heading, prefix string, obj interface{}) {
	if klog.V(v).Enabled() {
		if heading != "" {
			klog.InfoDepth(1, heading)
		}

		d := strings.Split(Dump(obj), "\n")
		// Print all but the last empty line
		for i := 0; i < len(d)-1; i++ {
			klog.InfoDepth(1, prefix+d[i])
		}
	}
}

// Dump dumps an object into YAML textual format
func Dump(obj interface{}) string {
	out, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Sprintf("<!!! FAILED TO MARSHAL %T (%v) !!!>\n", obj, err)
	}
	return string(out)
}
