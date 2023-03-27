package main

import (
	"bufio"
	"fmt"
	"k8s.io/klog/v2"
	"os/exec"
	"strings"
	"sync/atomic"

	"golang.org/x/net/context"
	"golang.org/x/sync/semaphore"

	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

type PreStartHook struct {
	preStartCmd     []string
	deviceInfos     map[string]*DevicePreStartInfo
	deviceInfosLock *semaphore.Weighted // Take this if you want to add/remove elements to/from deviceInfos. Not needed for operations on the values themselves.
}

type DevicePreStartInfo struct {
	lock             *semaphore.Weighted
	preStartFailures int32
}

func NewPreStartHook(cmd []string) *PreStartHook {
	klog.Info("Using PreStart hook")
	for i, a := range cmd {
		klog.Info("PreStartHook arg[%d] = %s\n", i, a)
	}

	return &PreStartHook{
		preStartCmd:     cmd,
		deviceInfos:     make(map[string]*DevicePreStartInfo),
		deviceInfosLock: semaphore.NewWeighted(1),
	}
}

func (hook *PreStartHook) TryHandlePreStartContainer(ctx context.Context, deviceIDs []string, allDevices rm.Devices, unhealthy chan<- *rm.Device) error {

	devices, err := findDevices(deviceIDs, allDevices)
	if err != nil {
		return err
	}

	return hook.tryHandlePreStartContainer(ctx, devices, unhealthy)
}

func (hook *PreStartHook) tryHandlePreStartContainer(ctx context.Context, devices []*rm.Device, unhealthyChan chan<- *rm.Device) error {
	logCtx := getLogContextFromDevices(devices)

	// check if any of the devices are already unhealthy
	for _, device := range devices {
		if device.Health != "Healthy" {
			return fmt.Errorf("Device %s has health status %s", deviceString(device), device.Health)
		}
	}

	// lock devices
	for _, device := range devices {

		preStartInfo, err := hook.getDevicePreStartInfo(ctx, device.ID, logCtx)
		if err != nil {
			return err
		}

		klog.Infof("PreStartHook(%s): locking device %s...\n", logCtx, deviceString(device))
		if err := preStartInfo.lockDevice(ctx); err != nil {
			return err
		}
		klog.Infof("PreStartHook(%s): locked device %s\n", logCtx, deviceString(device))
		defer preStartInfo.unlockDevice(ctx)
	}

	// run pre-start container command
	unhealthy, err := hook.runPreStartContainerCommand(ctx, devices)
	klog.Infof("PreStartHook(%s): pre start container command completed. unhealthy:%v command-err:%v context-err:%v\n", logCtx, unhealthy, err, ctx.Err())

	// if ctx deadline exceeded or command exited with non-zero exit code then add failure to all devices
	// when we fail 5 times then mark device as unhealthy
	if err == nil {
		err = ctx.Err()
	}

	if err != nil {
		klog.Infof("PreStartHook(%s): Error running pre-start command due to error: %v\n", logCtx, err)
		for _, device := range devices {
			preStartInfo, err := hook.getDevicePreStartInfo(ctx, device.ID, logCtx)
			if err != nil {
				klog.Infof("PreStartHook(%s): %v", logCtx, err)
				continue
			}

			failures := preStartInfo.incPreStartFailures()
			klog.Infof("PreStartHook(%s): Failure count for device %s: %d\n", logCtx, device.ID, failures)
			if failures > 4 {
				markUnhealthy(device, unhealthyChan, logCtx)
			}
		}
		return err
	}

	// if no timeout and exit code 0 then reset all pre-start failure counts
	for _, device := range devices {
		if preStartInfo, err := hook.getDevicePreStartInfo(ctx, device.ID, logCtx); err == nil {
			preStartInfo.resetPreStartFailures()
		} else {
			klog.Infof("PreStartHook(%s): Unable to failure reset count. Device not found: %v\n", logCtx, err)
		}

	}

	// mark reported devices as unhealthy
	for _, uuid := range unhealthy {
		if device, err := findDevice(uuid, devices); err == nil {
			markUnhealthy(device, unhealthyChan, logCtx)
		} else {
			klog.Infof("PreStartHook(%s): Unable to mark device as unhealty. Device not found: %v\n", logCtx, err)
		}
	}

	if len(unhealthy) > 0 {
		return fmt.Errorf("Found unhealthy devices %s", strings.Join(unhealthy, " "))
	}
	return nil
}

func (hook *PreStartHook) runPreStartContainerCommand(ctx context.Context, devices []*rm.Device) (unhealthy []string, err error) {
	if len(devices) == 0 {
		return
	}
	logCtx := getLogContextFromDevices(devices)

	cmd := hook.preStartCmd[0]
	args := hook.preStartCmd[1:]
	for _, device := range devices {
		uuid := strings.TrimPrefix(device.ID, "GPU-")
		args = append(args, uuid)
	}

	klog.Infof("PreStartHook(%s): creating command %s %v\n", logCtx, cmd, args)
	cmdCtx := exec.CommandContext(ctx, cmd, args...)
	klog.Infof("PreStartHook(%s): created command\n", logCtx)

	stdout, err := cmdCtx.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("error getting stdout pipe: %v", err)
		return
	}
	stdoutDone := make(chan struct{})
	go func() {
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			line := s.Text()
			fmt.Printf("PreStartHook(%s)::out: %s\n", logCtx, line)
		}
		stdoutDone <- struct{}{}
	}()
	klog.Infof("PreStartHook(%s): got stdout\n", logCtx)

	stderr, err := cmdCtx.StderrPipe()
	if err != nil {
		err = fmt.Errorf("error getting stderr pipe: %v", err)
		return
	}
	stderrDone := make(chan []string)
	go func() {
		s := bufio.NewScanner(stderr)
		uuids := []string{}
		for s.Scan() {
			line := s.Text()
			klog.Infof("PreStartHook(%s)::err: %s\n", logCtx, line)
			uuids = append(uuids, line)
		}
		stderrDone <- uuids
	}()
	klog.Infof("PreStartHook(%s): got stderr\n", logCtx)

	err = cmdCtx.Start()
	if err != nil {
		err = fmt.Errorf("error starting: %v", err)
		return
	}
	klog.Infof("PreStartHook(%s): started command\n", logCtx)

	err = cmdCtx.Wait()
	if err != nil {
		err = fmt.Errorf("command failed to run: %v", err)
	}
	klog.Infof("PreStartHook(%s): command exited with code %d\n", logCtx, cmdCtx.ProcessState.ExitCode())

	<-stdoutDone
	klog.Infof("PreStartHook(%s): stdout closed\n", logCtx)

	unhealthy = <-stderrDone
	klog.Infof("PreStartHook(%s): stderr closed\n", logCtx)

	return
}

