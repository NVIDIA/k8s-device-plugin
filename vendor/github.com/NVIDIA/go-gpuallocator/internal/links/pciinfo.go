/**
# Copyright 2023 NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package links

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// PciInfo is a type alias to nvml.PciInfo to allow for functions to be defined on the type.
type PciInfo nvml.PciInfo

// BusID provides a utility function that returns the string representation of the bus ID.
// Note that the []int8 slice member is named BusId.
func (p PciInfo) BusID() string {
	var bytes []byte
	for _, b := range p.BusId {
		if byte(b) == '\x00' {
			break
		}
		bytes = append(bytes, byte(b))
	}
	id := strings.ToLower(string(bytes))

	if id != "0000" {
		id = strings.TrimPrefix(id, "0000")
	}
	return id
}

// CPUAffinity returns the CPU affinity associated with a specified PCI device.
// If NUMA information is not available, this returns nil.
func (p PciInfo) CPUAffinity() *uint {
	node := p.NumaNode()
	if node < 0 {
		return nil
	}
	affinity := uint(node)
	return &affinity
}

// NumaNode returns the numa node associates with a PCI device.
// If numa is unsupported, -1 is returned.
func (p PciInfo) NumaNode() int64 {
	// Read the numa_node file associated with the PCI Device Info
	b, err := os.ReadFile(fmt.Sprintf("/sys/bus/pci/devices/%s/numa_node", p.BusID()))
	if err != nil {
		return -1
	}
	node, err := strconv.ParseInt(string(bytes.TrimSpace(b)), 10, 64)
	if err != nil {
		return -1
	}
	return node
}
