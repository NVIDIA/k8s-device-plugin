/**
# Copyright (c) 2021-2022, NVIDIA CORPORATION.  All rights reserved.
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

package vgpu

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNvidiaPCILibDevicesWithCustomRoot(t *testing.T) {
	root := t.TempDir()
	busID := "0000:03:00.0"
	deviceDir := filepath.Join(root, "bus", "pci", "devices", busID)
	require.NoError(t, os.MkdirAll(deviceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "vendor"), []byte("0x10de\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "class"), []byte("0x030000\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "config"), make([]byte, 256), 0o644))

	lib := NewNvidiaPCILib(root)
	devices, err := lib.Devices()
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, busID, devices[0].Address)
}

func TestNvidiaPCILibDevicesMissingRoot(t *testing.T) {
	lib := NewNvidiaPCILib("/nonexistent-sysfs-root")
	_, err := lib.Devices()
	require.Error(t, err)
}

func TestGetVendorSpecificCapability(t *testing.T) {
	devices, _ := NewMockNvidiaPCI().Devices()
	for _, device := range devices {
		// check for vendor id
		require.Equal(t, "0x10de", fmt.Sprintf("0x%x", GetWord(device.Config, 0)), "Nvidia PCI Vendor ID")
		// check for vendor specific capability
		capability, err := device.GetVendorSpecificCapability()
		require.NoError(t, err, "Get vendor specific capability from configuration space")
		require.NotZero(t, len(capability), "Vendor capability record")
		if device.Address == "passthrough" {
			require.Equal(t, 20, len(capability), "Vendor capability length for passthrough device")
		}
		if device.Address == "vgpu" {
			require.Equal(t, 27, len(capability), "Vendor capability length for vgpu device")
		}
	}
}
