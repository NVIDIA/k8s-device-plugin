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

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
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

// HealthProvider manages GPU device health monitoring with lifecycle
// control.
type HealthProvider interface {
	// Start initiates health monitoring. Blocks until initial setup
	// completes. Returns error if health monitoring cannot be started.
	Start(context.Context) error

	// Stop gracefully shuts down health monitoring and waits for all
	// goroutines to complete. Safe to call multiple times.
	Stop()

	// Health returns a read-only channel that receives devices that
	// have become unhealthy.
	Health() <-chan *Device
}

// nvmlHealthProvider implements HealthProvider using NVML event
// monitoring. This is a refactoring of the existing checkHealth logic
// with proper lifecycle management.
type nvmlHealthProvider struct {
	// Configuration
	nvml    nvml.Interface
	config  *spec.Config
	devices Devices

	// NVML resources
	eventSet nvml.EventSet

	// Lifecycle management
	ctx context.Context
	wg  sync.WaitGroup

	// State guards
	sync.Mutex
	started bool
	stopped bool

	// Communication
	healthChan chan *Device

	// XID filtering
	xidsDisabled disabledXIDs

	// Device placement maps (for MIG support)
	parentToDeviceMap map[string]*Device
	deviceIDToGiMap   map[string]uint32
	deviceIDToCiMap   map[string]uint32
}

// newNVMLHealthProvider creates a new health provider for NVML devices.
// Does not start monitoring - caller must call Start().
func newNVMLHealthProvider(ctx context.Context, nvmllib nvml.Interface, config *spec.Config, devices Devices) (HealthProvider, error) {
	xids := getDisabledHealthCheckXids()
	if xids.IsAllDisabled() {
		return &noopHealthProvider{}, nil
	}

	ret := nvmllib.Init()
	if ret != nvml.SUCCESS {
		if *config.Flags.FailOnInitError {
			return nil, fmt.Errorf("failed to initialize NVML: %v", ret)
		}
		klog.Warningf("NVML init failed: %v; health checks disabled", ret)
		return &noopHealthProvider{}, nil
	}
	defer func() {
		ret := nvmllib.Shutdown()
		if ret != nvml.SUCCESS {
			klog.Infof("Error shutting down NVML: %v", ret)
		}
	}()

	klog.Infof("Ignoring the following XIDs for health checks: %v", xids)

	p := &nvmlHealthProvider{
		ctx:          ctx,
		nvml:         nvmllib,
		config:       config,
		devices:      devices,
		healthChan:   make(chan *Device, 64),
		xidsDisabled: xids,
	}
	return p, nil
}

// Start initializes NVML, registers event handlers, and starts the
// monitoring goroutine. Blocks until initialization completes.
func (r *nvmlHealthProvider) Start(ctx context.Context) (rerr error) {
	r.Lock()
	defer r.Unlock()
	if r.started {
		// TODO: Is this an error condition? Could we just return?
		return fmt.Errorf("health provider already started")
	}
	r.Unlock()

	// Initialize NVML
	ret := r.nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", ret)
	}
	defer func() {
		if rerr != nil {
			_ = r.nvml.Shutdown()
		}
	}()

	// Create event set
	eventSet, ret := r.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to create event set: %v", ret)
	}
	defer func() {
		if rerr != nil {
			_ = eventSet.Free()
		}
	}()
	r.eventSet = eventSet

	// Register devices
	if err := r.registerDevices(); err != nil {
		return fmt.Errorf("failed to register devices: %w", err)
	}

	klog.Infof("Health monitoring started for %d devices", len(r.devices))

	// Start monitoring goroutine
	r.wg.Add(1)
	go r.runEventMonitor()

	r.started = true

	return nil
}

// Stop gracefully shuts down health monitoring and waits for the
// monitoring goroutine to complete.
func (r *nvmlHealthProvider) Stop() {
	r.Lock()
	defer r.Unlock()

	if r.stopped {
		return
	}

	if !r.started {
		r.stopped = true
		return
	}

	klog.V(2).Info("Stopping health provider...")

	// Wait for goroutine to finish (unlock during wait)
	// Goroutine will exit when parent context is cancelled
	r.Unlock()
	r.wg.Wait()
	r.Lock()

	// Cleanup NVML resources
	r.cleanup()

	// Close channel
	close(r.healthChan)

	r.stopped = true

	klog.Info("Health provider stopped")
}

// Health returns a read-only channel that receives devices that have
// become unhealthy.
func (r *nvmlHealthProvider) Health() <-chan *Device {
	return r.healthChan
}

// cleanup releases NVML resources.
func (r *nvmlHealthProvider) cleanup() {
	if r.eventSet != nil {
		ret := r.eventSet.Free()
		if ret != nvml.SUCCESS {
			klog.Warningf("Failed to free event set: %v", ret)
		}
		r.eventSet = nil
	}

	if ret := r.nvml.Shutdown(); ret != nvml.SUCCESS {
		klog.Warningf("NVML shutdown failed: %v", ret)
	}
}

