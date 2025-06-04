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
	"fmt"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/containerd/nri/pkg/api"
	"k8s.io/klog/v2"
)

// DeviceDiscoverer defines the interface for discovering assigned devices
type DeviceDiscoverer interface {
	// GetAssignedDevices returns the list of GPU and MIG device UUIDs assigned to a container
	GetAssignedDevices(c *api.Container) ([]string, error)
	// GetAllDevices returns all GPU and MIG devices in the system
	GetAllDevices() (map[string]*Device, error)
}

// Device represents a GPU or MIG device with cached UUID and index
type Device struct {
	nvml.Device
	uuid  string
	index string
}

// deviceDiscoverer discovers devices based on environment variables
type deviceDiscoverer struct{}

// NewDeviceDiscoverer creates a new device discoverer
func NewDeviceDiscoverer() DeviceDiscoverer {
	return &deviceDiscoverer{}
}

// GetAssignedDevices implements DeviceDiscoverer interface
func (d *deviceDiscoverer) GetAssignedDevices(c *api.Container) ([]string, error) {
	// Look for NVIDIA_VISIBLE_DEVICES environment variable
	for _, env := range c.Env {
		if strings.HasPrefix(env, "NVIDIA_VISIBLE_DEVICES=") {
			// Get the value after the equals sign
			value := strings.TrimPrefix(env, "NVIDIA_VISIBLE_DEVICES=")

			switch value {
			case "", "void":
				return nil, nil
			case "all":
				uuids, err := d.getAllDeviceUUIDs()
				if err != nil {
					return nil, fmt.Errorf("failed to get UUIDs from all devices: %w", err)
				}
				return uuids, nil
			default:
				ids := strings.Split(value, ",")
				uuids, err := d.normalizeToDeviceUUIDs(ids)
				if err != nil {
					return nil, fmt.Errorf("failed to process device IDs as UUID or Index: %w", err)
				}
				return uuids, nil
			}
		}
	}
	return nil, nil
}

// GetAllDevices implements DeviceDiscoverer interface
func (d *deviceDiscoverer) GetAllDevices() (map[string]*Device, error) {
	// Initialize NVML
	nvmlLib := nvml.New()
	if ret := nvmlLib.Init(); ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initialize NVML: %w", ret)
	}
	defer func() {
		if ret := nvmlLib.Shutdown(); ret != nvml.SUCCESS {
			klog.Warning("Failed to shutdown NVML", "error", ret)
		}
	}()

	// Create the nvlib device interface
	nvlib := device.New(nvmlLib)

	devices, err := nvlib.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get devices from nvlib: %w", err)
	}

	allDevices := make(map[string]*Device)
	for i, dev := range devices {
		// Add the GPU device
		uuid, ret := dev.GetUUID()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get GPU device UUID: %v", ret)
		}
		allDevices[uuid] = &Device{Device: dev, uuid: uuid, index: strconv.Itoa(i)}

		// Add MIG devices
		migs, err := dev.GetMigDevices()
		if err != nil {
			return nil, fmt.Errorf("failed to get MIG devices: %w", err)
		}
		for j, mig := range migs {
			// Convert MIG device to NVML device
			uuid, ret := mig.GetUUID()
			if ret != nvml.SUCCESS {
				return nil, fmt.Errorf("failed to get MIG device UUID: %v", ret)
			}
			migDevice, ret := nvmlLib.DeviceGetHandleByUUID(uuid)
			if ret != nvml.SUCCESS {
				return nil, fmt.Errorf("failed to get MIG device handle: %v", ret)
			}
			allDevices[uuid] = &Device{Device: migDevice, uuid: uuid, index: fmt.Sprintf("%d:%d", i, j)}
		}
	}
	return allDevices, nil
}

// getAllDeviceUUIDs returns a list of UUIDs from all devices
func (d *deviceDiscoverer) getAllDeviceUUIDs() ([]string, error) {
	devices, err := d.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get all devices: %w", err)
	}
	var uuids []string
	for uuid := range devices {
		uuids = append(uuids, uuid)
	}
	return uuids, nil
}

// normalizeToDeviceUUIDs processes a comma-separated list of UUIDs or indices
func (d *deviceDiscoverer) normalizeToDeviceUUIDs(ids []string) ([]string, error) {
	var uuids []string
	devices, err := d.GetAllDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to get all devices: %w", err)
	}

	for _, id := range ids {
		// Check if the ID is a UUID or an index
		if strings.Contains(id, ":") {
			// Convert MIG index to UUID
			for _, device := range devices {
				if device.index == id {
					uuids = append(uuids, device.uuid)
					break
				}
			}
		} else if _, err := strconv.Atoi(id); err == nil {
			// Convert GPU index to UUID
			for _, device := range devices {
				if device.index == id {
					uuids = append(uuids, device.uuid)
					break
				}
			}
		} else {
			// Assume it's a UUID
			uuids = append(uuids, id)
		}
	}
	return uuids, nil
}
