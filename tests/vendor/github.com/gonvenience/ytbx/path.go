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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

var dotRegEx = regexp.MustCompile(`^((\d+):)?(.*)$`)

// PathStyle is a custom type for supported path styles
type PathStyle int

// Supported styles are the Dot-Style (used by Spruce for example) and GoPatch
// Style which is used by BOSH
const (
	DotStyle PathStyle = iota
	GoPatchStyle
)

// Path points to a section in a data structure by using names to identify the
// location.
// Example:
//   ---
//   sizing:
//     api:
//       count: 2
// For example, `sizing.api.count` points to the key `sizing` of the root
// element and in there to the key `api` and so on and so forth.
type Path struct {
	Root         *InputFile
	DocumentIdx  int
	PathElements []PathElement
}

// PathElement represents one part of a path, which can either address an entry
// in a map (by name), a named-entry list entry (key and name), or an entry in a
// list (by index).
type PathElement struct {
	Idx  int
	Key  string
	Name string
}

func (path Path) String() string {
	return path.ToGoPatchStyle()
}

// ToGoPatchStyle returns the path as a GoPatch style string.
func (path *Path) ToGoPatchStyle() string {
	if len(path.PathElements) == 0 {
		return "/"
	}

	sections := []string{""}
	for _, element := range path.PathElements {
		switch {
		case element.Name != "" && element.Key == "":
			sections = append(sections, element.Name)

		case element.Name != "" && element.Key != "":
			sections = append(sections, fmt.Sprintf("%s=%s", element.Key, element.Name))

		default:
			sections = append(sections, strconv.Itoa(element.Idx))
		}
	}

	return strings.Join(sections, "/")
}

// ToDotStyle returns the path as a Dot-Style string.
func (path *Path) ToDotStyle() string {
	sections := []string{}

	for _, element := range path.PathElements {
		switch {
		case element.Name != "":
			sections = append(sections, element.Name)

		case element.Idx >= 0:
			sections = append(sections, strconv.Itoa(element.Idx))
		}
	}

	return strings.Join(sections, ".")
}

// RootDescription returns a description of the root level of this path, which
// could be the number of the respective document inside a YAML or if available
// the name of the document
func (path *Path) RootDescription() string {
	if path.Root != nil && path.DocumentIdx < len(path.Root.Names) {
		return path.Root.Names[path.DocumentIdx]
	}

	// Note: human style counting that starts with 1
	return fmt.Sprintf("document #%d", path.DocumentIdx+1)
}

// NewPathWithPathElement returns a new path based on a given path adding a new
// path element.
func NewPathWithPathElement(path Path, pathElement PathElement) Path {
	result := make([]PathElement, len(path.PathElements))
	copy(result, path.PathElements)

	return Path{
		Root:         path.Root,
		DocumentIdx:  path.DocumentIdx,
		PathElements: append(result, pathElement)}
}

// NewPathWithNamedElement returns a new path based on a given path adding a new
// of type entry in map using the name.
func NewPathWithNamedElement(path Path, name interface{}) Path {
	return NewPathWithPathElement(path, PathElement{
		Idx:  -1,
		Name: fmt.Sprintf("%v", name)})
}

// NewPathWithNamedListElement returns a new path based on a given path adding a
// new of type entry in a named-entry list by using key and name.
func NewPathWithNamedListElement(path Path, identifier interface{}, name interface{}) Path {
	return NewPathWithPathElement(path, PathElement{
		Idx:  -1,
		Key:  fmt.Sprintf("%v", identifier),
		Name: fmt.Sprintf("%v", name)})
}

// NewPathWithIndexedListElement returns a new path based on a given path adding
// a new of type list entry using the index.
func NewPathWithIndexedListElement(path Path, idx int) Path {
	return NewPathWithPathElement(path, PathElement{
		Idx: idx,
	})
}

// ComparePathsByValue returns all Path structure that have the same path value
func ComparePathsByValue(fromLocation string, toLocation string, duplicatePaths []Path) ([]Path, error) {
	from, err := LoadFile(fromLocation)
	if err != nil {
		return nil, err
	}

	to, err := LoadFile(toLocation)
	if err != nil {
		return nil, err
	}

	if len(from.Documents) > 1 || len(to.Documents) > 1 {
		return nil, fmt.Errorf("input files have more than one document, which is not supported yet")
	}

	duplicatePathsWithTheSameValue := []Path{}

	for _, path := range duplicatePaths {
		fromValue, err := Grab(from.Documents[0], path.ToGoPatchStyle())
		if err != nil {
			return nil, err
		}

		toValue, err := Grab(to.Documents[0], path.ToGoPatchStyle())
		if err != nil {
			return nil, err
		}

		if reflect.DeepEqual(fromValue, toValue) {
			duplicatePathsWithTheSameValue = append(duplicatePathsWithTheSameValue, path)
		}
	}
	return duplicatePathsWithTheSameValue, nil
}

