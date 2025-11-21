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
	// envDisableHealthChecks defines the environment variable that is
	// checked to determine whether healthchecks should be disabled. If
	// this envvar is set to "all" or contains the string "xids",
	// healthchecks are disabled entirely. If set, the envvar is treated
	// as a comma-separated list of Xids to ignore. Note that this is in
	// addition to the Application errors that are already ignored.
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	// envEnableHealthChecks defines the environment variable that is
	// checked to determine which XIDs should be explicitly enabled. XIDs
	// specified here override the ones specified in the
	// `DP_DISABLE_HEALTHCHECKS`. Note that this also allows individual
	// XIDs to be selected when ALL XIDs are disabled.
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
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// State guards
	mu      sync.Mutex
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

// NewNVMLHealthProvider creates a new health provider for NVML devices.
// Does not start monitoring - caller must call Start().
func NewNVMLHealthProvider(
	nvml nvml.Interface,
	config *spec.Config,
	devices Devices,
) HealthProvider {
	return &nvmlHealthProvider{
		nvml:       nvml,
		config:     config,
		devices:    devices,
		healthChan: make(chan *Device, 64),
	}
}

// Start initializes NVML, registers event handlers, and starts the
// monitoring goroutine. Blocks until initialization completes.
func (p *nvmlHealthProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("health provider already started")
	}
	p.started = true
	p.mu.Unlock()

	// Check if health checks are disabled
	p.xidsDisabled = getDisabledHealthCheckXids()
	if p.xidsDisabled.IsAllDisabled() {
		klog.Info("Health checks disabled via DP_DISABLE_HEALTHCHECKS")
		return nil
	}

	// Initialize NVML
	ret := p.nvml.Init()
	if ret != nvml.SUCCESS {
		if *p.config.Flags.FailOnInitError {
			return fmt.Errorf("failed to initialize NVML: %v", ret)
		}
		klog.Warningf("NVML init failed: %v; health checks disabled", ret)
		return nil
	}

	// Create event set
	eventSet, ret := p.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		if shutdownRet := p.nvml.Shutdown(); shutdownRet != nvml.SUCCESS {
			klog.Warningf("Failed to shutdown NVML: %v", shutdownRet)
		}
		return fmt.Errorf("failed to create event set: %v", ret)
	}
	p.eventSet = eventSet

	// Register devices
	if err := p.registerDevices(); err != nil {
		p.cleanup()
		return fmt.Errorf("failed to register devices: %w", err)
	}

	klog.Infof("Health monitoring started for %d devices", len(p.devices))
	klog.Infof("Ignoring the following XIDs for health checks: %v",
		p.xidsDisabled)

	// Create child context
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start monitoring goroutine
	p.wg.Add(1)
	go p.runEventMonitor()

	return nil
}

// Stop gracefully shuts down health monitoring and waits for the
// monitoring goroutine to complete.
func (p *nvmlHealthProvider) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	alreadyStarted := p.started
	p.mu.Unlock()

	if !alreadyStarted {
		return
	}

	klog.V(2).Info("Stopping health provider...")

	// Signal goroutine to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Wait for goroutine to finish
	p.wg.Wait()

	// Cleanup NVML resources
	p.cleanup()

	// Close channel
	close(p.healthChan)

	klog.Info("Health provider stopped")
}

// Health returns a read-only channel that receives devices that have
// become unhealthy.
func (p *nvmlHealthProvider) Health() <-chan *Device {
	return p.healthChan
}

// cleanup releases NVML resources.
func (p *nvmlHealthProvider) cleanup() {
	if p.eventSet != nil {
		ret := p.eventSet.Free()
		if ret != nvml.SUCCESS {
			klog.Warningf("Failed to free event set: %v", ret)
		}
		p.eventSet = nil
	}

	if ret := p.nvml.Shutdown(); ret != nvml.SUCCESS {
		klog.Warningf("NVML shutdown failed: %v", ret)
	}
}

