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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/NVIDIA/k8s-device-plugin/internal/mig"
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

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		if *r.config.Flags.FailOnInitError {
			return fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		}
		return nil
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Printf("Error shutting down NVML: %v", nvml.ErrorString(ret))
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

	eventSet := nvmlNewEventSet()
	defer nvmlDeleteEventSet(eventSet)

	for _, d := range devices {
		gpu, _, _, err := mig.GetMigDevicePartsByUUID(d.ID)
		if err != nil {
			gpu = d.ID
		}

		err = nvmlRegisterEventForDevice(eventSet, nvmlXidCriticalError, gpu)
		if err != nil && strings.HasSuffix(err.Error(), "Not Supported") {
			log.Printf("Warning: %s is too old to support healthchecking: %s. Marking it unhealthy.", d.ID, err)
			unhealthy <- d
			continue
		}
		if err != nil {
			return fmt.Errorf("unable to register events for health checking on GPU %v: %v", gpu, err)
		}
	}

	successiveEventErrorCount := 0
	for {
		select {
		case <-stop:
			return nil
		default:
		}

		e, err := nvmlWaitForEvent(eventSet, 5000)
		if err != nil && err.Error() != "Timeout" {
			successiveEventErrorCount += 1
			log.Printf("Error waiting for event (%d of %d): %v", successiveEventErrorCount, maxSuccessiveEventErrorCount, err)
			if successiveEventErrorCount >= maxSuccessiveEventErrorCount {
				log.Printf("Marking all devices as unhealthy")
				for _, d := range devices {
					unhealthy <- d
				}
			}
			continue
		}

		successiveEventErrorCount = 0
		if e.Etype != nvmlXidCriticalError {
			if e.Etype == nvmlDoubleBitEccError || e.Etype == nvmlSingleBitEccError {
				log.Printf("Skipping non-nvmlEventTypeXidCriticalError event: %+v", e)
			}
			continue
		}

		if skippedXids[e.Edata] {
			log.Printf("Skipping event %+v", e)
			continue
		}

		log.Printf("Processing event %+v", e)
		if e.UUID == nil || len(*e.UUID) == 0 {
			// All devices are unhealthy
			log.Printf("XidCriticalError: Xid=%d, All devices will go unhealthy.", e.Edata)
			for _, d := range devices {
				unhealthy <- d
			}
			continue
		}

		for _, d := range devices {
			// Please see https://github.com/NVIDIA/gpu-monitoring-tools/blob/148415f505c96052cb3b7fdf443b34ac853139ec/bindings/go/nvml/nvml.h#L1424
			// for the rationale why gi and ci can be set as such when the UUID is a full GPU UUID and not a MIG device UUID.
			gpu, gi, ci, err := mig.GetMigDevicePartsByUUID(d.ID)
			if err != nil {
				gpu = d.ID
				gi = 0xFFFFFFFF
				ci = 0xFFFFFFFF
			}

			if gpu == *e.UUID && gi == *e.GpuInstanceID && ci == *e.ComputeInstanceID {
				log.Printf("XidCriticalError: Xid=%d on Device=%s, the device will go unhealthy.", e.Edata, d.ID)
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
