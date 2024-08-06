// Copyright © 2023 The Homeport Team
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

package dyff

import (
	"fmt"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

// listItemIdentifier defines the contract for an list item identifier
type listItemIdentifier interface {
	// Name returns an unique name from the given entry, or an error in case this
	// is not possible due to missing fields
	Name(mappingNode *yamlv3.Node) (string, error)

	// FindNodeByName returns the node that matches the given name, or an error in
	// case it cannot find such an entry or required fields are missing
	FindNodeByName(sequenceNode *yamlv3.Node, name string) (*yamlv3.Node, error)

	// String returns a reprensation or explanation of the identifier itself
	String() string
}

// --- --- ---

// singleField is an list item identifier that relies on one field to serve as
// the defining field to differentiate between list items, e.g. 'name', or 'id'
type singleField struct {
	IdentifierFieldName string
}

var _ listItemIdentifier = &singleField{}

func (sf *singleField) FindNodeByName(sequenceNode *yamlv3.Node, name string) (*yamlv3.Node, error) {
	for _, mappingNode := range sequenceNode.Content {
		nameOfNode, err := sf.Name(mappingNode)
		if err != nil {
			return nil, err
		}

		if nameOfNode == name {
			return mappingNode, nil
		}
	}

	return nil, fmt.Errorf("failed to find mapping entry with name %q", name)
}

func (sf *singleField) Name(mappingNode *yamlv3.Node) (string, error) {
	result, err := grab(mappingNode, sf.IdentifierFieldName)
	if err != nil {
		return "", err
	}

	return followAlias(result).Value, nil
}

func (sf *singleField) String() string {
	return sf.IdentifierFieldName
}

// --- --- ---

// k8sItemIdentifier is an identifier aiming for Kubernetes items that have an
// api version, kind, and name field to be used
type k8sItemIdentifier struct{}

var k8sItem listItemIdentifier = &k8sItemIdentifier{}

func (i *k8sItemIdentifier) FindNodeByName(sequenceNode *yamlv3.Node, name string) (*yamlv3.Node, error) {
	for _, mappingNode := range sequenceNode.Content {
		nameOfNode, err := i.Name(mappingNode)
		if err != nil {
			return nil, err
		}

		if nameOfNode == name {
			return mappingNode, nil
		}
	}

	return nil, fmt.Errorf("failed to find mapping entry with name %q", name)
}

func (i *k8sItemIdentifier) Name(node *yamlv3.Node) (string, error) {
	if node.Kind != yamlv3.MappingNode {
		return "", fmt.Errorf("provided node is not a mapping node")
	}

	var elem []string

	apiVersion, err := grab(node, "apiVersion")
	if err != nil {
		return "", err
	}
	elem = append(elem, apiVersion.Value)

	kind, err := grab(node, "kind")
	if err != nil {
		return "", err
	}
	elem = append(elem, kind.Value)

	// namespace is optional and will be omitted if not set
	namespace, err := grab(node, "metadata.namespace")
	if err == nil {
		elem = append(elem, namespace.Value)
	}

	name, err := grab(node, "metadata.name")
	if err != nil {
		return "", err
	}
	elem = append(elem, name.Value)

	return strings.Join(elem, "/"), nil
}

func (lf *k8sItemIdentifier) String() string {
	return "resource"
}
