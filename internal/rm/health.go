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
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
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
)

// eventResult packages an NVML event with its return code for passing
// between the event receiver goroutine and the main processing loop.
type eventResult struct {
	event nvml.EventData
	ret   nvml.Return
}

// sendUnhealthyDevice sends a device to the unhealthy channel without
// blocking. If the channel is full, it logs an error and updates the device
// state directly. This prevents the health check goroutine from being blocked
// indefinitely if ListAndWatch is stalled.
func sendUnhealthyDevice(unhealthy chan<- *Device, d *Device) {
	select {
	case unhealthy <- d:
		klog.V(2).Infof("Device %s sent to unhealthy channel", d.ID)
	default:
		// Channel is full - this indicates ListAndWatch is not consuming
		// or the channel buffer is insufficient for the event rate
		klog.Errorf("Health channel full (capacity=%d)! "+
			"Unable to report device %s as unhealthy. "+
			"ListAndWatch may be stalled or event rate is too high.",
			cap(unhealthy), d.ID)
		// Update device state directly as fallback
		d.Health = pluginapi.Unhealthy
	}
}

// healthCheckStats tracks statistics about health check operations for
// observability and debugging.
type healthCheckStats struct {
	startTime              time.Time
	eventsProcessed        uint64
	devicesMarkedUnhealthy uint64
	errorCount             uint64
	xidByType              map[uint64]uint64 // XID code -> count
	mu                     sync.Mutex
}

// recordEvent increments the events processed counter and tracks XID
// distribution.
func (s *healthCheckStats) recordEvent(xid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventsProcessed++
	if s.xidByType == nil {
		s.xidByType = make(map[uint64]uint64)
	}
	s.xidByType[xid]++
}

// recordUnhealthy increments the devices marked unhealthy counter.
func (s *healthCheckStats) recordUnhealthy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devicesMarkedUnhealthy++
}

// recordError increments the error counter.
func (s *healthCheckStats) recordError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount++
}

// report logs a summary of health check statistics.
func (s *healthCheckStats) report() {
	s.mu.Lock()
	defer s.mu.Unlock()

	uptime := time.Since(s.startTime)
	klog.Infof("HealthCheck Stats: uptime=%v, events=%d, unhealthy=%d, errors=%d",
		uptime.Round(time.Second), s.eventsProcessed,
		s.devicesMarkedUnhealthy, s.errorCount)

	if len(s.xidByType) > 0 {
		klog.Infof("HealthCheck XID distribution: %v", s.xidByType)
	}
}

// nvmlHealthProvider encapsulates the state and logic for NVML-based GPU
// health monitoring. This struct groups related data and provides focused
// methods for device registration and event monitoring.
type nvmlHealthProvider struct {
	// Configuration
	nvmllib nvml.Interface
	devices Devices

	// Device placement maps (for MIG support)
	parentToDeviceMap map[string]*Device
	deviceIDToGiMap   map[string]uint32
	deviceIDToCiMap   map[string]uint32

	// XID filtering
	xidsDisabled disabledXIDs

	// Communication
	unhealthy chan<- *Device

	// Observability
	stats *healthCheckStats
}

// registerDeviceEvents registers NVML event handlers for all devices in the
// provider. Devices that fail registration are sent to the unhealthy channel.
// This method is separated for testability and clarity.
func (p *nvmlHealthProvider) registerDeviceEvents(eventSet nvml.EventSet) {
	eventMask := uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)

	for uuid, d := range p.parentToDeviceMap {
		gpu, ret := p.nvmllib.DeviceGetHandleByUUID(uuid)
		if ret != nvml.SUCCESS {
			klog.Infof("unable to get device handle from UUID: %v; marking it as unhealthy", ret)
			sendUnhealthyDevice(p.unhealthy, d)
			continue
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			klog.Infof("unable to determine the supported events for %v: %v; marking it as unhealthy", d.ID, ret)
			sendUnhealthyDevice(p.unhealthy, d)
			continue
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, eventSet)
		if ret == nvml.ERROR_NOT_SUPPORTED {
			klog.Warningf("Device %v is too old to support healthchecking.", d.ID)
		}
		if ret != nvml.SUCCESS {
			klog.Infof("Marking device %v as unhealthy: %v", d.ID, ret)
			sendUnhealthyDevice(p.unhealthy, d)
		}
	}
}

