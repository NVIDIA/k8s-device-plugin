/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY Type, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

type wslAllGPUsDevice struct{}

var _ deviceInfo = (*wslAllGPUsDevice)(nil)

// GetUUID returns "all" to represent all GPUs accessible via /dev/dxg on WSL.
func (d wslAllGPUsDevice) GetUUID() (string, error) {
	return "all", nil
}

// GetPaths returns the WSL GPU device path.
func (d wslAllGPUsDevice) GetPaths() ([]string, error) {
	return []string{"/dev/dxg"}, nil
}

// GetNumaNode returns no NUMA node association for WSL devices.
func (d wslAllGPUsDevice) GetNumaNode() (bool, int, error) {
	return false, 0, nil
}

// GetTotalMemory returns 0 as memory info is not available for WSL devices.
func (d wslAllGPUsDevice) GetTotalMemory() (uint64, error) {
	return 0, nil
}

// GetComputeCapability returns an empty string as compute capability is not available for WSL devices.
func (d wslAllGPUsDevice) GetComputeCapability() (string, error) {
	return "", nil
}
