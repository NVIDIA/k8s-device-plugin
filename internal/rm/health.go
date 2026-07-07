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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"
)

const (
	// envDisableHealthChecks defines the environment variable that is checked to determine whether healthchecks
	// should be disabled. If this envvar is set to "all" or contains the string "xids", healthchecks are
	// disabled entirely. If set, the envvar is treated as a comma-separated list of Xids to ignore. Note that
	// this is in addition to the Application errors that are already ignored.
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	// envEnableHealthChecks defines the environment variable that is checked to
	// determine which XIDs should be explicitly enabled. XIDs specified here
	// override the ones specified in the `DP_DISABLE_HEALTHCHECKS`.
	// Note that this also allows individual XIDs to be selected when ALL XIDs
	// are disabled.
	envEnableHealthChecks = "DP_ENABLE_HEALTHCHECKS"

	// polledHealthCheckInterval defines how frequently the polled health checks
	// run. These checks cover conditions not detectable via NVML events, such as
	// remapped rows, retired pages pending status, and GPU temperature.
	polledHealthCheckInterval = 30 * time.Second
)

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices.
// It combines event-based monitoring (XID errors, ECC errors) with periodic polled checks
// (remapped rows, retired pages, temperature).
func (r *nvmlResourceManager) checkHealth(stop <-chan interface{}, devices Devices, unhealthy chan<- *Device) error {
	xids := getDisabledHealthCheckXids()
	if xids.IsAllDisabled() {
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
			klog.Infof("Error shutting down NVML: %v", ret)
		}
	}()

	klog.Infof("Ignoring the following XIDs for health checks: %v", xids)

	eventSet, ret := r.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to create event set: %v", ret)
	}
	defer func() {
		_ = eventSet.Free()
	}()

	parentToDeviceMap := make(map[string]*Device)
	parentToDevicesMap := make(map[string][]*Device)
	deviceIDToGiMap := make(map[string]uint32)
	deviceIDToCiMap := make(map[string]uint32)

	eventMask := uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)
	for _, d := range devices {
		uuid, gi, ci, err := r.getDevicePlacement(d)
		if err != nil {
			klog.Warningf("Could not determine device placement for %v: %v; Marking it unhealthy.", d.ID, err)
			unhealthy <- d
			continue
		}
		deviceIDToGiMap[d.ID] = gi
		deviceIDToCiMap[d.ID] = ci
		parentToDeviceMap[uuid] = d
		parentToDevicesMap[uuid] = append(parentToDevicesMap[uuid], d)

		gpu, ret := r.nvml.DeviceGetHandleByUUID(uuid)
		if ret != nvml.SUCCESS {
			klog.Infof("unable to get device handle from UUID: %v; marking it as unhealthy", ret)
			unhealthy <- d
			continue
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			klog.Infof("unable to determine the supported events for %v: %v; marking it as unhealthy", d.ID, ret)
			unhealthy <- d
			continue
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, eventSet)
		switch {
		case ret == nvml.ERROR_NOT_SUPPORTED:
			klog.Warningf("Device %v is too old to support healthchecking.", d.ID)
		case ret != nvml.SUCCESS:
			klog.Infof("Marking device %v as unhealthy: %v", d.ID, ret)
			unhealthy <- d
		}
	}

	// Launch polled health checks (remapped rows, retired pages, temperature)
	// in parallel with the event-based health check loop.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.polledHealthChecks(stop, parentToDevicesMap, unhealthy)
	}()

	// Run event-based health check loop.
	for {
		select {
		case <-stop:
			wg.Wait()
			return nil
		default:
		}

		e, ret := eventSet.Wait(5000)
		if ret == nvml.ERROR_TIMEOUT {
			continue
		}
		if ret != nvml.SUCCESS {
			klog.Infof("Error waiting for event: %v; Marking all devices as unhealthy", ret)
			for _, d := range devices {
				unhealthy <- d
			}
			continue
		}

		// Handle double-bit (uncorrectable) ECC errors.
		if e.EventType == nvml.EventTypeDoubleBitEccError {
			eventUUID, ret := e.Device.GetUUID()
			if ret != nvml.SUCCESS {
				klog.Infof("Failed to determine uuid for DoubleBitEccError event: %v; Marking all devices as unhealthy.", ret)
				for _, d := range devices {
					unhealthy <- d
				}
				continue
			}
			klog.Infof("DoubleBitEccError on Device=%s; marking device(s) as unhealthy.", eventUUID)
			for _, d := range parentToDevicesMap[eventUUID] {
				unhealthy <- d
			}
			continue
		}

		// Log single-bit (correctable) ECC errors but do not mark unhealthy.
		if e.EventType == nvml.EventTypeSingleBitEccError {
			eventUUID, ret := e.Device.GetUUID()
			if ret != nvml.SUCCESS {
				klog.Warningf("Failed to determine uuid for SingleBitEccError event: %v", ret)
			} else {
				klog.Warningf("SingleBitEccError on Device=%s (correctable; not marking unhealthy).", eventUUID)
			}
			continue
		}

		if e.EventType != nvml.EventTypeXidCriticalError {
			klog.Infof("Skipping non-critical event: %+v", e)
			continue
		}

		if xids.IsDisabled(e.EventData) {
			klog.Infof("Skipping event %+v", e)
			continue
		}

		klog.Infof("Processing event %+v", e)
		eventUUID, ret := e.Device.GetUUID()
		if ret != nvml.SUCCESS {
			// If we cannot reliably determine the device UUID, we mark all devices as unhealthy.
			klog.Infof("Failed to determine uuid for event %v: %v; Marking all devices as unhealthy.", e, ret)
			for _, d := range devices {
				unhealthy <- d
			}
			continue
		}

		d, exists := parentToDeviceMap[eventUUID]
		if !exists {
			klog.Infof("Ignoring event for unexpected device: %v", eventUUID)
			continue
		}

		if d.IsMigDevice() && e.GpuInstanceId != 0xFFFFFFFF && e.ComputeInstanceId != 0xFFFFFFFF {
			gi := deviceIDToGiMap[d.ID]
			ci := deviceIDToCiMap[d.ID]
			if gi != e.GpuInstanceId || ci != e.ComputeInstanceId {
				continue
			}
			klog.Infof("Event for mig device %v (gi=%v, ci=%v)", d.ID, gi, ci)
		}

		klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking device as unhealthy.", e.EventData, d.ID)
		unhealthy <- d
	}
}

