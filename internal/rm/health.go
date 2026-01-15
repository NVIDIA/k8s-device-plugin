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
)

const (
	// envDisableHealthChecks defines the environment variable that is checked
	// to determine whether healthchecks should be disabled. If this envvar is
	// set to "all" or contains the string "xids", healthchecks are disabled
	// entirely. If set, the envvar is treated as a comma-separated list of
	// Xids to ignore. Note that this is in addition to the Application errors
	// that are already ignored.
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	// envEnableHealthChecks defines the environment variable that is checked to
	// determine which XIDs should be explicitly enabled. XIDs specified here
	// override the ones specified in the `DP_DISABLE_HEALTHCHECKS`.
	// Note that this also allows individual XIDs to be selected when ALL XIDs
	// are disabled.
	envEnableHealthChecks = "DP_ENABLE_HEALTHCHECKS"

	// unhealthySendTimeout is the maximum time to wait when sending a device
	// to the unhealthy channel if the channel is full. This ensures the health
	// check goroutine doesn't block indefinitely if ListAndWatch is stalled.
	unhealthySendTimeout = 30 * time.Second

	// nvmlInvalidInstanceID represents an invalid/unset value for MIG GPU and
	// Compute instance IDs. Used as a sentinel value for non-MIG devices.
	nvmlInvalidInstanceID uint32 = 0xFFFFFFFF
)

// eventResult packages an NVML event with its return code for passing
// between the event receiver goroutine and the main processing loop.
type eventResult struct {
	event nvml.EventData
	ret   nvml.Return
}

