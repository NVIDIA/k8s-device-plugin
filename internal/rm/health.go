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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

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
)

type placedDevice struct {
	parentUUID string
	device     *Device
}

func groupByParent(devices []placedDevice) map[string][]*Device {
	grouped := make(map[string][]*Device)
	for _, d := range devices {
		grouped[d.parentUUID] = append(grouped[d.parentUUID], d.device)
	}
	return grouped
}

func matchesMigEvent(deviceGI, deviceCI, eventGI, eventCI uint32) bool {
	giMatches := eventGI == nvml.GPU_INSTANCE_ID_ANY || deviceGI == eventGI
	ciMatches := eventCI == nvml.COMPUTE_INSTANCE_ID_ANY || deviceCI == eventCI
	return giMatches && ciMatches
}

const eventSetWaitTimeoutMs = 5000

var errHealthEventsNotSupported = errors.New("health events not supported")

// healthEvent is one result of an event set Wait that is delivered to every subscriber.
type healthEvent struct {
	data nvml.EventData
	ret  nvml.Return
}

// healthWatch owns the only NVML event set in the process and the only goroutine that waits on it.
// With the mixed MIG strategy every profile is a separate resource, and concurrent
// nvmlEventSetWait calls on separate event sets for the same parent GPU spin a CPU core.
//
// A watch is started by its first subscriber and stopped when its last subscriber leaves. It is
// not restarted: each plugin (re)start constructs new resource managers and with them a new watch.
type healthWatch struct {
	nvml nvml.Interface

	startOnce sync.Once
	startErr  error
	eventSet  nvml.EventSet
	// registrations is written before the wait loop starts and is read-only afterwards.
	registrations map[string]error

	mu      sync.Mutex
	subs    map[*healthSubscription]struct{}
	stopped bool
	stop    chan struct{}
	done    chan struct{}
}

func newHealthWatch(nvmllib nvml.Interface) *healthWatch {
	return &healthWatch{
		nvml:          nvmllib,
		registrations: make(map[string]error),
		subs:          make(map[*healthSubscription]struct{}),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
}

// subscribe starts the watch if required and returns a subscription that receives every event.
func (w *healthWatch) subscribe() (*healthSubscription, error) {
	w.startOnce.Do(func() {
		w.startErr = w.start()
	})
	if w.startErr != nil {
		return nil, w.startErr
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return nil, errors.New("health watch is already stopped")
	}
	sub := &healthSubscription{
		events: make(chan healthEvent),
		done:   make(chan struct{}),
	}
	w.subs[sub] = struct{}{}
	return sub, nil
}

// unsubscribe removes the subscription. Removing the last subscription stops the wait loop and
// blocks until the event set has been freed.
func (w *healthWatch) unsubscribe(sub *healthSubscription) {
	w.mu.Lock()
	delete(w.subs, sub)
	close(sub.done)
	last := len(w.subs) == 0 && !w.stopped
	if last {
		w.stopped = true
		close(w.stop)
	}
	w.mu.Unlock()

	if last {
		<-w.done
	}
}

func (w *healthWatch) start() error {
	ret := w.nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", ret)
	}

	eventSet, ret := w.nvml.EventSetCreate()
	if ret != nvml.SUCCESS {
		_ = w.nvml.Shutdown()
		return fmt.Errorf("failed to create event set: %v", ret)
	}

	count, ret := w.nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		_ = eventSet.Free()
		_ = w.nvml.Shutdown()
		return fmt.Errorf("failed to get device count: %v", ret)
	}

	eventMask := uint64(nvml.EventTypeXidCriticalError | nvml.EventTypeDoubleBitEccError | nvml.EventTypeSingleBitEccError)
	for i := range count {
		gpu, ret := w.nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			klog.Warningf("Unable to get handle for device at index %d: %v", i, ret)
			continue
		}
		uuid, ret := gpu.GetUUID()
		if ret != nvml.SUCCESS {
			klog.Warningf("Unable to get UUID for device at index %d: %v", i, ret)
			continue
		}

		supportedEvents, ret := gpu.GetSupportedEventTypes()
		if ret != nvml.SUCCESS {
			w.registrations[uuid] = fmt.Errorf("unable to determine the supported events: %v", ret)
			continue
		}

		ret = gpu.RegisterEvents(eventMask&supportedEvents, eventSet)
		switch {
		case ret == nvml.ERROR_NOT_SUPPORTED:
			w.registrations[uuid] = errHealthEventsNotSupported
		case ret != nvml.SUCCESS:
			w.registrations[uuid] = fmt.Errorf("failed to register events: %v", ret)
		default:
			w.registrations[uuid] = nil
		}
	}
	w.eventSet = eventSet

	go w.run()
	return nil
}