// runEventMonitor monitors NVML events and reports unhealthy devices.
// This is the existing checkHealth logic refactored into a goroutine.
func (p *nvmlHealthProvider) runEventMonitor() {
	defer p.wg.Done()

	klog.V(2).Info("Health check: event monitor started")
	defer klog.V(2).Info("Health check: event monitor stopped")

	for {
		// Check for context cancellation
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Wait for NVML event (5 second timeout)
		event, ret := p.eventSet.Wait(5000)

		if ret == nvml.ERROR_TIMEOUT {
			continue
		}

		if ret != nvml.SUCCESS {
			klog.Infof("Error waiting for event: %v; marking all "+
				"devices as unhealthy", ret)
			for _, device := range p.devices {
				p.sendUnhealthy(device)
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
		if p.xidsDisabled.IsDisabled(event.EventData) {
			klog.Infof("Skipping event %+v", event)
			continue
		}

		klog.Infof("Processing event %+v", event)

		// Find device for event
		eventUUID, ret := event.Device.GetUUID()
		if ret != nvml.SUCCESS {
			klog.Infof("Failed to determine uuid for event %v: %v; "+
				"marking all devices as unhealthy.", event, ret)
			for _, device := range p.devices {
				p.sendUnhealthy(device)
			}
			continue
		}

		device, exists := p.parentToDeviceMap[eventUUID]
		if !exists {
			klog.Infof("Ignoring event for unexpected device: %v",
				eventUUID)
			continue
		}

		// Handle MIG devices
		if device.IsMigDevice() &&
			event.GpuInstanceId != 0xFFFFFFFF &&
			event.ComputeInstanceId != 0xFFFFFFFF {
			gi := p.deviceIDToGiMap[device.ID]
			ci := p.deviceIDToCiMap[device.ID]

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
		p.sendUnhealthy(device)
	}
}

// sendUnhealthy sends device to unhealthy channel (non-blocking).
func (p *nvmlHealthProvider) sendUnhealthy(device *Device) {
	select {
	case p.healthChan <- device:
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
func (p *nvmlHealthProvider) registerDevices() error {
	p.parentToDeviceMap = make(map[string]*Device)
	p.deviceIDToGiMap = make(map[string]uint32)
	p.deviceIDToCiMap = make(map[string]uint32)

	eventMask := uint64(nvml.EventTypeXidCriticalError |
		nvml.EventTypeDoubleBitEccError |
		nvml.EventTypeSingleBitEccError)

	for _, device := range p.devices {
		uuid, gi, ci, err := p.getDevicePlacement(device)
		if err != nil {
			klog.Warningf("Could not determine device placement for "+
				"%v: %v; marking it unhealthy.", device.ID, err)
			device.Health = pluginapi.Unhealthy
			p.sendUnhealthy(device)
			continue
		}

		p.deviceIDToGiMap[device.ID] = gi
		p.deviceIDToCiMap[device.ID] = ci
		p.parentToDeviceMap[uuid] = device

		gpu, ret := p.nvml.DeviceGetHandleByUUID(uuid)
		if ret != nvml.SUCCESS {
			klog.Infof("unable to get device handle from UUID: %v; "+
				"marking it as unhealthy", ret)
			device.Health = pluginapi.Unhealthy
			p.sendUnhealthy(device)
			continue
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			klog.Infof("unable to determine the supported events for "+
				"%v: %v; marking it as unhealthy", device.ID, ret)
			device.Health = pluginapi.Unhealthy
			p.sendUnhealthy(device)
			continue
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, p.eventSet)
		if ret == nvml.ERROR_NOT_SUPPORTED {
			klog.Warningf("Device %v is too old to support "+
				"healthchecking.", device.ID)
		}
		if ret != nvml.SUCCESS {
			klog.Infof("Marking device %v as unhealthy: %v",
				device.ID, ret)
			device.Health = pluginapi.Unhealthy
			p.sendUnhealthy(device)
		}
	}

	return nil
}

// getDevicePlacement returns the placement of the specified device.
// For a MIG device the placement is defined by the 3-tuple
// <parent UUID, GI, CI>. For a full device the returned 3-tuple is the
// device's uuid and 0xFFFFFFFF for the other two elements.
func (p *nvmlHealthProvider) getDevicePlacement(
	d *Device,
) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return d.GetUUID(), 0xFFFFFFFF, 0xFFFFFFFF, nil
	}
	return p.getMigDeviceParts(d)
}

// getMigDeviceParts returns the parent GI and CI ids of the MIG device.
func (p *nvmlHealthProvider) getMigDeviceParts(
	d *Device,
) (string, uint32, uint32, error) {
	if !d.IsMigDevice() {
		return "", 0, 0, fmt.Errorf("cannot get GI and CI of full device")
	}

	uuid := d.GetUUID()
	// For older driver versions, the call to DeviceGetHandleByUUID will
	// fail for MIG devices.
	mig, ret := p.nvml.DeviceGetHandleByUUID(uuid)
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
		//nolint:gosec  // We know that the values returned from Get*InstanceId are within the valid uint32 range.
		return parentUUID, uint32(gi), uint32(ci), nil
	}
	return parseMigDeviceUUID(uuid)
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