// runEventMonitor monitors NVML events and reports unhealthy devices.
// This is the existing checkHealth logic refactored into a goroutine.
func (r *nvmlHealthProvider) runEventMonitor() {
	defer r.wg.Done()

	klog.V(2).Info("Health check: event monitor started")
	defer klog.V(2).Info("Health check: event monitor stopped")

	for {
		// Check for context cancellation
		select {
		case <-r.ctx.Done():
			return
		default:
		}

		// Wait for NVML event (5 second timeout)
		event, ret := r.eventSet.Wait(5000)
		if ret == nvml.ERROR_TIMEOUT {
			continue
		}

		if ret != nvml.SUCCESS {
			klog.Infof("Error waiting for event: %v; marking all "+
				"devices as unhealthy", ret)
			for _, device := range r.devices {
				r.sendUnhealthy(device)
			}
			continue
		}

		// Only process XID critical errors
		if event.EventType != nvml.EventTypeXidCriticalError {
			klog.Infof("Skipping non-nvmlEventTypeXidCriticalError "+
				"event: %+v", event)
			continue
		}

		// Check if XID is disabled
		if r.xidsDisabled.IsDisabled(event.EventData) {
			klog.Infof("Skipping event %+v", event)
			continue
		}

		klog.Infof("Processing event %+v", event)

		// Find device for event
		eventUUID, ret := event.Device.GetUUID()
		if ret != nvml.SUCCESS {
			klog.Infof("Failed to determine uuid for event %v: %v; "+
				"marking all devices as unhealthy.", event, ret)
			for _, device := range r.devices {
				r.sendUnhealthy(device)
			}
			continue
		}

		device, exists := r.parentToDeviceMap[eventUUID]
		if !exists {
			klog.Infof("Ignoring event for unexpected device: %v",
				eventUUID)
			continue
		}

		// Handle MIG devices
		if device.IsMigDevice() &&
			event.GpuInstanceId != 0xFFFFFFFF &&
			event.ComputeInstanceId != 0xFFFFFFFF {
			gi := r.deviceIDToGiMap[device.ID]
			ci := r.deviceIDToCiMap[device.ID]

			if gi != event.GpuInstanceId || ci != event.ComputeInstanceId {
				continue
			}

			klog.Infof("Event for mig device %v (gi=%v, ci=%v)",
				device.ID, gi, ci)
		}

		// Mark device unhealthy
		klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking "+
			"device as unhealthy.", event.EventData, device.ID)

		device.Health = pluginapi.Unhealthy
		r.sendUnhealthy(device)
	}
}

// sendUnhealthy sends device to unhealthy channel (non-blocking).
func (r *nvmlHealthProvider) sendUnhealthy(device *Device) {
	select {
	case r.healthChan <- device:
		// Sent successfully
	default:
		// Channel full
		klog.Errorf("Health channel full! Device %s update dropped. "+
			"ListAndWatch may be stalled.", device.ID)
		// Device.Health already set to Unhealthy
	}
}

// registerDevices registers all devices with the NVML event set.
// This is the existing logic from checkHealth().
func (r *nvmlHealthProvider) registerDevices() error {
	r.parentToDeviceMap = make(map[string]*Device)
	r.deviceIDToGiMap = make(map[string]uint32)
	r.deviceIDToCiMap = make(map[string]uint32)

	eventMask := uint64(nvml.EventTypeXidCriticalError |
		nvml.EventTypeDoubleBitEccError |
		nvml.EventTypeSingleBitEccError)

	for _, device := range r.devices {
		uuid, gi, ci, err := r.getDevicePlacement(device)
		if err != nil {
			klog.Warningf("Could not determine device placement for "+
				"%v: %v; marking it unhealthy.", device.ID, err)
			device.Health = pluginapi.Unhealthy
			r.sendUnhealthy(device)
			continue
		}

		r.deviceIDToGiMap[device.ID] = gi
		r.deviceIDToCiMap[device.ID] = ci
		r.parentToDeviceMap[uuid] = device

		gpu, ret := r.nvml.DeviceGetHandleByUUID(uuid)
		if ret != nvml.SUCCESS {
			klog.Infof("unable to get device handle from UUID: %v; "+
				"marking it as unhealthy", ret)
			device.Health = pluginapi.Unhealthy
			r.sendUnhealthy(device)
			continue
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			klog.Infof("unable to determine the supported events for "+
				"%v: %v; marking it as unhealthy", device.ID, ret)
			device.Health = pluginapi.Unhealthy
			r.sendUnhealthy(device)
			continue
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, r.eventSet)
		if ret == nvml.ERROR_NOT_SUPPORTED {
			klog.Warningf("Device %v is too old to support "+
				"healthchecking.", device.ID)
		}
		if ret != nvml.SUCCESS {
			klog.Infof("Marking device %v as unhealthy: %v",
				device.ID, ret)
			device.Health = pluginapi.Unhealthy
			r.sendUnhealthy(device)
		}
	}

	return nil
}

