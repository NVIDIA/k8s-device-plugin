/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NVIDIA/go-gpuallocator/gpuallocator"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Constants to represent the various device list strategies
const (
	DeviceListStrategyEnvvar       = "envvar"
	DeviceListStrategyVolumeMounts = "volume-mounts"
)

// Constants to represent the various device id strategies
const (
	DeviceIDStrategyUUID  = "uuid"
	DeviceIDStrategyIndex = "index"
)

// Constants for use by the 'volume-mounts' device list strategy
const (
	deviceListAsVolumeMountsHostPath          = "/dev/null"
	deviceListAsVolumeMountsContainerPathRoot = "/var/run/nvidia-container-devices"
)

// NvidiaDevicePlugin implements the Kubernetes device plugin API
type NvidiaDevicePlugin struct {
	rm               rm.ResourceManager
	config           *spec.Config
	deviceListEnvvar string
	allocatePolicy   gpuallocator.Policy
	socket           string

	server        *grpc.Server
	cachedDevices rm.Devices
	health        chan *rm.Device
	stop          chan interface{}
}

// NewNvidiaDevicePlugin returns an initialized NvidiaDevicePlugin
func NewNvidiaDevicePlugin(config *spec.Config, resourceManager rm.ResourceManager, allocatePolicy gpuallocator.Policy) *NvidiaDevicePlugin {
	_, name := resourceManager.Resource().Split()

	return &NvidiaDevicePlugin{
		rm:               resourceManager,
		config:           config,
		deviceListEnvvar: "NVIDIA_VISIBLE_DEVICES",
		allocatePolicy:   allocatePolicy,
		socket:           pluginapi.DevicePluginPath + "nvidia-" + name + ".sock",

		// These will be reinitialized every
		// time the plugin server is restarted.
		cachedDevices: nil,
		server:        nil,
		health:        nil,
		stop:          nil,
	}
}

func (plugin *NvidiaDevicePlugin) initialize() {
	plugin.cachedDevices = plugin.Devices()
	plugin.server = grpc.NewServer([]grpc.ServerOption{}...)
	plugin.health = make(chan *rm.Device)
	plugin.stop = make(chan interface{})
}

func (plugin *NvidiaDevicePlugin) cleanup() {
	close(plugin.stop)
	plugin.cachedDevices = nil
	plugin.server = nil
	plugin.health = nil
	plugin.stop = nil
}

// Devices returns the full set of devices associated with the plugin.
func (plugin *NvidiaDevicePlugin) Devices() rm.Devices {
	return plugin.cachedDevices
}

// Start starts the gRPC server, registers the device plugin with the Kubelet,
// and starts the device healthchecks.
func (plugin *NvidiaDevicePlugin) Start() error {
	plugin.initialize()

	err := plugin.Serve()
	if err != nil {
		log.Printf("Could not start device plugin for '%s': %s", plugin.rm.Resource(), err)
		plugin.cleanup()
		return err
	}
	log.Printf("Starting to serve '%s' on %s", plugin.rm.Resource(), plugin.socket)

	err = plugin.Register()
	if err != nil {
		log.Printf("Could not register device plugin: %s", err)
		plugin.Stop()
		return err
	}
	log.Printf("Registered device plugin for '%s' with Kubelet", plugin.rm.Resource())

	go plugin.rm.CheckHealth(plugin.stop, plugin.cachedDevices, plugin.health)

	return nil
}

// Stop stops the gRPC server.
func (plugin *NvidiaDevicePlugin) Stop() error {
	if plugin == nil || plugin.server == nil {
		return nil
	}
	log.Printf("Stopping to serve '%s' on %s", plugin.rm.Resource(), plugin.socket)
	plugin.server.Stop()
	if err := os.Remove(plugin.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	plugin.cleanup()
	return nil
}

// Serve starts the gRPC server of the device plugin.
func (plugin *NvidiaDevicePlugin) Serve() error {
	os.Remove(plugin.socket)
	sock, err := net.Listen("unix", plugin.socket)
	if err != nil {
		return err
	}

	pluginapi.RegisterDevicePluginServer(plugin.server, plugin)

	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.Printf("Starting GRPC server for '%s'", plugin.rm.Resource())
			err := plugin.server.Serve(sock)
			if err == nil {
				break
			}

			log.Printf("GRPC server for '%s' crashed with error: %v", plugin.rm.Resource(), err)

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				// quit
				log.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", plugin.rm.Resource())
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount++
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := plugin.dial(plugin.socket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (plugin *NvidiaDevicePlugin) Register() error {
	conn, err := plugin.dial(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.socket),
		ResourceName: string(plugin.rm.Resource()),
		Options: &pluginapi.DevicePluginOptions{
			GetPreferredAllocationAvailable: true,
		},
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// GetDevicePluginOptions returns the values of the optional settings for this plugin
func (plugin *NvidiaDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	options := &pluginapi.DevicePluginOptions{
		GetPreferredAllocationAvailable: true,
	}
	return options, nil
}

// ListAndWatch lists devices and update that list according to the health status
func (plugin *NvidiaDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.apiDevices()})

	for {
		select {
		case <-plugin.stop:
			return nil
		case d := <-plugin.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			log.Printf("'%s' device marked unhealthy: %s", plugin.rm.Resource(), d.ID)
			s.Send(&pluginapi.ListAndWatchResponse{Devices: plugin.apiDevices()})
		}
	}
}

// GetPreferredAllocation returns the preferred allocation from the set of devices specified in the request
func (plugin *NvidiaDevicePlugin) GetPreferredAllocation(ctx context.Context, r *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}
	for _, req := range r.ContainerRequests {
		devices, err := plugin.getPreferredAllocationDevices(req.AvailableDeviceIDs, req.MustIncludeDeviceIDs, int(req.AllocationSize))
		if err != nil {
			return nil, fmt.Errorf("error getting list of preferred allocation devices: %v", err)
		}

		resp := &pluginapi.ContainerPreferredAllocationResponse{
			DeviceIDs: devices,
		}

		response.ContainerResponses = append(response.ContainerResponses, resp)
	}
	return response, nil
}

func (plugin *NvidiaDevicePlugin) getPreferredAllocationDevices(available, required []string, size int) ([]string, error) {
	var devices []string

	// If an allocation policy is set and none of the available device IDs has
	// attributes (i.e. is replicated), then use the gpuallocator to get the
	// preferred set of allocated devices.
	if plugin.allocatePolicy != nil && !rm.AnnotatedIDs(available).AnyHasAnnotations() {
		available, err := gpuallocator.NewDevicesFrom(available)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve list of available devices: %v", err)
		}

		required, err := gpuallocator.NewDevicesFrom(required)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve list of required devices: %v", err)
		}

		allocated := plugin.allocatePolicy.Allocate(available, required, size)

		for _, device := range allocated {
			devices = append(devices, device.UUID)
		}

		return devices, nil
	}

	// Otherwise return a list of sorted devices, being sure to include any
	// required ones at the front. Sorting them ensures that devices from the
	// same GPU (in the case of sharing) are chosen first before moving on to
	// the next one (i.e we follow a packed sharing strategy rather than a
	// distributed one).
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}
	for _, a := range available {
		if !requiredSet[a] {
			devices = append(devices, a)
		}
	}
	sort.Strings(devices)
	devices = append(required, devices...)
	if len(devices) < size {
		return nil, fmt.Errorf("not enough available devices to satisfy allocation")
	}

	return devices[:size], nil
}