func markUnhealthy(device *rm.Device, unhealthyChan chan<- *rm.Device, logCtx string) {
	klog.Infof("PreStartHook(%s): Marking device %s as unhealthy\n", logCtx, deviceString(device))
	unhealthyChan <- device
}

func getLogContextFromDeviceIDs(deviceIDs []string) string {
	return strings.ReplaceAll(strings.Join(deviceIDs, "-"), "nvidia", "")
}

func getLogContextFromDevices(devices []*rm.Device) string {
	ids := make([]string, len(devices))
	for i, v := range devices {
		ids[i] = v.ID
	}
	return getLogContextFromDeviceIDs(ids)
}

func (hook *PreStartHook) getDevicePreStartInfo(ctx context.Context, deviceId string, logCtx string) (*DevicePreStartInfo, error) {
	err := hook.deviceInfosLock.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}

	defer hook.deviceInfosLock.Release(1)
	info, ok := hook.deviceInfos[deviceId]
	if !ok {
		klog.Infof("PreStartHook(%s): Creating device PreStart info for %q\n", logCtx, deviceId)
		info = newDevicePreStartInfo()
		hook.deviceInfos[deviceId] = info
	}
	return info, nil
}

func findDevice(deviceId string, allDevices []*rm.Device) (*rm.Device, error) {
	for _, device := range allDevices {
		if device.ID == deviceId {
			return device, nil
		}
	}
	return nil, fmt.Errorf("Device %q not found", deviceId)
}

func findDevices(deviceIds []string, allDevices rm.Devices) ([]*rm.Device, error) {
	result := make([]*rm.Device, len(deviceIds))
	for i, deviceId := range deviceIds {
		result[i] = allDevices.GetByID(deviceId)
		if result[i] == nil {
			return nil, fmt.Errorf("Device %q not found", deviceId)
		}
	}
	return result, nil
}

func deviceString(device *rm.Device) string {
	return fmt.Sprintf("%q %s", device.ID, strings.Join(device.Paths, " "))
}

func newDevicePreStartInfo() *DevicePreStartInfo {
	return &DevicePreStartInfo{
		lock:             semaphore.NewWeighted(1),
		preStartFailures: 0,
	}
}

func (d *DevicePreStartInfo) lockDevice(ctx context.Context) error {
	return d.lock.Acquire(ctx, 1)
}

func (d *DevicePreStartInfo) unlockDevice(ctx context.Context) {
	d.lock.Release(1)
}

// increments pre-start failure count and return the current value
func (d *DevicePreStartInfo) incPreStartFailures() int32 {
	return atomic.AddInt32(&d.preStartFailures, 1)
}

// resets pre-start failure back count to zero
func (d *DevicePreStartInfo) resetPreStartFailures() {
	atomic.StoreInt32(&d.preStartFailures, 0)
}
