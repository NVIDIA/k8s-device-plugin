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
	"fmt"
	"strconv"
	"strings"

	"github.com/NVIDIA/gpu-feature-discovery/internal/resource"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// NewNVMLLabeler creates a new NVML-based labeler using the provided NVML library and config.
func NewNVMLLabeler(manager resource.Manager, config *spec.Config) (Labeler, error) {
	if err := manager.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize NVML: %v", err)
	}
	defer manager.Shutdown()

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

	resourceLabeler, err := NewResourceLabeler(manager, config)
	if err != nil {
		return nil, fmt.Errorf("error creating resource labeler: %v", err)
	}

	l := Merge(
		machineTypeLabeler,
		versionLabeler,
		migCapabilityLabeler,
		resourceLabeler,
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
		"nvidia.com/cuda.driver.major":  driverMajor,
		"nvidia.com/cuda.driver.minor":  driverMinor,
		"nvidia.com/cuda.driver.rev":    driverRev,
		"nvidia.com/cuda.runtime.major": fmt.Sprintf("%d", *cudaMajor),
		"nvidia.com/cuda.runtime.minor": fmt.Sprintf("%d", *cudaMinor),
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
