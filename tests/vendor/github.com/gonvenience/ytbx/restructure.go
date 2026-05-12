// Copyright © 2019 The Homeport Team
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
	"sort"

	yamlv3 "gopkg.in/yaml.v3"
)

// DisableRemainingKeySort disables that during restructuring of map keys, all
// unknown keys are also sorted in such a way that it improves the readability.
var DisableRemainingKeySort = false

var knownKeyOrders = [][]string{
	{"name", "director_uuid", "releases", "instance_groups", "networks", "resource_pools", "compilation"},
	{"name", "url", "version", "sha1"},

	// Concourse (https://concourse-ci.org/pipelines.html, https://concourse-ci.org/steps.html, https://concourse-ci.org/resources.html)
	{"jobs", "resources", "resource_types"},
	{"name", "type", "source"},
	{"get"},
	{"put"},
	{"task"},

	// SUSE SCF role manifest (https://github.com/SUSE/scf/blob/develop/container-host-files/etc/scf/config/role-manifest.yml)
	{"releases", "instance_groups", "configuration", "variables"},
	{"auth", "templates"},

	// Universal default #1 ... name should always be first
	{"name"},

	// Universal default #2 ... key should always be first
	{"key"},

	// Universal default #3 ... id should always be first
	{"id"},
}

func lookupMap(list []string) map[string]int {
	result := make(map[string]int, len(list))
	for idx, entry := range list {
		result[entry] = idx
	}

	return result
}

func lookupMapOfContentList(list []*yamlv3.Node) map[string]int {
	lookup := make(map[string]int, len(list))
	for i := 0; i < len(list); i += 2 {
		lookup[list[i].Value] = i
	}

	return lookup
}

func maxDepth(node *yamlv3.Node) (max int) {
	rootPath, _ := ParseGoPatchStylePathString("/")
	traverseTree(
		rootPath,
		nil,
		node,
		func(p Path, _ *yamlv3.Node, _ *yamlv3.Node) {
			if depth := len(p.PathElements); depth > max {
				max = depth
			}
		},
	)

	return max
}

func countCommonKeys(keys []string, list []string) (counter int) {
	lookup := lookupMap(keys)
	for _, key := range list {
		if _, ok := lookup[key]; ok {
			counter++
		}
	}

	return
}

func commonKeys(setA []string, setB []string) []string {
	result, lookup := []string{}, lookupMap(setB)
	for _, entry := range setA {
		if _, ok := lookup[entry]; ok {
			result = append(result, entry)
		}
	}

	return result
}

func reorderKeyValuePairsInMappingNodeContent(mappingNode *yamlv3.Node, keys []string) {
	// Create list with all keys, that are not part of the provided list of keys
	remainingKeys, keysLookup := []string{}, lookupMap(keys)
	for i := 0; i < len(mappingNode.Content); i += 2 {
		key := mappingNode.Content[i].Value
		if _, ok := keysLookup[key]; !ok {
			remainingKeys = append(remainingKeys, key)
		}
	}

	// Sort remaining keys by sorting long and possibly hard to read structure
	// to the end of the mapping
	if !DisableRemainingKeySort {
		sort.Slice(remainingKeys, func(i, j int) bool {
			valI, _ := getValueByKey(mappingNode, remainingKeys[i])
			valJ, _ := getValueByKey(mappingNode, remainingKeys[j])
			return maxDepth(valI) < maxDepth(valJ)
		})
	}

	// Rebuild a new YAML Node list (content) key by key by using first the keys
	// from the reorder list and then all remaining keys
	content, contentLookup := []*yamlv3.Node{}, lookupMapOfContentList(mappingNode.Content)
	for _, key := range append(keys, remainingKeys...) {
		idx := contentLookup[key]
		content = append(content,
			mappingNode.Content[idx],
			mappingNode.Content[idx+1],
		)
	}

	mappingNode.Content = content
}

func getSuitableReorderFunction(keys []string) func(*yamlv3.Node) {
	topCandidateIdx, topCandidateHits := -1, -1
	for idx, candidate := range knownKeyOrders {
		if count := countCommonKeys(keys, candidate); count > 0 && count > topCandidateHits {
			topCandidateIdx = idx
			topCandidateHits = count
		}
	}

	if topCandidateIdx >= 0 {
		return func(input *yamlv3.Node) {
			reorderKeyValuePairsInMappingNodeContent(
				input,
				commonKeys(knownKeyOrders[topCandidateIdx], keys),
			)
		}
	}

	return nil
}

// RestructureObject takes an object and traverses down any sub elements such as
// list entries or map values to recursively call restructure itself. On YAML
// MappingNodes, it will use a look-up mechanism to decide if the order of key
// in that map need to be rearranged to meet some known established human order.
func RestructureObject(node *yamlv3.Node) {
	switch node.Kind {
	case yamlv3.DocumentNode:
		RestructureObject(node.Content[0])

	case yamlv3.MappingNode:
		keys := listKeys(node)
		if fn := getSuitableReorderFunction(keys); fn != nil {
			fn(node)
		}

		// Restructure the values of the respective keys of this YAML MapSlice
		for i := 0; i < len(node.Content); i += 2 {
			RestructureObject(node.Content[i+1])
		}

	case yamlv3.SequenceNode:
		for i := range node.Content {
			RestructureObject(node.Content[i])
		}
	}
}