// ComparePaths returns all duplicate Path structures between two documents.
func ComparePaths(fromLocation string, toLocation string, compareByValue bool) ([]Path, error) {
	var duplicatePaths []Path

	pathsFromLocation, err := ListPaths(fromLocation)
	if err != nil {
		return nil, err
	}
	pathsToLocation, err := ListPaths(toLocation)
	if err != nil {
		return nil, err
	}

	lookup := map[string]struct{}{}
	for _, pathsFrom := range pathsFromLocation {
		lookup[pathsFrom.ToGoPatchStyle()] = struct{}{}
	}

	for _, pathsTo := range pathsToLocation {
		if _, ok := lookup[pathsTo.ToGoPatchStyle()]; ok {
			duplicatePaths = append(duplicatePaths, pathsTo)
		}
	}

	if !compareByValue {
		return duplicatePaths, nil
	}

	return ComparePathsByValue(fromLocation, toLocation, duplicatePaths)
}

// ListPaths returns all paths in the documents using the provided choice of
// path style.
func ListPaths(location string) ([]Path, error) {
	inputfile, err := LoadFile(location)
	if err != nil {
		return nil, err
	}

	paths := []Path{}
	for idx, document := range inputfile.Documents {
		root := Path{DocumentIdx: idx}

		traverseTree(root, nil, document, func(path Path, _ *yamlv3.Node, _ *yamlv3.Node) {
			paths = append(paths, path)
		})
	}

	return paths, nil
}

// IsPathInTree returns whether the provided path is in the given YAML structure
func IsPathInTree(tree *yamlv3.Node, pathString string) (bool, error) {
	searchPath, err := ParsePathString(pathString, tree)
	if err != nil {
		return false, err
	}

	resultChan := make(chan bool)

	go func() {
		for _, node := range tree.Content {
			traverseTree(Path{}, nil, node, func(path Path, _ *yamlv3.Node, _ *yamlv3.Node) {
				if path.ToGoPatchStyle() == searchPath.ToGoPatchStyle() {
					resultChan <- true
				}
			})

			resultChan <- false
		}
	}()

	return <-resultChan, nil
}

func traverseTree(path Path, parent *yamlv3.Node, node *yamlv3.Node, leafFunc func(path Path, parent *yamlv3.Node, leaf *yamlv3.Node)) {
	switch node.Kind {
	case yamlv3.DocumentNode:
		traverseTree(
			path,
			node,
			node.Content[0],
			leafFunc,
		)

	case yamlv3.SequenceNode:
		if identifier := GetIdentifierFromNamedList(node); identifier != "" {
			for _, mappingNode := range node.Content {
				name, _ := getValueByKey(mappingNode, identifier)
				tmpPath := NewPathWithNamedListElement(path, identifier, name.Value)
				for i := 0; i < len(mappingNode.Content); i += 2 {
					k, v := mappingNode.Content[i], mappingNode.Content[i+1]
					if k.Value == identifier { // skip the identifier mapping entry
						continue
					}

					traverseTree(
						NewPathWithNamedElement(tmpPath, k.Value),
						node,
						v,
						leafFunc,
					)
				}
			}

		} else {
			for idx, entry := range node.Content {
				traverseTree(
					NewPathWithIndexedListElement(path, idx),
					node,
					entry,
					leafFunc,
				)
			}
		}

	case yamlv3.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			k, v := node.Content[i], node.Content[i+1]
			traverseTree(
				NewPathWithNamedElement(path, k.Value),
				node,
				v,
				leafFunc,
			)
		}

	default:
		leafFunc(path, parent, node)
	}
}

// ParseGoPatchStylePathString returns a path by parsing a string representation
// which is assumed to be a GoPatch style path.
func ParseGoPatchStylePathString(path string) (Path, error) {
	// Special case for root path
	if path == "/" {
		return Path{DocumentIdx: 0, PathElements: nil}, nil
	}

	// Hacky solution to deal with escaped slashes, replace them with a "safe"
	// replacement string that is later resolved into a simple slash
	path = strings.Replace(path, `\/`, `%2F`, -1)

	elements := make([]PathElement, 0)
	for i, section := range strings.Split(path, "/") {
		if i == 0 {
			continue
		}

		keyNameSplit := strings.Split(section, "=")
		switch len(keyNameSplit) {
		case 1:
			if idx, err := strconv.Atoi(keyNameSplit[0]); err == nil {
				elements = append(elements, PathElement{
					Idx: idx,
				})

			} else {
				elements = append(elements, PathElement{
					Idx:  -1,
					Name: strings.Replace(keyNameSplit[0], `%2F`, "/", -1),
				})
			}

		case 2:
			elements = append(elements, PathElement{Idx: -1,
				Key:  strings.Replace(keyNameSplit[0], `%2F`, "/", -1),
				Name: strings.Replace(keyNameSplit[1], `%2F`, "/", -1),
			})

		default:
			return Path{}, &InvalidPathString{
				Style:       GoPatchStyle,
				PathString:  path,
				Explanation: fmt.Sprintf("element '%s' cannot contain more than one equal sign", section),
			}
		}
	}

	return Path{DocumentIdx: 0, PathElements: elements}, nil
}

