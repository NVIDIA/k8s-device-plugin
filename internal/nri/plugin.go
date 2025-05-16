/*
 * Copyright (c) 2025 NVIDIA CORPORATION.  All rights reserved.
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

package nri

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/containerd/nri/pkg/api"
	nriplugin "github.com/containerd/nri/pkg/stub"
	"golang.org/x/exp/slices"
	"k8s.io/klog/v2"
	"k8s.io/utils/cpuset"
)

// Compile-time interface checks
var (
	_ nriplugin.Plugin = (*NUMAFilterPlugin)(nil)
)

const (
	// NUMAFilterPluginName is the name of the NRI NUMA filter plugin
	NUMAFilterPluginName = "nvidia-numa-filter"
)

// NUMAFilterPlugin implements the NRI plugin interface for NUMA-aware container placement.
// It maintains the system's NUMA topology and ensures containers are placed on the
// correct NUMA nodes based on their assigned GPUs and MIG devices.
type NUMAFilterPlugin struct {
	// cache represents the system-wide NUMA topology
	cache *numaTopologyCache
	// stub represents the NRI plugin stub
	stub nriplugin.Stub
	// deviceDiscoverer is used to discover assigned devices
	deviceDiscoverer DeviceDiscoverer
}

// numaTopologyCache represents the system-wide NUMA topology
type numaTopologyCache struct {
	// All NUMA nodes in the system
	systemNodes []int
	// Map of device UUID (GPU or MIG) to NUMA node
	deviceNodes map[string]int
}

// NewNUMAFilterPlugin creates a new NRI plugin for NUMA filtering
func NewNUMAFilterPlugin() *NUMAFilterPlugin {
	return &NUMAFilterPlugin{
		deviceDiscoverer: NewDeviceDiscoverer(),
	}
}

// Start starts the NRI plugin
func (p *NUMAFilterPlugin) Start(ctx context.Context) (rerr error) {
	// Get all system NUMA nodes
	systemNodes, err := p.getAllSystemNUMANodes()
	if err != nil {
		return fmt.Errorf("failed to get system NUMA nodes: %w", err)
	}
	klog.V(4).Info("Discovered system NUMA nodes", "nodes", systemNodes)

	// Get all device NUMA nodes
	deviceNodes, err := p.getAllDeviceNUMANodes()
	if err != nil {
		return fmt.Errorf("failed to get device NUMA nodes: %w", err)
	}
	klog.V(4).Info("Discovered device NUMA nodes", "deviceCount", len(deviceNodes))

	p.cache = &numaTopologyCache{
		systemNodes: systemNodes,
		deviceNodes: deviceNodes,
	}

	// Create the plugin stub with options
	opts := []nriplugin.Option{
		nriplugin.WithPluginName(NUMAFilterPluginName),
		nriplugin.WithPluginIdx("00"), // Use index 00 to run before other plugins
	}

	s, err := nriplugin.New(p, opts...)
	if err != nil {
		return fmt.Errorf("failed to create plugin stub: %w", err)
	}
	p.stub = s

	// Start the plugin
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	return nil
}

// Stop stops the NRI plugin
func (p *NUMAFilterPlugin) Stop() {
	if p != nil && p.stub != nil {
		p.stub.Stop()
	}
}

// CreateContainer is called when a new container is being created. It ensures the container
// is placed on the correct NUMA nodes based on its assigned GPUs and MIG devices.
func (p *NUMAFilterPlugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	nodes, err := p.filterContainerNUMANodes(container)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get NUMA nodes: %w", err)
	}

	var adjust *api.ContainerAdjustment
	if len(nodes) > 0 {
		adjust = &api.ContainerAdjustment{
			Linux: &api.LinuxContainerAdjustment{
				Resources: &api.LinuxResources{
					Cpu: &api.LinuxCPU{
						Mems: strings.Join(nodes, ","),
					},
				},
			},
		}
	}

	return adjust, nil, nil
}

// UpdateContainer is called when a container's resources are being updated. It ensures the
// container remains on the correct NUMA nodes based on its assigned GPUs and MIG devices.
func (p *NUMAFilterPlugin) UpdateContainer(ctx context.Context, pod *api.PodSandbox, container *api.Container, resources *api.LinuxResources) ([]*api.ContainerUpdate, error) {
	nodes, err := p.filterContainerNUMANodes(container)
	if err != nil {
		return nil, fmt.Errorf("failed to get NUMA nodes: %w", err)
	}

	var updates []*api.ContainerUpdate
	if len(nodes) > 0 {
		update := &api.ContainerUpdate{
			Linux: &api.LinuxContainerUpdate{
				Resources: &api.LinuxResources{
					Cpu: &api.LinuxCPU{
						Mems: strings.Join(nodes, ","),
					},
				},
			},
		}
		updates = append(updates, update)
	}

	return updates, nil
}

// filterContainerNUMANodes filters and returns the NUMA nodes for a container based on its
// existing CPU NUMA nodes and assigned GPU/MIG devices. It ensures that:
// 1. Only valid system NUMA nodes are included from the container's existing nodes
// 2. All NUMA nodes from assigned GPU/MIG devices are included
func (p *NUMAFilterPlugin) filterContainerNUMANodes(c *api.Container) ([]string, error) {
	if p.cache == nil {
		return nil, fmt.Errorf("NUMA topology not initialized")
	}

	// Start with existing NUMA nodes from container, but only include valid system nodes
	var numaNodes []int
	if c.Linux != nil && c.Linux.Resources != nil && c.Linux.Resources.Cpu != nil {
		mems := c.Linux.Resources.Cpu.Mems
		// Parse NUMA nodes using k8s.io/utils/cpuset
		nodes, err := cpuset.Parse(mems)
		if err != nil {
			return nil, fmt.Errorf("failed to parse NUMA nodes from mems %q: %w", mems, err)
		}
		// Only include nodes that exist in the system
		for _, node := range nodes.List() {
			if slices.Contains(p.cache.systemNodes, node) {
				numaNodes = append(numaNodes, node)
			}
		}
	}

	// If no existing NUMA nodes were found, initialize with all system nodes
	if len(numaNodes) == 0 {
		numaNodes = append(numaNodes, p.cache.systemNodes...)
	}

	// Get assigned GPUs and MIG devices
	uuids, err := p.deviceDiscoverer.GetAssignedDevices(c)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned devices: %w", err)
	}

	// Add NUMA nodes for each UUID
	deviceNodes, err := p.getDeviceNUMANodes(uuids)
	if err != nil {
		return nil, fmt.Errorf("failed to get NUMA nodes for devices: %w", err)
	}
	numaNodes = append(numaNodes, deviceNodes...)

	// Convert to strings
	nodeStrs := make([]string, len(numaNodes))
	for i, node := range numaNodes {
		nodeStrs[i] = strconv.Itoa(node)
	}

	return nodeStrs, nil
}

// getAllSystemNUMANodes gets all NUMA nodes in the system that have CPUs assigned to them
func (p *NUMAFilterPlugin) getAllSystemNUMANodes() ([]int, error) {
	// Read /sys/devices/system/node/node* to get NUMA nodes
	nodes, err := filepath.Glob("/sys/devices/system/node/node*")
	if err != nil {
		return nil, fmt.Errorf("failed to list NUMA nodes: %w", err)
	}

	systemNodes := make([]int, 0, len(nodes))
	for _, node := range nodes {
		nodeID := filepath.Base(node)[4:] // Remove "node" prefix
		// Check if the node has any CPUs assigned
		cpulist, err := os.ReadFile(filepath.Join(node, "cpulist"))
		if err != nil {
			return nil, fmt.Errorf("failed to read CPU list for NUMA node %s: %w", nodeID, err)
		}

		// Only include nodes with non-empty CPU lists
		if len(strings.TrimSpace(string(cpulist))) > 0 {
			nodeInt, err := strconv.Atoi(nodeID)
			if err != nil {
				return nil, fmt.Errorf("failed to parse NUMA node ID %s: %w", nodeID, err)
			}
			systemNodes = append(systemNodes, nodeInt)
		}
	}
	return systemNodes, nil
}

// getAllDeviceNUMANodes gets the NUMA nodes for all GPUs and MIG devices in the system
func (p *NUMAFilterPlugin) getAllDeviceNUMANodes() (map[string]int, error) {
	deviceNodes := make(map[string]int)
	devices, err := p.deviceDiscoverer.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	for uuid, device := range devices {
		node, ret := device.GetNumaNodeId()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get NUMA node for device UUID %s: %v", uuid, ret)
		}
		deviceNodes[uuid] = node
		klog.V(4).Info("Added device to NUMA topology", "deviceUUID", uuid, "numaNode", node)
	}
	return deviceNodes, nil
}

// getDeviceNUMANodes gets the NUMA nodes for a list of devices by UUID, first checking the cache
// and then looking them up if not found.
func (p *NUMAFilterPlugin) getDeviceNUMANodes(uuids []string) ([]int, error) {
	var nodes []int
	var missingUUIDs []string

	// Check the cache first
	for _, uuid := range uuids {
		if node, exists := p.cache.deviceNodes[uuid]; exists {
			nodes = append(nodes, node)
		} else {
			missingUUIDs = append(missingUUIDs, uuid)
		}
	}

	// If all UUIDs are in the cache, return the nodes
	if len(missingUUIDs) == 0 {
		return nodes, nil
	}

	// Otherwise, get the missing devices
	devices, err := p.deviceDiscoverer.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	for _, uuid := range missingUUIDs {
		device, exists := devices[uuid]
		if !exists {
			return nil, fmt.Errorf("device with UUID %s not found", uuid)
		}

		// Get NUMA node of device
		node, ret := device.GetNumaNodeId()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get NUMA node for device %s: %v", uuid, ret)
		}
		// Cache the result
		p.cache.deviceNodes[uuid] = node
		nodes = append(nodes, node)
	}
	return nodes, nil
}