// Allocate which return list of devices.
func (plugin *NvidiaDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		for _, id := range req.DevicesIDs {
			if !plugin.cachedDevices.Contains(id) {
				return nil, fmt.Errorf("invalid allocation request for '%s': unknown device: %s", plugin.rm.Resource(), id)
			}
		}

		response := pluginapi.ContainerAllocateResponse{}

		ids := req.DevicesIDs
		deviceIDs := plugin.deviceIDsFromAnnotatedDeviceIDs(ids)

		if plugin.config.Flags.Plugin.DeviceListStrategy == DeviceListStrategyEnvvar {
			response.Envs = plugin.apiEnvs(plugin.deviceListEnvvar, deviceIDs)
		}
		if plugin.config.Flags.Plugin.DeviceListStrategy == DeviceListStrategyVolumeMounts {
			response.Envs = plugin.apiEnvs(plugin.deviceListEnvvar, []string{deviceListAsVolumeMountsContainerPathRoot})
			response.Mounts = plugin.apiMounts(deviceIDs)
		}
		if plugin.config.Flags.Plugin.PassDeviceSpecs {
			response.Devices = plugin.apiDeviceSpecs(plugin.config.Flags.NvidiaDriverRoot, ids)
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	return &responses, nil
}

// PreStartContainer is unimplemented for this plugin
func (plugin *NvidiaDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func (plugin *NvidiaDevicePlugin) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

func (plugin *NvidiaDevicePlugin) deviceIDsFromAnnotatedDeviceIDs(ids []string) []string {
	var deviceIDs []string
	if plugin.config.Flags.Plugin.DeviceIDStrategy == DeviceIDStrategyUUID {
		deviceIDs = rm.AnnotatedIDs(ids).GetIDs()
	}
	if plugin.config.Flags.Plugin.DeviceIDStrategy == DeviceIDStrategyIndex {
		deviceIDs = plugin.cachedDevices.Subset(ids).GetIndices()
	}
	return deviceIDs
}

func (plugin *NvidiaDevicePlugin) apiDevices() []*pluginapi.Device {
	return plugin.cachedDevices.GetPluginDevices()
}

func (plugin *NvidiaDevicePlugin) apiEnvs(envvar string, deviceIDs []string) map[string]string {
	return map[string]string{
		envvar: strings.Join(deviceIDs, ","),
	}
}

func (plugin *NvidiaDevicePlugin) apiMounts(deviceIDs []string) []*pluginapi.Mount {
	var mounts []*pluginapi.Mount

	for _, id := range deviceIDs {
		mount := &pluginapi.Mount{
			HostPath:      deviceListAsVolumeMountsHostPath,
			ContainerPath: filepath.Join(deviceListAsVolumeMountsContainerPathRoot, id),
		}
		mounts = append(mounts, mount)
	}

	return mounts
}

func (plugin *NvidiaDevicePlugin) apiDeviceSpecs(driverRoot string, ids []string) []*pluginapi.DeviceSpec {
	var specs []*pluginapi.DeviceSpec

	paths := []string{
		"/dev/nvidiactl",
		"/dev/nvidia-uvm",
		"/dev/nvidia-uvm-tools",
		"/dev/nvidia-modeset",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			spec := &pluginapi.DeviceSpec{
				ContainerPath: p,
				HostPath:      filepath.Join(driverRoot, p),
				Permissions:   "rw",
			}
			specs = append(specs, spec)
		}
	}

	for _, p := range plugin.cachedDevices.Subset(ids).GetPaths() {
		spec := &pluginapi.DeviceSpec{
			ContainerPath: p,
			HostPath:      filepath.Join(driverRoot, p),
			Permissions:   "rw",
		}
		specs = append(specs, spec)
	}

	return specs
}