// polledHealthChecks runs periodic health checks that cannot be detected via
// NVML events. These cover hardware conditions such as remapped memory rows,
// pending retired pages, and GPU temperature reaching the shutdown threshold.
func (r *nvmlResourceManager) polledHealthChecks(stop <-chan interface{}, parentToDevicesMap map[string][]*Device, unhealthy chan<- *Device) {
	ticker := time.NewTicker(polledHealthCheckInterval)
	defer ticker.Stop()

	// Track devices already reported unhealthy to avoid duplicate reports.
	reported := make(map[string]bool)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			for uuid, devices := range parentToDevicesMap {
				allReported := true
				for _, d := range devices {
					if !reported[d.ID] {
						allReported = false
						break
					}
				}
				if allReported {
					continue
				}

				gpu, ret := r.nvml.DeviceGetHandleByUUID(uuid)
				if ret != nvml.SUCCESS {
					klog.Warningf("Unable to get device handle for %v during polled health check: %v", uuid, ret)
					continue
				}

				checks := []struct {
					name  string
					check func(nvml.Device) (string, bool)
				}{
					{"RemappedRows", r.checkRemappedRows},
					{"RetiredPages", r.checkRetiredPages},
					{"Temperature", r.checkTemperature},
				}

				for _, hc := range checks {
					reason, failed := hc.check(gpu)
					if !failed {
						continue
					}
					klog.Infof("%s health check failed for %v: %s; marking device(s) as unhealthy.", hc.name, uuid, reason)
					for _, d := range devices {
						if !reported[d.ID] {
							reported[d.ID] = true
							unhealthy <- d
						}
					}
					break
				}
			}
		}
	}
}

// checkRemappedRows checks whether the GPU has experienced a row remapping
// failure or has a pending row remap that requires a GPU reset.
// See: https://docs.nvidia.com/deploy/a100-gpu-mem-error-mgmt/index.html
func (r *nvmlResourceManager) checkRemappedRows(gpu nvml.Device) (string, bool) {
	_, uncRows, isPending, failureOccurred, ret := gpu.GetRemappedRows()
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return "", false
	}
	if ret != nvml.SUCCESS {
		klog.Warningf("Failed to get remapped rows: %v", ret)
		return "", false
	}

	if failureOccurred {
		return "row remapping failure occurred (uncorrectable memory error)", true
	}
	if isPending {
		return "row remapping is pending (GPU reset required)", true
	}
	if uncRows > 0 {
		klog.Warningf("GPU has %d uncorrectable remapped row(s); rows were successfully remapped", uncRows)
	}
	return "", false
}

