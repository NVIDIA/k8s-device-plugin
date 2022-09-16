/*
 * Copyright (c) 2019-2022, NVIDIA CORPORATION.  All rights reserved.
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

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvml"
)

const (
	// envDisableHealthChecks defines the environment variable that is checked to determine whether healthchecks
	// should be disabled. If this envvar is set to "all" or contains the string "xids", healthchecks are
	// disabled entirely. If set, the envvar is treated as a comma-separated list of Xids to ignore. Note that
	// this is in addition to the Application errors that are already ignored.
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	allHealthChecks        = "xids"

	// maxSuccessiveEventErrorCount sets the number of errors waiting for events before marking all devices as unhealthy.
	maxSuccessiveEventErrorCount = 3
)

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *resourceManager) checkHealth(stop <-chan interface{}, devices Devices, unhealthy chan<- *Device) error {
	disableHealthChecks := strings.ToLower(os.Getenv(envDisableHealthChecks))
	if disableHealthChecks == "all" {
		disableHealthChecks = allHealthChecks
	}
	if strings.Contains(disableHealthChecks, "xids") {
		return nil
	}

	ret := r.nvml.Init()
	if ret != nvml.SUCCESS {
		if *r.config.Flags.FailOnInitError {
			return fmt.Errorf("failed to initialize NVML: %v", ret)
		}
		return nil
	}
	defer func() {
		ret := r.nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Printf("Error shutting down NVML: %v", ret)
		}
	}()

	// FIXME: formalize the full list and document it.
	// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
	// Application errors: the GPU should still be healthy
	applicationErrorXids := []uint64{
		13, // Graphics Engine Exception
		31, // GPU memory page fault
		43, // GPU stopped processing
		45, // Preemptive cleanup, due to previous errors
		68, // Video processor exception
	}

	skippedXids := make(map[uint64]bool)
	for _, id := range applicationErrorXids {
		skippedXids[id] = true
	}

	for _, additionalXid := range getAdditionalXids(disableHealthChecks) {
		skippedXids[additionalXid] = true
	}

	eventSet, ret := r.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to create event set: %v", ret)
	}
	defer eventSet.Free()

	eventMask := uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)
	for _, d := range devices {
		uuid, _, _, err := r.getMigDevicePartsByUUID(d)
		if err != nil {
			log.Printf("Warning: could not determine parent device for %v: %v; Marking it unhealthy.", d.ID, err)
			unhealthy <- d
			continue
		}

		gpu, ret := r.nvml.DeviceGetHandleByUUID(uuid)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to get device handle from UUID: %v", ret)
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unabled to determine the supported events for %v: %v", d.ID, ret)
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, eventSet)
		if ret == nvml.ERROR_NOT_SUPPORTED {
			log.Printf("Warning: %s is too old to support healthchecking: %s. Marking it unhealthy.", d.ID, err)
			unhealthy <- d
			continue
		}
		if ret != nvml.SUCCESS {
			return fmt.Errorf("unable to register events for health checking on GPU %v: %v", d.ID, err)
		}
	}

	successiveEventErrorCount := 0
	for {
		select {
		case <-stop:
			return nil
		default:
		}

		e, ret := eventSet.Wait(5000)
		if ret == nvml.ERROR_TIMEOUT {
			continue
		}
		if ret != nvml.SUCCESS {
			// TODO: I think they may actually be an error state from the start. Not sure if we need successive errors here.
			successiveEventErrorCount++
			log.Printf("Error waiting for event (%d of %d): %v", successiveEventErrorCount, maxSuccessiveEventErrorCount, ret)
			if successiveEventErrorCount >= maxSuccessiveEventErrorCount {
				log.Printf("Marking all devices as unhealthy")
				for _, d := range devices {
					unhealthy <- d
				}
			}
			continue
		}

		successiveEventErrorCount = 0
		if e.EventType != nvml.EventTypeXidCriticalError {
			log.Printf("Skipping non-nvmlEventTypeXidCriticalError event: %+v", e)
			continue
		}

		if skippedXids[e.EventData] {
			log.Printf("Skipping event %+v", e)
			continue
		}

		log.Printf("Processing event %+v", e)
		eventUUID, ret := e.Device.GetUUID()
		if ret != nvml.SUCCESS {
			// If we cannot reliably determine the device UUID, we mark all devices as unhealthy.
			log.Printf("Failed to determine uuid for event %v: %v; Marking all devices as unhealthy.", e, ret)
			for _, d := range devices {
				unhealthy <- d
			}
			continue
		}

		for _, d := range devices {
			// Please see https://github.com/NVIDIA/gpu-monitoring-tools/blob/148415f505c96052cb3b7fdf443b34ac853139ec/bindings/go/nvml/nvml.h#L1424
			// for the rationale why gi and ci can be set as such when the UUID is a full GPU UUID and not a MIG device UUID.
			uuid, gi, ci, err := r.getMigDevicePartsByUUID(d)
			if err != nil {
				log.Printf("Failed to get device parts device %v; marking device unhealthy", d.ID)
				unhealthy <- d
				continue
			}
			if uuid == eventUUID && gi == uint(e.GpuInstanceId) && ci == uint(e.ComputeInstanceId) {
				log.Printf("XidCriticalError: Xid=%d on Device=%s, the device will go unhealthy.", e.EventData, d.ID)
				unhealthy <- d
			}
		}
	}
}

// getAdditionalXids returns a list of additional Xids to skip from the specified string.
// The input is treaded as a comma-separated string and all valid uint64 values are considered as Xid values. Invalid values
// are ignored.
func getAdditionalXids(input string) []uint64 {
	if input == "" {
		return nil
	}

	var additionalXids []uint64
	for _, additionalXid := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(additionalXid)
		if trimmed == "" {
			continue
		}
		xid, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			log.Printf("Ignoring malformed Xid value %v: %v", trimmed, err)
			continue
		}
		additionalXids = append(additionalXids, xid)
	}

	return additionalXids
}

// getMigDevicePartsByUUID returns the parent GPU UUID and GI and CI ids of the MIG device.
func (r *resourceManager) getMigDevicePartsByUUID(d *Device) (string, uint, uint, error) {
	uuid := d.ID
	if !d.IsMigDevice() {
		return uuid, 0xFFFFFFFF, 0xFFFFFFFF, nil
	}
	// For older driver versions, the call to DeviceGetHandleByUUID will fail for MIG devices.
	migHandle, ret := r.nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		return getMIGDeviceInfo(migHandle)
	}
	return parseMigDeviceUUID(uuid)
}

// getMIGDeviceInfo returns the parent ID, gi, and ci for the specified device
func getMIGDeviceInfo(mig nvml.Device) (string, uint, uint, error) {
	parentHandle, ret := mig.GetDeviceHandleFromMigDeviceHandle()
	if ret != nvml.SUCCESS {
		return "", 0, 0, ret
	}

	parentUUID, ret := parentHandle.GetUUID()
	if ret != nvml.SUCCESS {
		return "", 0, 0, ret
	}

	gi, ret := mig.GetGpuInstanceId()
	if ret != nvml.SUCCESS {
		return "", 0, 0, ret
	}

	ci, ret := mig.GetComputeInstanceId()
	if ret != nvml.SUCCESS {
		return "", 0, 0, ret
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
