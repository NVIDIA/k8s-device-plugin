/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package cdi

import (
	"fmt"
	"path/filepath"

	"github.com/NVIDIA/nvidia-container-toolkit/pkg/nvcdi"
	"github.com/NVIDIA/nvidia-container-toolkit/pkg/nvcdi/transform"
	cdiapi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	cdispec "github.com/container-orchestrated-devices/container-device-interface/specs-go"
	"github.com/sirupsen/logrus"
	nvdevice "gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/device"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

const (
	cdiRoot = "/var/run/cdi"
)

// cdiHandler creates CDI specs for devices assocatied with the device plugin
type cdiHandler struct {
	logger           *logrus.Logger
	nvml             nvml.Interface
	nvdevice         nvdevice.Interface
	driverRoot       string
	targetDriverRoot string
	nvidiaCTKPath    string
	cdiRoot          string
	cdilib           nvcdi.Interface
	vendor           string
	class            string
	deviceIDStrategy string
}

var _ Interface = &cdiHandler{}

// newHandler constructs a new instance of the 'cdi' interface
func newHandler(opts ...Option) (Interface, error) {
	c := &cdiHandler{}
	for _, opt := range opts {
		opt(c)
	}
	if c.logger == nil {
		c.logger = logrus.StandardLogger()
	}
	if c.nvml == nil {
		c.nvml = nvml.New()
	}
	if c.nvdevice == nil {
		c.nvdevice = nvdevice.New(nvdevice.WithNvml(c.nvml))
	}
	if c.deviceIDStrategy == "" {
		c.deviceIDStrategy = "uuid"
	}
	if c.driverRoot == "" {
		c.driverRoot = "/"
	}
	if c.targetDriverRoot == "" {
		c.targetDriverRoot = c.driverRoot
	}

	deviceNamer, err := nvcdi.NewDeviceNamer(c.deviceIDStrategy)
	if err != nil {
		return nil, err
	}

	c.cdilib = nvcdi.New(
		nvcdi.WithLogger(c.logger),
		nvcdi.WithNvmlLib(c.nvml),
		nvcdi.WithDeviceLib(c.nvdevice),
		nvcdi.WithNVIDIACTKPath(c.nvidiaCTKPath),
		nvcdi.WithDriverRoot(c.driverRoot),
		nvcdi.WithDeviceNamer(deviceNamer),
	)

	return c, nil
}

// CreateSpecFile creates a CDI spec file for the specified devices.
func (cdi *cdiHandler) CreateSpecFile() error {
	cdi.logger.Infof("Generating CDI spec for resource: %s/%s", cdi.vendor, cdi.class)

	ret := cdi.nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", ret)
	}
	defer cdi.nvml.Shutdown()

	deviceSpecs, err := cdi.cdilib.GetAllDeviceSpecs()
	if err != nil {
		return fmt.Errorf("failed to get CDI device specs: %v", err)
	}

	edits, err := cdi.cdilib.GetCommonEdits()
	if err != nil {
		return fmt.Errorf("failed to get common CDI spec edits: %v", err)
	}

	spec := &cdispec.Spec{
		Kind:           cdi.vendor + "/" + cdi.class,
		Devices:        deviceSpecs,
		ContainerEdits: *edits.ContainerEdits,
	}

	minVersion, err := cdiapi.MinimumRequiredVersion(spec)
	if err != nil {
		return fmt.Errorf("failed to get minimum required CDI spec version: %v", err)
	}
	cdi.logger.Infof("Using minimum required CDI spec version: %s", minVersion)
	spec.Version = minVersion

	if cdi.driverRoot != cdi.targetDriverRoot {
		err = transform.NewDriverRootTransform(cdi.driverRoot, cdi.targetDriverRoot).Apply(spec)
		if err != nil {
			return fmt.Errorf("failed to transform spec: %v", err)
		}
	}

	specName, err := cdiapi.GenerateNameForSpec(spec)
	if err != nil {
		return fmt.Errorf("failed to generate spec name: %v", err)
	}

	return (*cdiSpec)(spec).write(filepath.Join(cdiRoot, specName+".json"))
}

// QualifiedName constructs a CDI qualified device name for the specified resources.
// Note: This assumes that the specified id matches the device name returned by the naming strategy.
func (cdi *cdiHandler) QualifiedName(id string) string {
	return cdiapi.QualifiedName(cdi.vendor, cdi.class, id)
}