// sendUnhealthyDevice sends a device to the unhealthy channel. It first
// attempts a non-blocking send. If the channel is full, it falls back to a
// blocking send with a timeout. This ensures the health check goroutine
// doesn't block indefinitely while still providing backpressure feedback.
func sendUnhealthyDevice(unhealthy chan<- *Device, d *Device) {
	// Try non-blocking send first
	select {
	case unhealthy <- d:
		klog.V(2).Infof("Device %s sent to unhealthy channel", d.ID)
		return
	default:
		// Channel is full, fall through to blocking send with timeout
	}

	// Channel was full - log warning and try blocking send with timeout
	klog.Warningf("Health channel full (capacity=%d), waiting to send "+
		"device %s (timeout=%v)", cap(unhealthy), d.ID, unhealthySendTimeout)

	select {
	case unhealthy <- d:
		klog.V(2).Infof("Device %s sent to unhealthy channel after wait", d.ID)
	case <-time.After(unhealthySendTimeout):
		// Timeout - ListAndWatch is likely stalled
		klog.Errorf("Timeout after %v sending device %s to unhealthy channel. "+
			"ListAndWatch may be stalled. Device state updated directly but "+
			"kubelet may not be notified.", unhealthySendTimeout, d.ID)
		// Mark unhealthy directly as last resort - kubelet won't see this
		// until ListAndWatch resumes, but at least internal state is correct
		d.MarkUnhealthy("channel-timeout")
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

// report logs a summary of health check statistics. It copies all values
// under the lock to avoid holding the lock during logging operations.
func (s *healthCheckStats) report() {
	s.mu.Lock()
	uptime := time.Since(s.startTime)
	events := s.eventsProcessed
	unhealthy := s.devicesMarkedUnhealthy
	errors := s.errorCount

	// Copy map to avoid reference escaping lock
	var xidCopy map[uint64]uint64
	if len(s.xidByType) > 0 {
		xidCopy = make(map[uint64]uint64, len(s.xidByType))
		for k, v := range s.xidByType {
			xidCopy[k] = v
		}
	}
	s.mu.Unlock()

	klog.Infof("HealthCheck Stats: uptime=%v, events=%d, unhealthy=%d, errors=%d",
		uptime.Round(time.Second), events, unhealthy, errors)

	if len(xidCopy) > 0 {
		klog.Infof("HealthCheck XID distribution: %v", xidCopy)
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

// runEventMonitor runs the main event monitoring loop with context-based
// shutdown coordination and granular error handling. This method preserves
// all robustness features from the original implementation while being
// testable independently.
func (p *nvmlHealthProvider) runEventMonitor(
	ctx context.Context,
	eventSet nvml.EventSet,
	handleError func(nvml.Return, Devices, chan<- *Device) bool,
) error {
	// Event receive channel with buffer
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

			// Wait for NVML event with 5-second timeout. On context cancellation,
			// this goroutine will exit within 5 seconds. This delay is acceptable
			// for Kubernetes graceful shutdown (default terminationGracePeriodSeconds
			// is 30 seconds).
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
				p.stats.recordError()
				shouldContinue := handleError(result.ret, p.devices, p.unhealthy)
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
			if p.xidsDisabled.IsDisabled(e.EventData) {
				klog.Infof("Skipping event %+v", e)
				continue
			}

			klog.Infof("Processing event %+v", e)

			// Record event stats
			p.stats.recordEvent(e.EventData)

			// Get device UUID from event
			eventUUID, ret := e.Device.GetUUID()
			if ret != nvml.SUCCESS {
				// If we cannot reliably determine the device UUID, we mark all devices as unhealthy.
				klog.Infof("Failed to determine uuid for event %v: %v; Marking all devices as unhealthy.", e, ret)
				p.stats.recordError()
				for _, d := range p.devices {
					p.stats.recordUnhealthy()
					sendUnhealthyDevice(p.unhealthy, d)
				}
				continue
			}

			// Find the device that matches this event
			d, exists := p.parentToDeviceMap[eventUUID]
			if !exists {
				klog.Infof("Ignoring event for unexpected device: %v", eventUUID)
				continue
			}

			// For MIG devices, verify the GI/CI matches
			if d.IsMigDevice() && e.GpuInstanceId != nvmlInvalidInstanceID && e.ComputeInstanceId != nvmlInvalidInstanceID {
				gi := p.deviceIDToGiMap[d.ID]
				ci := p.deviceIDToCiMap[d.ID]
				if gi != e.GpuInstanceId || ci != e.ComputeInstanceId {
					continue
				}
				klog.Infof("Event for mig device %v (gi=%v, ci=%v)", d.ID, gi, ci)
			}

			klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking device as unhealthy.", e.EventData, d.ID)
			p.stats.recordUnhealthy()
			d.MarkUnhealthy(fmt.Sprintf("XID-%d", e.EventData))
			sendUnhealthyDevice(p.unhealthy, d)
		}
	}
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
	if devices == nil {
		klog.Error("handleEventWaitError called with nil devices map")
		return false
	}

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

// checkHealth orchestrates GPU health monitoring by coordinating NVML
// initialization, device registration, and event monitoring. This function
// acts as the main entry point and delegates specific responsibilities to
// focused methods on nvmlHealthProvider.
//
// The orchestration flow:
//  1. Initialize stats tracking and XID filtering
//  2. Initialize NVML and create event set
//  3. Build device placement maps (for MIG support)
//  4. Create nvmlHealthProvider with configuration
//  5. Register device events
//  6. Start context-based shutdown coordination
//  7. Start periodic stats reporting
//  8. Run event monitoring loop
//
// All robustness features are preserved: stats tracking, granular error
// handling, context-based shutdown, and non-blocking device reporting.
func (r *nvmlResourceManager) checkHealth(ctx context.Context, stop <-chan interface{}, devices Devices, unhealthy chan<- *Device) error {
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
		if ret := eventSet.Free(); ret != nvml.SUCCESS {
			klog.Warningf("Failed to free NVML event set: %v", ret)
		}
	}()

	// Build device placement maps for MIG support
	parentToDeviceMap := make(map[string]*Device)
	deviceIDToGiMap := make(map[string]uint32)
	deviceIDToCiMap := make(map[string]uint32)

	placements := &withDevicePlacements{r.nvml}
	for _, d := range devices {
		uuid, gi, ci, err := placements.getDevicePlacement(d)
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

	// Create derived context for coordinating shutdown. This allows the caller
	// to propagate cancellation while also supporting the legacy stop channel.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Goroutine to watch for stop signal and cancel context
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
			// Parent context cancelled, no need to wait for stop
		}
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

	// Run event monitor with error handler
	return provider.runEventMonitor(ctx, eventSet, r.handleEventWaitError)
}

const allXIDs = 0

// disabledXIDs stores a map of explicitly disabled XIDs.
// The special XID `allXIDs` indicates that all XIDs are disabled, but does
// allow for specific XIDs to be enabled even if this is the case.
type disabledXIDs map[uint64]bool

// Disabled returns whether XID-based health checks are disabled.
// These are considered if all XIDs have been disabled AND no other XIDs have
// been explicitly enabled.
func (h disabledXIDs) IsAllDisabled() bool {
	if allDisabled, ok := h[allXIDs]; ok {
		return allDisabled
	}
	// At this point we either have explicitly disabled XIDs or explicitly
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

// withDevicePlacements wraps nvml.Interface for device placement operations.
// This enables cleaner testing by allowing placement logic to be tested
// independently of the full resource manager.
type withDevicePlacements struct {
	nvml.Interface
}

// getDevicePlacement returns the placement of the specified device.
// For a MIG device the placement is defined by the 3-tuple
// <parent UUID, GI, CI>. For a full device the returned 3-tuple is the
// device's uuid and nvmlInvalidInstanceID for the other two elements.
func (w *withDevicePlacements) getDevicePlacement(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return d.GetUUID(), nvmlInvalidInstanceID, nvmlInvalidInstanceID, nil
	}
	return w.getMigDeviceParts(d)
}

// getMigDeviceParts returns the parent GI and CI ids of the MIG device.
func (w *withDevicePlacements) getMigDeviceParts(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return "", 0, 0, fmt.Errorf("cannot get GI and CI of full device")
	}

	uuid := d.GetUUID()
	// For older driver versions, the call to DeviceGetHandleByUUID will fail
	// for MIG devices.
	mig, ret := w.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		parentHandle, ret := mig.GetDeviceHandleFromMigDeviceHandle()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent device handle: %v", ret)
		}
		parentUUID, ret := parentHandle.GetUUID()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent uuid: %v", ret)
		}

		giID, ret := mig.GetGpuInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get GPU Instance ID: %v", ret)
		}

		ciID, ret := mig.GetComputeInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get Compute Instance ID: %v", ret)
		}

		//nolint:gosec  // GI/CI IDs are within valid uint32 range.
		return parentUUID, uint32(giID), uint32(ciID), nil
	}

	return parseMigDeviceUUID(uuid)
}