func (w *healthWatch) run() {
	defer close(w.done)
	defer func() {
		if ret := w.eventSet.Free(); ret != nvml.SUCCESS {
			klog.Infof("Error freeing event set: %v", ret)
		}
		if ret := w.nvml.Shutdown(); ret != nvml.SUCCESS {
			klog.Infof("Error shutting down NVML: %v", ret)
		}
	}()

	for {
		select {
		case <-w.stop:
			return
		default:
		}

		e, ret := w.eventSet.Wait(eventSetWaitTimeoutMs)
		if ret == nvml.ERROR_TIMEOUT {
			continue
		}

		w.mu.Lock()
		subs := make([]*healthSubscription, 0, len(w.subs))
		for sub := range w.subs {
			subs = append(subs, sub)
		}
		w.mu.Unlock()

		// Sending outside the lock lets a subscriber that the loop is blocked on unsubscribe.
		for _, sub := range subs {
			select {
			case sub.events <- healthEvent{data: e, ret: ret}:
			case <-sub.done:
			}
		}
	}
}

// healthSubscription delivers every event from the shared wait loop to one plugin.
type healthSubscription struct {
	events chan healthEvent
	done   chan struct{}
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *nvmlResourceManager) checkHealth(stop <-chan any, devices Devices, unhealthy chan<- *Device) error {
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

	sub, err := r.watch.subscribe()
	if err != nil {
		return err
	}
	defer r.watch.unsubscribe(sub)

	// The sends must not outlive the plugin, or the subscription is never released and the
	// shared event set is never freed.
	markUnhealthy := func(d *Device) {
		select {
		case unhealthy <- d:
		case <-stop:
		}
	}

	placedDevices := make([]placedDevice, 0, len(devices))
	deviceIDToGiMap := make(map[string]uint32)
	deviceIDToCiMap := make(map[string]uint32)

	for _, d := range devices {
		uuid, gi, ci, err := r.getDevicePlacement(d)
		if err != nil {
			klog.Warningf("Could not determine device placement for %v: %v; Marking it unhealthy.", d.ID, err)
			markUnhealthy(d)
			continue
		}
		deviceIDToGiMap[d.ID] = gi
		deviceIDToCiMap[d.ID] = ci
		placedDevices = append(placedDevices, placedDevice{parentUUID: uuid, device: d})
	}
	parentToDeviceMap := groupByParent(placedDevices)

	for parentUUID, subdevices := range parentToDeviceMap {
		err, registered := r.watch.registrations[parentUUID]
		switch {
		case !registered:
			klog.Infof("Marking device %v as unhealthy: not registered for health events", parentUUID)
			for _, d := range subdevices {
				markUnhealthy(d)
			}
		case errors.Is(err, errHealthEventsNotSupported):
			klog.Warningf("Device %v is too old to support healthchecking.", parentUUID)
		case err != nil:
			klog.Infof("Marking device %v as unhealthy: %v", parentUUID, err)
			for _, d := range subdevices {
				markUnhealthy(d)
			}
		}
	}

	for {
		var event healthEvent
		select {
		case <-stop:
			return nil
		case event = <-sub.events:
		}

		e, ret := event.data, event.ret
		if ret != nvml.SUCCESS {
			klog.Infof("Error waiting for event: %v; Marking all devices as unhealthy", ret)
			for _, d := range devices {
				markUnhealthy(d)
			}
			continue
		}

		if e.EventType != nvml.EventTypeXidCriticalError {
			klog.Infof("Skipping non-nvmlEventTypeXidCriticalError event: %+v", e)
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
				markUnhealthy(d)
			}
			continue
		}

		ds, exists := parentToDeviceMap[eventUUID]
		if !exists {
			klog.Infof("Ignoring event for unexpected device: %v", eventUUID)
			continue
		}

		for _, d := range ds {
			if d.IsMigDevice() {
				gi := deviceIDToGiMap[d.ID]
				ci := deviceIDToCiMap[d.ID]
				if !matchesMigEvent(gi, ci, e.GpuInstanceId, e.ComputeInstanceId) {
					continue
				}
				klog.Infof("Event for mig device %v (gi=%v, ci=%v)", d.ID, gi, ci)
			}

			klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking device as unhealthy.", e.EventData, d.ID)
			markUnhealthy(d)
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
	if allDisabled, ok := h[allXIDs]; !ok || !allDisabled {
		// At this point we wither have explicitly disabled XIDs or explicitly
		// enabled XIDs. Since ANY XID that's not specified is assumed enabled, we
		// return here.
		return false
	}
	// An XID the caller enabled explicitly is still checked for, even with
	// every other one disabled, so the health checks as a whole are not off.
	// getDisabledHealthCheckXids records such an XID as a false entry.
	for xid, disabled := range h {
		if xid != allXIDs && !disabled {
			return false
		}
	}
	return true
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
	// An XID nobody named is disabled exactly when all XIDs are. Asking
	// IsAllDisabled here would make such an XID checked for as soon as some
	// other XID is enabled by name.
	return h[allXIDs]
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
	if ret != nvml.SUCCESS {
		// Fall back to parsing the legacy MIG UUID format (MIG-GPU-<parent-uuid>/<gi>/<ci>).
		// Modern drivers assign opaque MIG UUIDs that carry no placement information,
		// so if parsing fails the NVML error above must not be masked: it is the
		// actual reason the device placement could not be determined.
		parentUUID, gi, ci, err := parseMigDeviceUUID(uuid)
		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to get MIG device handle for %s: %s; %w", uuid, nvml.ErrorString(ret), err)
		}
		return parentUUID, gi, ci, nil
	}
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
