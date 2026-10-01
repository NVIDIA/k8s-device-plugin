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
	"slices"
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

// gpuToWatch is a parent GPU that should be registered with the shared event set.
type gpuToWatch struct {
	uuid   string
	handle nvml.Device
	mask   uint64
	device *Device
}

// healthSub is one plugin's devices and the channel used to mark them unhealthy.
type healthSub struct {
	devices           Devices
	parentToDeviceMap map[string][]*Device
	deviceIDToGiMap   map[string]uint32
	deviceIDToCiMap   map[string]uint32
	xids              disabledXIDs
	unhealthy         chan<- *Device
	stop              <-chan any
}

func (s *healthSub) notify(d *Device) {
	select {
	case s.unhealthy <- d:
	case <-s.stop:
	}
}

// healthWatch owns the process-wide NVML event set and the single Wait loop.
// Mixed MIG starts one plugin per profile; sharing one Wait avoids overlapping
// nvmlEventSetWait calls on the same parent GPU.
type healthWatch struct {
	mu         sync.Mutex
	eventMu    sync.Mutex
	eventSet   nvml.EventSet
	registered map[string]struct{}
	subs       []*healthSub
	running    bool
	// draining is set when the last subscriber leaves so a concurrent
	// subscribe waits for the wait loop to free the event set.
	draining bool
	loopDone chan struct{}
}

func newHealthWatch() *healthWatch {
	return &healthWatch{
		registered: make(map[string]struct{}),
	}
}

func (w *healthWatch) subscribe(lib nvml.Interface, sub *healthSub, gpus []gpuToWatch) error {
	w.mu.Lock()
	for w.draining {
		done := w.loopDone
		w.mu.Unlock()
		if done != nil {
			<-done
		}
		w.mu.Lock()
	}
	if w.eventSet == nil {
		eventSet, ret := lib.EventSetCreate()
		if ret != nvml.SUCCESS {
			w.mu.Unlock()
			return fmt.Errorf("failed to create event set: %v", ret)
		}
		w.eventSet = eventSet
	}
	eventSet := w.eventSet
	// Join subs before RegisterEvents so a mixed-MIG profile is already in
	// the fan-out list when the parent GPU is already registered.
	w.subs = append(w.subs, sub)
	w.mu.Unlock()

	for _, gpu := range gpus {
		w.eventMu.Lock()
		w.mu.Lock()
		_, already := w.registered[gpu.uuid]
		if already {
			w.mu.Unlock()
			w.eventMu.Unlock()
			continue
		}
		ret := gpu.handle.RegisterEvents(gpu.mask, eventSet)
		if ret == nvml.SUCCESS {
			w.registered[gpu.uuid] = struct{}{}
		}
		w.mu.Unlock()
		w.eventMu.Unlock()

		switch {
		case ret == nvml.ERROR_NOT_SUPPORTED:
			klog.Warningf("Device %v is too old to support healthchecking.", gpu.device.ID)
		case ret != nvml.SUCCESS:
			klog.Infof("Marking device %v as unhealthy: %v", gpu.device.ID, ret)
			sub.notify(gpu.device)
		}
	}

	w.mu.Lock()
	if !w.running {
		w.running = true
		w.loopDone = make(chan struct{})
		go w.loop()
	}
	w.mu.Unlock()
	return nil
}