// checkRetiredPages checks whether the GPU has pages pending retirement.
// Pending page retirements indicate that the GPU requires a reboot to complete
// the retirement of faulty memory pages.
func (r *nvmlResourceManager) checkRetiredPages(gpu nvml.Device) (string, bool) {
	status, ret := gpu.GetRetiredPagesPendingStatus()
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return "", false
	}
	if ret != nvml.SUCCESS {
		klog.Warningf("Failed to get retired pages pending status: %v", ret)
		return "", false
	}

	if status == nvml.FEATURE_ENABLED {
		return "pages are pending retirement (reboot required)", true
	}
	return "", false
}

// checkTemperature checks whether the GPU temperature has reached or exceeded
// the hardware shutdown threshold. A GPU at this temperature will be shut down
// by the hardware to prevent damage. If the temperature has reached the
// slowdown threshold, a warning is logged but the device is not marked unhealthy.
func (r *nvmlResourceManager) checkTemperature(gpu nvml.Device) (string, bool) {
	shutdownTemp, ret := gpu.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_SHUTDOWN)
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return "", false
	}
	if ret != nvml.SUCCESS {
		klog.Warningf("Failed to get shutdown temperature threshold: %v", ret)
		return "", false
	}

	currentTemp, ret := gpu.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return "", false
	}
	if ret != nvml.SUCCESS {
		klog.Warningf("Failed to get current GPU temperature: %v", ret)
		return "", false
	}

	if currentTemp >= shutdownTemp {
		return fmt.Sprintf("GPU temperature (%d°C) has reached shutdown threshold (%d°C)", currentTemp, shutdownTemp), true
	}

	slowdownTemp, ret := gpu.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_SLOWDOWN)
	if ret == nvml.SUCCESS && currentTemp >= slowdownTemp {
		klog.Warningf("GPU temperature (%d°C) has reached slowdown threshold (%d°C); GPU is thermally throttling", currentTemp, slowdownTemp)
	}

	return "", false
}

const allXIDs = 0

// disabledXIDs stores a map of explicitly disabled XIDs.
// The special XID `allXIDs` indicates that all XIDs are disabled, but does
// allow for specific XIDs to be enabled even if this is the case.
type disabledXIDs map[uint64]bool

// Disabled returns whether XID-based health checks are disabled.
// These are considered if all XIDs have been disabled AND no other XIDs have
// been explcitly enabled.
func (h disabledXIDs) IsAllDisabled() bool {
	if allDisabled, ok := h[allXIDs]; ok {
		return allDisabled
	}
	// At this point we wither have explicitly disabled XIDs or explicitly
	// enabled XIDs. Since ANY XID that's not specified is assumed enabled, we
	// return here.
	return false
}

// IsDisabled checks whether the specified XID has been explicitly disalbled.
// An XID is considered disabled if it has been explicitly disabled, or all XIDs
// have been disabled.
func (h disabledXIDs) IsDisabled(xid uint64) bool {
	// Handle the case where enabled=all.
	if explicitAll, ok := h[allXIDs]; ok && !explicitAll {
		return false
	}
	// Handle the case where the XID has been specifically enabled (or disabled)
	if disabled, ok := h[xid]; ok {
		return disabled
	}
	return h.IsAllDisabled()
}

