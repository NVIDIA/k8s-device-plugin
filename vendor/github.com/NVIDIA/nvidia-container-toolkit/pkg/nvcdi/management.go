/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
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

package nvcdi

import (
	"fmt"
	"path/filepath"
	"strings"

	"tags.cncf.io/container-device-interface/pkg/cdi"
	"tags.cncf.io/container-device-interface/specs-go"

	"github.com/NVIDIA/nvidia-container-toolkit/internal/discover"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/edits"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/nvsandboxutils"
)

type managementlib nvcdilib

var _ deviceSpecGeneratorFactory = (*managementlib)(nil)

func (l *managementlib) DeviceSpecGenerators(...string) (DeviceSpecGenerator, error) {
	return l, nil
}

// GetDeviceSpecs returns the CDI device specs for a single all device.
func (m *managementlib) GetDeviceSpecs() ([]specs.Device, error) {
	devices, err := m.newManagementDeviceDiscoverer()
	if err != nil {
		return nil, fmt.Errorf("failed to create device discoverer: %v", err)
	}

	edits, err := edits.FromDiscoverer(devices)
	if err != nil {
		return nil, fmt.Errorf("failed to create edits from discoverer: %v", err)
	}

	if len(edits.DeviceNodes) == 0 {
		return nil, fmt.Errorf("no NVIDIA device nodes found")
	}

	device := specs.Device{
		Name:           "all",
		ContainerEdits: *edits.ContainerEdits,
	}
	return []specs.Device{device}, nil
}

// GetCommonEdits returns the common edits for use in managementlib containers.
func (m *managementlib) GetCommonEdits() (*cdi.ContainerEdits, error) {
	if m.nvsandboxutilslib != nil {
		if r := m.nvsandboxutilslib.Init(m.driverRoot); r != nvsandboxutils.SUCCESS {
			m.logger.Warningf("Failed to init nvsandboxutils: %v; ignoring", r)
			m.nvsandboxutilslib = nil
		}
		defer func() {
			if m.nvsandboxutilslib == nil {
				return
			}
			_ = m.nvsandboxutilslib.Shutdown()
		}()
	}

	driver, err := (*nvcdilib)(m).newDriverVersionDiscoverer()
	if err != nil {
		return nil, fmt.Errorf("failed to create driver library discoverer: %v", err)
	}

	edits, err := edits.FromDiscoverer(driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create edits from discoverer: %v", err)
	}

	return edits, nil
}

type managementDiscoverer struct {
	discover.Discover
}

// newManagementDeviceDiscoverer returns a discover.Discover that discovers device nodes for use in managementlib containers.
// NVML is not used to query devices and all device nodes are returned.
func (m *managementlib) newManagementDeviceDiscoverer() (discover.Discover, error) {
	deviceNodes := discover.NewCharDeviceDiscoverer(
		m.logger,
		m.devRoot,
		[]string{
			"/dev/nvidia*",
			"/dev/nvidia-caps/nvidia-cap*",
			"/dev/nvidia-modeset",
			"/dev/nvidia-uvm-tools",
			"/dev/nvidia-uvm",
			"/dev/nvidiactl",
			"/dev/nvidia-caps-imex-channels/channel*",
		},
	)

	deviceFolderPermissionHooks := newDeviceFolderPermissionHookDiscoverer(
		m.logger,
		m.devRoot,
		m.hookCreator,
		deviceNodes,
	)

	d := discover.Merge(
		&managementDiscoverer{deviceNodes},
		deviceFolderPermissionHooks,
	)
	return d, nil
}

func (m *managementDiscoverer) Devices() ([]discover.Device, error) {
	devices, err := m.Discover.Devices()
	if err != nil {
		return devices, err
	}

	var filteredDevices []discover.Device
	for _, device := range devices {
		if m.nodeIsBlocked(device.HostPath) {
			continue
		}
		filteredDevices = append(filteredDevices, device)
	}

	return filteredDevices, nil
}

// nodeIsBlocked returns true if the specified device node should be ignored.
func (m managementDiscoverer) nodeIsBlocked(path string) bool {
	blockedPrefixes := []string{"nvidia-fs", "nvidia-nvswitch", "nvidia-nvlink"}
	nodeName := filepath.Base(path)
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(nodeName, prefix) {
			return true
		}
	}
	return false
}