const allXIDs = 0

// disabledXIDs stores a map of explicitly disabled XIDs.
// The special XID `allXIDs` indicates that all XIDs are disabled, but
// does allow for specific XIDs to be enabled even if this is the case.
type disabledXIDs map[uint64]bool

// IsAllDisabled returns whether XID-based health checks are disabled.
// These are considered if all XIDs have been disabled AND no other XIDs
// have been explcitly enabled.
func (h disabledXIDs) IsAllDisabled() bool {
	if allDisabled, ok := h[allXIDs]; ok {
		return allDisabled
	}
	// At this point we wither have explicitly disabled XIDs or
	// explicitly enabled XIDs. Since ANY XID that's not specified is
	// assumed enabled, we return here.
	return false
}

// IsDisabled checks whether the specified XID has been explicitly
// disalbled. An XID is considered disabled if it has been explicitly
// disabled, or all XIDs have been disabled.
func (h disabledXIDs) IsDisabled(xid uint64) bool {
	// Handle the case where enabled=all.
	if explicitAll, ok := h[allXIDs]; ok && !explicitAll {
		return false
	}
	// Handle the case where the XID has been specifically enabled (or
	// disabled)
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
// Note that if an XID is explicitly enabled, this takes precedence over
// it having been disabled either explicitly or implicitly.
func getDisabledHealthCheckXids() disabledXIDs {
	disabled := newHealthCheckXIDs(
		// TODO: We should not read the envvar here directly, but
		// instead "upgrade" this to a top-level config option.
		strings.Split(strings.ToLower(os.Getenv(envDisableHealthChecks)), ",")...,
	)
	enabled := newHealthCheckXIDs(
		// TODO: We should not read the envvar here directly, but
		// instead "upgrade" this to a top-level config option.
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
// Special xid values 'all' and 'xids' return a special map that matches
// all xids. For other xids, these are converted to a uint64 values with
// invalid values being ignored.
func newHealthCheckXIDs(xids ...string) disabledXIDs {
	output := make(disabledXIDs)
	for _, xid := range xids {
		trimmed := strings.TrimSpace(xid)
		if trimmed == "all" || trimmed == "xids" {
			// TODO: We should have a different type for "all" and
			// "all-except"
			return disabledXIDs{allXIDs: true}
		}
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			klog.Infof("Ignoring malformed Xid value %v: %v",
				trimmed, err)
			continue
		}

		output[id] = true
	}
	return output
}

// getDevicePlacement returns the placement of the specified device.
// For a MIG device the placement is defined by the 3-tuple
// <parent UUID, GI, CI>. For a full device the returned 3-tuple is the
// device's uuid and 0xFFFFFFFF for the other two elements.
func (r *nvmlHealthProvider) getDevicePlacement(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return d.GetUUID(), 0xFFFFFFFF, 0xFFFFFFFF, nil
	}
	return r.getMigDeviceParts(d)
}

// getMigDeviceParts returns the parent GI and CI ids of the MIG device.
func (r *nvmlHealthProvider) getMigDeviceParts(d *Device) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return "", 0, 0, fmt.Errorf("cannot get GI and CI of full device")
	}

	uuid := d.GetUUID()
	// For older driver versions, the call to DeviceGetHandleByUUID will
	// fail for MIG devices.
	mig, ret := r.nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		parentHandle, ret := mig.GetDeviceHandleFromMigDeviceHandle()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent "+
				"device handle: %v", ret)
		}

		parentUUID, ret := parentHandle.GetUUID()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get parent "+
				"uuid: %v", ret)
		}
		gi, ret := mig.GetGpuInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get GPU "+
				"Instance ID: %v", ret)
		}

		ci, ret := mig.GetComputeInstanceId()
		if ret != nvml.SUCCESS {
			return "", 0, 0, fmt.Errorf("failed to get Compute "+
				"Instance ID: %v", ret)
		}
		//nolint:gosec  // We know that the values returned from Get*InstanceId
		//  are within the valid uint32 range.
		return parentUUID, uint32(gi), uint32(ci), nil
	}
	return parseMigDeviceUUID(uuid)
}

// parseMigDeviceUUID splits the MIG device UUID into the parent device
// UUID and ci and gi
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

// noopHealthProvider is a no-op implementation for platforms or
// configurations that don't support health monitoring.
type noopHealthProvider struct {
	healthChan chan *Device
}

func (n *noopHealthProvider) Start(context.Context) error {
	n.healthChan = make(chan *Device)
	return nil
}

func (n *noopHealthProvider) Stop() {
	if n.healthChan != nil {
		close(n.healthChan)
	}
}

func (n *noopHealthProvider) Health() <-chan *Device {
	return n.healthChan
}
