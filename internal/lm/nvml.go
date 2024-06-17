/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

package lm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/resource"
)

var errMPSSharingNotSupported = errors.New("MPS sharing is not supported")

// NewDeviceLabeler creates a new labeler for the specified resource manager.
func NewDeviceLabeler(manager resource.Manager, config *spec.Config) (Labeler, error) {
	if err := manager.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize resource manager: %v", err)
	}
	defer func() {
		_ = manager.Shutdown()
	}()

	devices, err := manager.GetDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	if len(devices) == 0 {
		return empty{}, nil
	}

	machineTypeLabeler, err := newMachineTypeLabeler(*config.Flags.GFD.MachineTypeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to construct machine type labeler: %v", err)
	}

	versionLabeler, err := newVersionLabeler(manager)
	if err != nil {
		return nil, fmt.Errorf("failed to construct version labeler: %v", err)
	}

	migCapabilityLabeler, err := newMigCapabilityLabeler(manager)
	if err != nil {
		return nil, fmt.Errorf("error creating mig capability labeler: %v", err)
	}

	sharingLabeler, err := newSharingLabeler(manager, config)
	if err != nil {
		return nil, fmt.Errorf("error creating sharing labeler: %w", err)
	}

	resourceLabeler, err := NewResourceLabeler(manager, config)
	if err != nil {
		return nil, fmt.Errorf("error creating resource labeler: %v", err)
	}

	gpuModeLabeler, err := newGPUModeLabeler(manager)
	if err != nil {
		return nil, fmt.Errorf("error creating resource labeler: %v", err)
	}

	l := Merge(
		machineTypeLabeler,
		versionLabeler,
		migCapabilityLabeler,
		sharingLabeler,
		resourceLabeler,
		gpuModeLabeler,
	)

	return l, nil
}

// newVersionLabeler creates a labeler that generates the CUDA and driver version labels.
func newVersionLabeler(manager resource.Manager) (Labeler, error) {
	driverVersion, err := manager.GetDriverVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting driver version: %v", err)
	}

	driverVersionSplit := strings.Split(driverVersion, ".")
	if len(driverVersionSplit) > 3 || len(driverVersionSplit) < 2 {
		return nil, fmt.Errorf("error getting driver version: Version \"%s\" does not match format \"X.Y[.Z]\"", driverVersion)
	}

	driverMajor := driverVersionSplit[0]
	driverMinor := driverVersionSplit[1]
	driverRev := ""
	if len(driverVersionSplit) > 2 {
		driverRev = driverVersionSplit[2]
	}

	cudaMajor, cudaMinor, err := manager.GetCudaDriverVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting cuda driver version: %v", err)
	}

	labels := Labels{
		// Deprecated labels
		"nvidia.com/cuda.driver.major":  driverMajor,
		"nvidia.com/cuda.driver.minor":  driverMinor,
		"nvidia.com/cuda.driver.rev":    driverRev,
		"nvidia.com/cuda.runtime.major": fmt.Sprintf("%d", *cudaMajor),
		"nvidia.com/cuda.runtime.minor": fmt.Sprintf("%d", *cudaMinor),

		// New labels
		"nvidia.com/cuda.driver-version.major":    driverMajor,
		"nvidia.com/cuda.driver-version.minor":    driverMinor,
		"nvidia.com/cuda.driver-version.revision": driverRev,
		"nvidia.com/cuda.driver-version.full":     driverVersion,
		"nvidia.com/cuda.runtime-version.major":   fmt.Sprintf("%d", *cudaMajor),
		"nvidia.com/cuda.runtime-version.minor":   fmt.Sprintf("%d", *cudaMinor),
		"nvidia.com/cuda.runtime-version.full":    fmt.Sprintf("%d.%d", *cudaMajor, *cudaMinor),
	}
	return labels, nil
}

// newMigCapabilityLabeler creates a new MIG capability labeler using the provided NVML library.
// If any GPU on the node is mig-capable the label is set to true.
func newMigCapabilityLabeler(manager resource.Manager) (Labeler, error) {
	isMigCapable := false

	devices, err := manager.GetDevices()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		// no devices, return empty labels
		return empty{}, nil
	}

	// loop through all devices to check if any one of them is MIG capable
	for _, d := range devices {
		isMigCapable, err = d.IsMigCapable()
		if err != nil {
			return nil, fmt.Errorf("error getting mig capability: %v", err)
		}
		if isMigCapable {
			break
		}
	}

	labels := Labels{
		"nvidia.com/mig.capable": strconv.FormatBool(isMigCapable),
	}
	return labels, nil
}

func newSharingLabeler(manager resource.Manager, config *spec.Config) (Labeler, error) {
	if config == nil || config.Sharing.SharingStrategy() != spec.SharingStrategyMPS {
		labels := Labels{
			"nvidia.com/mps.capable": "false",
		}
		return labels, nil
	}

	capable, err := isMPSCapable(manager)
	if err != nil {
		return nil, fmt.Errorf("failed to check MPS-capable: %w", err)
	}

	labels := Labels{
		"nvidia.com/mps.capable": strconv.FormatBool(capable),
	}
	return labels, nil
}

func isMPSCapable(manager resource.Manager) (bool, error) {
	devices, err := manager.GetDevices()
	if err != nil {
		return false, fmt.Errorf("failed to get device: %w", err)
	}

	for _, d := range devices {
		isMigEnabled, err := d.IsMigEnabled()
		if err != nil {
			return false, fmt.Errorf("failed to check if device is MIG-enabled: %w", err)
		}
		if isMigEnabled {
			return false, fmt.Errorf("%w for mig devices", errMPSSharingNotSupported)
		}
	}
	return true, nil
}

func newGPUModeLabeler(manager resource.Manager) (Labeler, error) {

	devices, err := manager.GetDevices()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		// no devices, return empty labels
		return empty{}, nil
	}

	class := ""
	// loop through all devices to check if all of them are on same gpu mode
	for _, d := range devices {
		val, err := d.GetClass()
		if err != nil {
			return nil, err
		}
		if class != "" && val != class {
			break
		} else {
			class = val
		}
	}

	gpuMode := ""
	switch class {
	case "0x030000":
		gpuMode = "graphics"
	case "0x030200":
		gpuMode = "compute"
	default:
		gpuMode = "unknown"
	}
	labels := Labels{
		"nvidia.com/gpu.mode": gpuMode,
	}
	return labels, nil
}
