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

func TestGetVendorSpecificCapabilityMalformedLength(t *testing.T) {
	// A truncated config (or a device exposing a malformed vendor-specific
	// capability record whose length field makes start+length exceed the
	// buffer) must not panic. It should be skipped so a single bad device
	// can't crash gpu-feature-discovery.
	config := make([]byte, 256)
	config[PciStatusByte] |= PciStatusCapabilityList
	// Point the capability list at offset 224 and mark it vendor-specific.
	config[PciCapabilityList] = 224
	config[224+PciCapabilityListID] = PciCapabilityVendorSpecificID
	config[224+PciCapabilityListNext] = 0
	config[224+PciCapabilityLength] = 40
	// 224 + 40 == 264 which is past the 256-byte buffer.

	device := &PCIDevice{Address: "malformed", Config: config}
	capability, err := device.GetVendorSpecificCapability()
	require.NoError(t, err, "malformed capability length must not panic")
	require.Nil(t, capability, "malformed capability record should be skipped")
}
