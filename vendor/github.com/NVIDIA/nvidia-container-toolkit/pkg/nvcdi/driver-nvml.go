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

	"github.com/NVIDIA/nvidia-container-toolkit/internal/discover"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/lookup"
	"github.com/NVIDIA/nvidia-container-toolkit/internal/lookup/cuda"
	"github.com/sirupsen/logrus"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

// NewDriverDiscoverer creates a discoverer for the libraries and binaries associated with a driver installation.
// The supplied NVML Library is used to query the expected driver version.
func NewDriverDiscoverer(logger *logrus.Logger, driverRoot string, nvidiaCTKPath string, nvmllib nvml.Interface) (discover.Discover, error) {
	if r := nvmllib.Init(); r != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initalize NVML: %v", r)
	}
	defer nvmllib.Shutdown()

	version, r := nvmllib.SystemGetDriverVersion()
	if r != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to determine driver version: %v", r)
	}

	return newDriverVersionDiscoverer(logger, driverRoot, nvidiaCTKPath, version)
}

func newDriverVersionDiscoverer(logger *logrus.Logger, driverRoot string, nvidiaCTKPath string, version string) (discover.Discover, error) {
	libraries, err := NewDriverLibraryDiscoverer(logger, driverRoot, nvidiaCTKPath, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create discoverer for driver libraries: %v", err)
	}

	ipcs, err := discover.NewIPCDiscoverer(logger, driverRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create discoverer for IPC sockets: %v", err)
	}

	firmwares := NewDriverFirmwareDiscoverer(logger, driverRoot, version)

	binaries := NewDriverBinariesDiscoverer(logger, driverRoot)

	d := discover.Merge(
		libraries,
		ipcs,
		firmwares,
		binaries,
	)

	return d, nil
}

// NewDriverLibraryDiscoverer creates a discoverer for the libraries associated with the specified driver version.
func NewDriverLibraryDiscoverer(logger *logrus.Logger, driverRoot string, nvidiaCTKPath string, version string) (discover.Discover, error) {
	libraryPaths, err := getVersionLibs(logger, driverRoot, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get libraries for driver version: %v", err)
	}

	libraries := discover.NewMounts(
		logger,
		lookup.NewFileLocator(
			lookup.WithLogger(logger),
			lookup.WithRoot(driverRoot),
		),
		driverRoot,
		libraryPaths,
	)

	cfg := &discover.Config{
		DriverRoot:    driverRoot,
		NvidiaCTKPath: nvidiaCTKPath,
	}
	hooks, _ := discover.NewLDCacheUpdateHook(logger, libraries, cfg)

	d := discover.Merge(
		libraries,
		hooks,
	)

	return d, nil
}

// NewDriverFirmwareDiscoverer creates a discoverer for GSP firmware associated with the specified driver version.
func NewDriverFirmwareDiscoverer(logger *logrus.Logger, driverRoot string, version string) discover.Discover {
	gspFirmwarePath := filepath.Join("/lib/firmware/nvidia", version, "gsp*.bin")
	return discover.NewMounts(
		logger,
		lookup.NewFileLocator(
			lookup.WithLogger(logger),
			lookup.WithRoot(driverRoot),
		),
		driverRoot,
		[]string{gspFirmwarePath},
	)
}

// NewDriverBinariesDiscoverer creates a discoverer for GSP firmware associated with the GPU driver.
func NewDriverBinariesDiscoverer(logger *logrus.Logger, driverRoot string) discover.Discover {
	return discover.NewMounts(
		logger,
		lookup.NewExecutableLocator(logger, driverRoot),
		driverRoot,
		[]string{
			"nvidia-smi",              /* System management interface */
			"nvidia-debugdump",        /* GPU coredump utility */
			"nvidia-persistenced",     /* Persistence mode utility */
			"nvidia-cuda-mps-control", /* Multi process service CLI */
			"nvidia-cuda-mps-server",  /* Multi process service server */
		},
	)
}

// getVersionLibs checks the LDCache for libraries ending in the specified driver version.
// Although the ldcache at the specified driverRoot is queried, the paths are returned relative to this driverRoot.
// This allows the standard mount location logic to be used for resolving the mounts.
func getVersionLibs(logger *logrus.Logger, driverRoot string, version string) ([]string, error) {
	logger.Infof("Using driver version %v", version)

	libCudaPaths, err := cuda.New(
		cuda.WithLogger(logger),
		cuda.WithDriverRoot(driverRoot),
	).Locate("." + version)
	if err != nil {
		return nil, fmt.Errorf("failed to locate libcuda.so.%v: %v", version, err)
	}
	libRoot := filepath.Dir(libCudaPaths[0])

	libraries := lookup.NewFileLocator(
		lookup.WithLogger(logger),
		lookup.WithSearchPaths(libRoot),
		lookup.WithOptional(true),
	)

	libs, err := libraries.Locate("*.so." + version)
	if err != nil {
		return nil, fmt.Errorf("failed to locate libraries for driver version %v: %v", version, err)
	}

	if driverRoot == "/" || driverRoot == "" {
		return libs, nil
	}

	var relative []string
	for _, l := range libs {
		relative = append(relative, strings.TrimPrefix(l, driverRoot))
	}

	return relative, nil
}