// ParseDotStylePathString returns a path by parsing a string representation
// which is assumed to be a Dot-Style path.
func ParseDotStylePathString(path string, node *yamlv3.Node) (Path, error) {
	if node.Kind != yamlv3.DocumentNode {
		return Path{}, fmt.Errorf("node has to be of kind DocumentNode for parsing a document path")
	}

	elements := make([]PathElement, 0)
	pointer := node.Content[0]

	for _, section := range strings.Split(path, ".") {
		switch {
		case pointer == nil:
			// If the pointer is nil, it means that the previous section of the path
			// string could not be found in the data structure and that all remaining
			// sections are assumed to be of type map.
			elements = append(elements, PathElement{Idx: -1, Name: section})

		case pointer.Kind == yamlv3.MappingNode:
			if value, err := getValueByKey(pointer, section); err == nil {
				pointer = value
				elements = append(elements, PathElement{Idx: -1, Name: section})

			} else {
				pointer = nil
				elements = append(elements, PathElement{Idx: -1, Name: section})
			}

		case pointer.Kind == yamlv3.SequenceNode:
			list := pointer.Content
			if id, err := strconv.Atoi(section); err == nil {
				if id < 0 || id >= len(list) {
					return Path{}, &InvalidPathString{
						Style:       DotStyle,
						PathString:  path,
						Explanation: fmt.Sprintf("provided list index %d is not in range: 0..%d", id, len(list)-1),
					}
				}

				pointer = list[id]
				elements = append(elements, PathElement{Idx: id})

			} else {
				identifier := GetIdentifierFromNamedList(pointer)
				value, ok := getEntryFromNamedList(pointer, identifier, section)
				if !ok {
					names, err := listNamesOfNamedList(pointer, identifier)
					if err != nil {
						return Path{}, &InvalidPathString{
							Style:       DotStyle,
							PathString:  path,
							Explanation: fmt.Sprintf("provided named list entry '%s' cannot be found in list", section),
						}
					}

					return Path{}, &InvalidPathString{
						Style:       DotStyle,
						PathString:  path,
						Explanation: fmt.Sprintf("provided named list entry '%s' cannot be found in list, available names are: %s", section, strings.Join(names, ", ")),
					}
				}

				pointer = value
				elements = append(elements, PathElement{Idx: -1, Key: identifier, Name: section})
			}
		}
	}

	return Path{DocumentIdx: 0, PathElements: elements}, nil
}

// ParseDotStylePathStringUnsafe returns a path by parsing a string
// representation, which is assumed to be a Dot-Style path, but *without*
// checking it against a YAML Node
func ParseDotStylePathStringUnsafe(path string) (Path, error) {
	matches := dotRegEx.FindStringSubmatch(path)
	if matches == nil {
		return Path{}, NewInvalidPathError(GoPatchStyle, path,
			"failed to parse path string, because path does not match expected format",
		)
	}

	var documentIdx int
	if len(matches[2]) > 0 {
		var err error
		documentIdx, err = strconv.Atoi(matches[2])
		if err != nil {
			return Path{}, NewInvalidPathError(GoPatchStyle, path,
				"failed to parse path string, cannot parse document index: %s", matches[2],
			)
		}
	}

	// Reset path variable to only contain the raw path string
	path = matches[3]

	var elements []PathElement
	for _, section := range strings.Split(path, ".") {
		if idx, err := strconv.Atoi(section); err == nil {
			elements = append(elements, PathElement{Idx: idx})

		} else {
			// This is the unsafe part here, since there is no YAML node to
			// check against, it can only be assumed it is a mapping
			elements = append(elements, PathElement{Idx: -1, Name: section})
		}
	}

	return Path{DocumentIdx: documentIdx, PathElements: elements}, nil
}

// ParsePathString returns a path by parsing a string representation
// of a path, which can be one of the supported types.
func ParsePathString(pathString string, node *yamlv3.Node) (Path, error) {
	if strings.HasPrefix(pathString, "/") {
		return ParseGoPatchStylePathString(pathString)
	}

	return ParseDotStylePathString(pathString, node)
}

// ParsePathStringUnsafe returns a path by parsing a string representation of a
// path, which can either be GoPatch or DotStyle, but will not check the path
// elements against a given YAML document to verify the types (unsafe)
func ParsePathStringUnsafe(pathString string) (Path, error) {
	if strings.HasPrefix(pathString, "/") {
		return ParseGoPatchStylePathString(pathString)
	}

	return ParseDotStylePathStringUnsafe(pathString)
}

func (element PathElement) isMapElement() bool {
	return len(element.Key) == 0 &&
		len(element.Name) > 0
}

func (element PathElement) isComplexListElement() bool {
	return len(element.Key) > 0 &&
		len(element.Name) > 0
}

func (element PathElement) isSimpleListElement() bool {
	return len(element.Key) == 0 &&
		len(element.Name) == 0
}
