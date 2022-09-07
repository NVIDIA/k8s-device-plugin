// Copyright (c) 2021 - 2022, NVIDIA CORPORATION. All rights reserved.

package mig

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

const (
	nvidiaProcDriverPath   = "/proc/driver/nvidia"
	nvidiaCapabilitiesPath = nvidiaProcDriverPath + "/capabilities"

	nvcapsProcDriverPath = "/proc/driver/nvidia-caps"
	nvcapsMigMinorsPath  = nvcapsProcDriverPath + "/mig-minors"
	nvcapsDevicePath     = "/dev/nvidia-caps"
)

// GetMigDevicePartsByUUID returns the parent GPU UUID and GI and CI ids of the MIG device.
func GetMigDevicePartsByUUID(uuid string) (string, uint, uint, error) {
	// For older driver versions, the call to DeviceGetHandleByUUID will fail for MIG devices.
	migHandle, ret := nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		return getMIGDeviceInfo(migHandle)
	}
	return parseMigDeviceUUID(uuid)
}

// GetMigCapabilityDevicePaths returns a mapping of MIG capability path to device node path
func GetMigCapabilityDevicePaths() (map[string]string, error) {
	// Open nvcapsMigMinorsPath for walking.
	// If the nvcapsMigMinorsPath does not exist, then we are not on a MIG
	// capable machine, so there is nothing to do.
	// The format of this file is discussed in:
	//     https://docs.nvidia.com/datacenter/tesla/mig-user-guide/index.html#unique_1576522674
	minorsFile, err := os.Open(nvcapsMigMinorsPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error opening MIG minors file: %v", err)
	}
	defer minorsFile.Close()

	// Define a function to process each each line of nvcapsMigMinorsPath
	processLine := func(line string) (string, int, error) {
		var gpu, gi, ci, migMinor int

		// Look for a CI access file
		n, _ := fmt.Sscanf(line, "gpu%d/gi%d/ci%d/access %d", &gpu, &gi, &ci, &migMinor)
		if n == 4 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/ci%d/access", gpu, gi, ci)
			return capPath, migMinor, nil
		}

		// Look for a GI access file
		n, _ = fmt.Sscanf(line, "gpu%d/gi%d/access %d", &gpu, &gi, &migMinor)
		if n == 3 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/access", gpu, gi)
			return capPath, migMinor, nil
		}

		// Look for the MIG config file
		n, _ = fmt.Sscanf(line, "config %d", &migMinor)
		if n == 1 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath + "/mig/config")
			return capPath, migMinor, nil
		}

		// Look for the MIG monitor file
		n, _ = fmt.Sscanf(line, "monitor %d", &migMinor)
		if n == 1 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath + "/mig/monitor")
			return capPath, migMinor, nil
		}

		return "", 0, fmt.Errorf("unparsable line: %v", line)
	}

	// Walk each line of nvcapsMigMinorsPath and construct a mapping of nvidia
	// capabilities path to device minor for that capability
	capsDevicePaths := make(map[string]string)
	scanner := bufio.NewScanner(minorsFile)
	for scanner.Scan() {
		capPath, migMinor, err := processLine(scanner.Text())
		if err != nil {
			log.Printf("Skipping line in MIG minors file: %v", err)
			continue
		}
		capsDevicePaths[capPath] = fmt.Sprintf(nvcapsDevicePath+"/nvidia-cap%d", migMinor)
	}
	return capsDevicePaths, nil
}

// getMIGDeviceInfo returns the parent ID, gi, and ci for the specified device
func getMIGDeviceInfo(mig nvml.Device) (string, uint, uint, error) {
	parentHandle, ret := mig.GetDeviceHandleFromMigDeviceHandle()
	if ret != nvml.SUCCESS {
		return "", 0, 0, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	parentUUID, ret := parentHandle.GetUUID()
	if ret != nvml.SUCCESS {
		return "", 0, 0, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	gi, ret := mig.GetGpuInstanceId()
	if ret != nvml.SUCCESS {
		return "", 0, 0, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	ci, ret := mig.GetComputeInstanceId()
	if ret != nvml.SUCCESS {
		return "", 0, 0, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	return parentUUID, uint(gi), uint(ci), nil
}

// parseMigDeviceUUID splits the MIG device UUID into the parent device UUID and ci and gi
func parseMigDeviceUUID(mig string) (string, uint, uint, error) {
	tokens := strings.SplitN(mig, "-", 2)
	if len(tokens) != 2 || tokens[0] != "MIG" {
		return "", 0, 0, fmt.Errorf("Unable to parse UUID as MIG device")
	}

	tokens = strings.SplitN(tokens[1], "/", 3)
	if len(tokens) != 3 || !strings.HasPrefix(tokens[0], "GPU-") {
		return "", 0, 0, fmt.Errorf("Unable to parse UUID as MIG device")
	}

	gi, err := strconv.Atoi(tokens[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("Unable to parse UUID as MIG device")
	}

	ci, err := strconv.Atoi(tokens[2])
	if err != nil {
		return "", 0, 0, fmt.Errorf("Unable to parse UUID as MIG device")
	}

	return tokens[0], uint(gi), uint(ci), nil
}
