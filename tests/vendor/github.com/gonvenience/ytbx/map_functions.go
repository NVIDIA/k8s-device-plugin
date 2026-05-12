// Copyright © 2018 The Homeport Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ytbx

import (
	yamlv3 "gopkg.in/yaml.v3"
)

// listKeys returns a list of the keys of a Go-YAML v3 MappingNode (map)
func listKeys(mappingNode *yamlv3.Node) []string {
	keys := []string{}
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keys = append(keys, mappingNode.Content[i].Value)
	}

	return keys
}

// ListStringKeys lists the keys in a MappingNode
func ListStringKeys(mappingNode *yamlv3.Node) ([]string, error) {
	return listKeys(mappingNode), nil
}

// getValueByKey returns the value for a given key in a provided mapping node,
// or nil with an error if there is no such entry. This is comparable to getting
// a value from a map with `foobar[key]`.
func getValueByKey(mappingNode *yamlv3.Node, key string) (*yamlv3.Node, error) {
	for i := 0; i < len(mappingNode.Content); i += 2 {
		k, v := mappingNode.Content[i], mappingNode.Content[i+1]
		if k.Value == key {
			return v, nil
		}
	}

	return nil, &KeyNotFoundInMapError{
		MissingKey:    key,
		AvailableKeys: listKeys(mappingNode),
	}
}
