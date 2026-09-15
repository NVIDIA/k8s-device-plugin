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
	"testing"

	"github.com/stretchr/testify/require"
)

// MockVGPU represents mock of VGPU interface
type MockVGPU struct {
	devices []*Device
}

// Devices returns VGPU devices with mocked data
func (p *MockVGPU) Devices() ([]*Device, error) {
	return p.devices, nil
}

func TestIsVGPUDevice(t *testing.T) {
	mockVGPU := NewMockVGPU().(*Lib)
	devices, _ := mockVGPU.pci.Devices()
	for _, device := range devices {
		// check for vendor id
		require.Equal(t, "0x10de", fmt.Sprintf("0x%x", GetWord(device.Config, 0)), "Nvidia PCI Vendor ID")
		// check for vendor capability records
		capability, err := device.GetVendorSpecificCapability()
		require.NoError(t, err, "Get vendor capabilities from configuration space")
		require.NotZero(t, len(capability), "Vendor capability record")
		if device.Address == "passthrough" {
			require.False(t, mockVGPU.IsVGPUDevice(capability), "Is not a virtual GPU device")
			require.Equal(t, 20, len(capability), "Vendor capability length for passthrough device")
		}
		if device.Address == "vgpu" {
			require.Equal(t, 27, len(capability), "Vendor capability length for vgpu device")
			require.Equal(t, uint8(9), GetByte(capability, 0), "Vendor capability ID")
		}
	}
}

func TestVGPUGetInfo(t *testing.T) {
	devices, _ := NewMockVGPU().Devices()
	for _, device := range devices {
		if device.pci.Address == "vgpu" {
			require.NotEmpty(t, device.pci.Config, "Device Configuration data")
			require.Equal(t, len(device.pci.Config), 256, "Device configuration data length")

			require.NotEmpty(t, device.vGPUCapability, "Vendor capability record")
			require.Equal(t, device.vGPUCapability[0], uint8(9), "Vendor capability id")

			info, err := device.GetInfo()
			require.NoError(t, err, "Get host driver version and branch")
			require.NotNil(t, info, "Host driver info")
			require.Equal(t, "460.16", info.HostDriverVersion, "Host driver version")
			require.Equal(t, "r460_00", info.HostDriverBranch, "Host driver branch")
		}
	}
}

func TestVGPUGetInfoRejectsMalformedRecords(t *testing.T) {
	tests := []struct {
		name       string
		capability []byte
		err        string
	}{
		{
			name:       "missing record",
			capability: []byte{0x09, 0x00, 0x05, 0x56, 0x46},
			err:        "cannot find driver version record",
		},
		{
			name:       "truncated record header",
			capability: []byte{0x09, 0x00, 0x06, 0x56, 0x46, 0x01},
			err:        "truncated vendor capability record",
		},
		{
			name:       "zero-length record",
			capability: []byte{0x09, 0x00, 0x07, 0x56, 0x46, 0x01, 0x00},
			err:        "invalid vendor capability record length 0",
		},
		{
			name:       "record shorter than header",
			capability: []byte{0x09, 0x00, 0x07, 0x56, 0x46, 0x01, 0x01},
			err:        "invalid vendor capability record length 1",
		},
		{
			name:       "record exceeds capability",
			capability: []byte{0x09, 0x00, 0x07, 0x56, 0x46, 0x01, 0xff},
			err:        "vendor capability record exceeds capability bounds",
		},
		{
			name:       "truncated driver record header",
			capability: []byte{0x09, 0x00, 0x06, 0x56, 0x46, 0x00},
			err:        "truncated driver version record",
		},
		{
			name:       "driver record shorter than payload",
			capability: []byte{0x09, 0x00, 0x07, 0x56, 0x46, 0x00, 0x02},
			err:        "invalid driver version record length 2",
		},
		{
			name:       "driver record exceeds capability",
			capability: []byte{0x09, 0x00, 0x07, 0x56, 0x46, 0x00, 0x16},
			err:        "driver version record exceeds capability bounds",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			device := &Device{
				pci:            &PCIDevice{Address: "malformed"},
				vGPUCapability: tc.capability,
			}

			info, err := device.GetInfo()

			require.ErrorContains(t, err, tc.err)
			require.Nil(t, info)
		})
	}
}

func TestVGPUGetInfoSkipsUnknownRecords(t *testing.T) {
	capability := make([]byte, 30)
	copy(capability, []byte{0x09, 0x00, 0x1e, 0x56, 0x46})
	copy(capability[5:], []byte{0x01, 0x03, 0xff})
	copy(capability[8:], []byte{0x00, 0x16})
	copy(capability[10:], []byte("460.16"))
	copy(capability[20:], []byte("r460_00"))

	device := &Device{
		pci:            &PCIDevice{Address: "vgpu"},
		vGPUCapability: capability,
	}

	info, err := device.GetInfo()

	require.NoError(t, err)
	require.Equal(t, "460.16", info.HostDriverVersion)
	require.Equal(t, "r460_00", info.HostDriverBranch)
}