func (w *healthWatch) unsubscribe(sub *healthSub) {
	w.mu.Lock()
	w.subs = slices.DeleteFunc(w.subs, func(s *healthSub) bool {
		return s == sub
	})
	if len(w.subs) > 0 || !w.running {
		w.mu.Unlock()
		return
	}
	w.draining = true
	done := w.loopDone
	w.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (w *healthWatch) idleLocked() bool {
	return len(w.subs) == 0
}

func (w *healthWatch) freeEventSet(es nvml.EventSet) {
	if es == nil {
		return
	}
	w.eventMu.Lock()
	ret := es.Free()
	w.eventMu.Unlock()
	if ret != nvml.SUCCESS {
		klog.Infof("Error freeing event set: %v", ret)
	}
}

func (w *healthWatch) loop() {
	for {
		w.mu.Lock()
		if w.idleLocked() {
			es := w.eventSet
			w.eventSet = nil
			w.registered = make(map[string]struct{})
			w.running = false
			done := w.loopDone
			w.mu.Unlock()

			w.freeEventSet(es)

			w.mu.Lock()
			w.draining = false
			w.mu.Unlock()
			if done != nil {
				close(done)
			}
			return
		}
		eventSet := w.eventSet
		w.mu.Unlock()

		w.eventMu.Lock()
		e, ret := eventSet.Wait(eventSetWaitTimeoutMs)
		w.eventMu.Unlock()

		w.mu.Lock()
		subs := append([]*healthSub(nil), w.subs...)
		w.mu.Unlock()
		if len(subs) == 0 {
			continue
		}

		if ret == nvml.ERROR_TIMEOUT {
			continue
		}
		if ret != nvml.SUCCESS {
			klog.Infof("Error waiting for event: %v; Marking all devices as unhealthy", ret)
			for _, s := range subs {
				for _, d := range s.devices {
					s.notify(d)
				}
			}
			continue
		}

		for _, s := range subs {
			s.handleEvent(e)
		}
	}
}

func (s *healthSub) handleEvent(e nvml.EventData) {
	if e.EventType != nvml.EventTypeXidCriticalError {
		klog.Infof("Skipping non-nvmlEventTypeXidCriticalError event: %+v", e)
		return
	}
	if s.xids.IsDisabled(e.EventData) {
		klog.Infof("Skipping event %+v", e)
		return
	}

	klog.Infof("Processing event %+v", e)
	eventUUID, ret := e.Device.GetUUID()
	if ret != nvml.SUCCESS {
		klog.Infof("Failed to determine uuid for event %v: %v; Marking all devices as unhealthy.", e, ret)
		for _, d := range s.devices {
			s.notify(d)
		}
		return
	}

	ds, exists := s.parentToDeviceMap[eventUUID]
	if !exists {
		return
	}

	for _, d := range ds {
		if d.IsMigDevice() {
			gi := s.deviceIDToGiMap[d.ID]
			ci := s.deviceIDToCiMap[d.ID]
			if !matchesMigEvent(gi, ci, e.GpuInstanceId, e.ComputeInstanceId) {
				continue
			}
			klog.Infof("Event for mig device %v (gi=%v, ci=%v)", d.ID, gi, ci)
		}

		klog.Infof("XidCriticalError: Xid=%d on Device=%s; marking device as unhealthy.", e.EventData, d.ID)
		s.notify(d)
	}
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

	placedDevices := make([]placedDevice, 0, len(devices))
	deviceIDToGiMap := make(map[string]uint32)
	deviceIDToCiMap := make(map[string]uint32)
	gpus := make([]gpuToWatch, 0, len(devices))

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
		placedDevices = append(placedDevices, placedDevice{parentUUID: uuid, device: d})

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

		gpus = append(gpus, gpuToWatch{
			uuid:   uuid,
			handle: gpu,
			mask:   eventMask & supportedEvents,
			device: d,
		})
	}

	watch := r.watch
	if watch == nil {
		watch = newHealthWatch()
	}
	sub := &healthSub{
		devices:           devices,
		parentToDeviceMap: groupByParent(placedDevices),
		deviceIDToGiMap:   deviceIDToGiMap,
		deviceIDToCiMap:   deviceIDToCiMap,
		xids:              xids,
		unhealthy:         unhealthy,
		stop:              stop,
	}
	if err := watch.subscribe(r.nvml, sub, gpus); err != nil {
		return err
	}
	defer watch.unsubscribe(sub)

	<-stop
	return nil
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