// handleEventWaitError categorizes NVML errors and determines the
// appropriate action. Returns true if health checking should continue,
// false if it should terminate.
func (r *nvmlResourceManager) handleEventWaitError(
	ret nvml.Return,
	devices Devices,
	unhealthy chan<- *Device,
) bool {
	klog.Errorf("Error waiting for NVML event: %v (code: %d)", ret, ret)

	switch ret {
	case nvml.ERROR_GPU_IS_LOST:
		// Definitive hardware failure - mark all devices unhealthy
		klog.Error("GPU_IS_LOST error: Marking all devices as unhealthy")
		for _, d := range devices {
			sendUnhealthyDevice(unhealthy, d)
		}
		return true // Continue checking - devices may recover

	case nvml.ERROR_UNINITIALIZED:
		// NVML state corrupted - this shouldn't happen in event loop
		klog.Error("NVML uninitialized error: This is unexpected, terminating health check")
		return false // Fatal, exit health check

	case nvml.ERROR_UNKNOWN, nvml.ERROR_NOT_SUPPORTED:
		// Potentially transient or driver issue
		klog.Warningf("Transient NVML error (%v): Will retry on next iteration", ret)
		return true // Continue checking

	default:
		// Unknown error - be conservative and mark devices unhealthy
		klog.Errorf("Unexpected NVML error %v: Marking all devices unhealthy conservatively", ret)
		for _, d := range devices {
			sendUnhealthyDevice(unhealthy, d)
		}
		return true // Continue checking
	}
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *nvmlResourceManager) checkHealth(stop <-chan interface{}, devices Devices, unhealthy chan<- *Device) error {
	// Initialize stats tracking
	stats := &healthCheckStats{
		startTime: time.Now(),
		xidByType: make(map[uint64]uint64),
	}
	defer stats.report() // Log stats summary on exit

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
	klog.V(2).Infof("CheckHealth: Starting for %d devices", len(devices))

	eventSet, ret := r.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to create event set: %v", ret)
	}
	defer func() {
		_ = eventSet.Free()
	}()

	// Build device placement maps for MIG support
	parentToDeviceMap := make(map[string]*Device)
	deviceIDToGiMap := make(map[string]uint32)
	deviceIDToCiMap := make(map[string]uint32)

	for _, d := range devices {
		uuid, gi, ci, err := r.getDevicePlacement(d)
		if err != nil {
			klog.Warningf("Could not determine device placement for %v: %v; Marking it unhealthy.", d.ID, err)
			sendUnhealthyDevice(unhealthy, d)
			continue
		}
		deviceIDToGiMap[d.ID] = gi
		deviceIDToCiMap[d.ID] = ci
		parentToDeviceMap[uuid] = d
	}

	// Create health provider with device maps
	provider := &nvmlHealthProvider{
		nvmllib:           r.nvml,
		devices:           devices,
		parentToDeviceMap: parentToDeviceMap,
		deviceIDToGiMap:   deviceIDToGiMap,
		deviceIDToCiMap:   deviceIDToCiMap,
		xidsDisabled:      xids,
		unhealthy:         unhealthy,
		stats:             stats,
	}

	// Register device events
	provider.registerDeviceEvents(eventSet)

	// Create context for coordinating shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine to watch for stop signal and cancel context
	go func() {
		<-stop
		cancel()
	}()

	// Start periodic stats reporting goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats.report()
			}
		}
	}()

	// Event receive channel with small buffer
	eventChan := make(chan eventResult, 10)

	// Start goroutine to receive NVML events
	go func() {
		defer close(eventChan)
		for {
			// Check if we should stop
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Wait for NVML event with timeout
			e, ret := eventSet.Wait(5000)

			// Try to send event result, but respect context cancellation
			select {
			case <-ctx.Done():
				return
			case eventChan <- eventResult{event: e, ret: ret}:
			}
		}
	}()

	// Main event processing loop
	for {
		select {
		case <-ctx.Done():
			klog.V(2).Info("Health check stopped cleanly")
			return nil

		case result, ok := <-eventChan:
			if !ok {
				// Event channel closed, exit
				return nil
			}

			// Handle timeout - just continue
			if result.ret == nvml.ERROR_TIMEOUT {
				continue
			}

			// Handle NVML errors with granular error handling
			if result.ret != nvml.SUCCESS {
				stats.recordError()
				shouldContinue := r.handleEventWaitError(result.ret, devices, unhealthy)
				if !shouldContinue {
					return fmt.Errorf("fatal NVML error: %v", result.ret)
				}
				continue
			}

			e := result.event

			// Filter non-critical events
			if e.EventType != nvml.EventTypeXidCriticalError {
				klog.Infof("Skipping non-nvmlEventTypeXidCriticalError event: %+v", e)
				continue
			}

			// Check if this XID is disabled
			if provider.xidsDisabled.IsDisabled(e.EventData) {
				klog.Infof("Skipping event %+v", e)
				continue
			}

			klog.Infof("Processing event %+v", e)

			// Record event stats
			stats.recordEvent(e.EventData)

			// Get device UUID from event
			eventUUID, ret := e.Device.GetUUID()
			if ret != nvml.SUCCESS {
				// If we cannot reliably determine the device UUID, we mark all devices as unhealthy.
				klog.Infof("Failed to determine uuid for event %v: %v; Marking all devices as unhealthy.", e, ret)
				stats.recordError()
				for _, d := range devices {
					stats.recordUnhealthy()
					sendUnhealthyDevice(unhealthy, d)
				}
				continue
			}

			// Find the device that matches this event
			d, exists := provider.parentToDeviceMap[eventUUID]
			if !exists {
				klog.Infof("Ignoring event for unexpected device: %v", eventUUID)
				continue
			}

			// For MIG devices, verify the GI/CI matches
			if d.IsMigDevice() && e.GpuInstanceId != 0xFFFFFFFF && e.ComputeInstanceId != 0xFFFFFFFF {
				gi := provider.deviceIDToGiMap[d.ID]
				ci := provider.deviceIDToCiMap[d.ID]
				if gi != e.GpuInstanceId || ci != e.ComputeInstanceId {
					continue
				}
				klog.Infof("Event for mig device %v (gi=%v, ci=%v)", d.ID, gi, ci)
			}

			klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking device as unhealthy.", e.EventData, d.ID)
			stats.recordUnhealthy()
			d.MarkUnhealthy(fmt.Sprintf("XID-%d", e.EventData))
			sendUnhealthyDevice(unhealthy, d)
		}
	}
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