// getDisabledHealthCheckXids returns the XIDs that should be ignored.
// Here we combine the following (in order of precedence):
// * A list of explicitly disabled XIDs (including all XIDs)
// * A list of hardcoded disabled XIDs
// * A list of explicitly enabled XIDs (including all XIDs)
//
// Note that if an XID is explicitly enabled, this takes precedence over it
// having been disabled either explicitly or implicitly.
func getDisabledHealthCheckXids() disabledXIDs {
	disabled := newHealthCheckXIDs(
		// TODO: We should not read the envvar here directly, but instead
		// "upgrade" this to a top-level config option.
		strings.Split(strings.ToLower(os.Getenv(envDisableHealthChecks)), ",")...,
	)
	enabled := newHealthCheckXIDs(
		// TODO: We should not read the envvar here directly, but instead
		// "upgrade" this to a top-level config option.
		strings.Split(strings.ToLower(os.Getenv(envEnableHealthChecks)), ",")...,
	)

	// Add the list of hardcoded disabled (ignored) XIDs:
	// FIXME: formalize the full list and document it.
	// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
	// Application errors: the GPU should still be healthy
	ignoredXids := []uint64{
		13,  // Graphics Engine Exception
		31,  // GPU memory page fault
		43,  // GPU stopped processing
		45,  // Preemptive cleanup, due to previous errors
		68,  // Video processor exception
		109, // Context Switch Timeout Error
	}
	for _, ignored := range ignoredXids {
		disabled[ignored] = true
	}

	// Explicitly ENABLE specific XIDs,
	for enabled := range enabled {
		disabled[enabled] = false
	}
	return disabled
}

// newHealthCheckXIDs converts a list of Xids to a healthCheckXIDs map.
// Special xid values 'all' and 'xids' return a special map that matches all
// xids.
// For other xids, these are converted to a uint64 values with invalid values
// being ignored.
func newHealthCheckXIDs(xids ...string) disabledXIDs {
	output := make(disabledXIDs)
	for _, xid := range xids {
		trimmed := strings.TrimSpace(xid)
		if trimmed == "all" || trimmed == "xids" {
			// TODO: We should have a different type for "all" and "all-except"
			return disabledXIDs{allXIDs: true}
		}
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			klog.Infof("Ignoring malformed Xid value %v: %v", trimmed, err)
			continue
		}

		output[id] = true
	}
	return output
}

// getDevicePlacement returns the placement of the specified device.
// For a MIG device the placement is defined by the 3-tuple <parent UUID, GI, CI>
// For a full device the returned 3-tuple is the device's uuid and 0xFFFFFFFF for the other two elements.
func (r *nvmlResourceManager) getDevicePlacement(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return d.GetUUID(), 0xFFFFFFFF, 0xFFFFFFFF, nil
	}
	return r.getMigDeviceParts(d)
}

// getMigDeviceParts returns the parent GI and CI ids of the MIG device.
func (r *nvmlResourceManager) getMigDeviceParts(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return "", 0, 0, fmt.Errorf("cannot get GI and CI of full device")
	}

	uuid := d.GetUUID()
	// For older driver versions, the call to DeviceGetHandleByUUID will fail for MIG devices.
	mig, ret := r.nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		parentHandle, ret := mig.GetDeviceHandleFromMigDeviceHandle()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent device handle: %v", ret)
		}

		parentUUID, ret := parentHandle.GetUUID()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent uuid: %v", ret)
		}
		gi, ret := mig.GetGpuInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get GPU Instance ID: %v", ret)
		}

		ci, ret := mig.GetComputeInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get Compute Instance ID: %v", ret)
		}
		//nolint:gosec  // We know that the values returned from Get*InstanceId are within the valid uint32 range.
		return parentUUID, uint32(gi), uint32(ci), nil
	}
	return parseMigDeviceUUID(uuid)
}

// parseMigDeviceUUID splits the MIG device UUID into the parent device UUID and ci and gi
func parseMigDeviceUUID(mig string) (string, uint32, uint32, error) {
	tokens := strings.SplitN(mig, "-", 2)
	if len(tokens) != 2 || tokens[0] != "MIG" {
		return "", 0, 0, fmt.Errorf("unable to parse UUID as MIG device")
	}

	tokens = strings.SplitN(tokens[1], "/", 3)
	if len(tokens) != 3 || !strings.HasPrefix(tokens[0], "GPU-") {
		return "", 0, 0, fmt.Errorf("unable to parse UUID as MIG device")
	}

	gi, err := toUint32(tokens[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("unable to parse UUID as MIG device")
	}

	ci, err := toUint32(tokens[2])
	if err != nil {
		return "", 0, 0, fmt.Errorf("unable to parse UUID as MIG device")
	}

	return tokens[0], gi, ci, nil
}

func toUint32(s string) (uint32, error) {
	u, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	//nolint:gosec  // Since we parse s with a 32-bit size this will not overflow.
	return uint32(u), nil
}
