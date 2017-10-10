// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/NVIDIA/nvidia-docker/src/nvml"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

const (
	resourceName = "nvidia.com/gpu"
	serverSock   = pluginapi.DevicePluginPath + "nvidia.sock"
)

type NvidiaDevicePlugin struct {
	devs   []*pluginapi.Device
	socket string

	stop   chan interface{}
	update chan []*pluginapi.Device

	server *grpc.Server
}

// NewNvidiaDevicePlugin returns an initialized NvidiaDevicePlugin.
func NewNvidiaDevicePlugin() *NvidiaDevicePlugin {
	return &NvidiaDevicePlugin{
		devs:   getDevices(),
		socket: serverSock,

		stop:   make(chan interface{}),
		update: make(chan []*pluginapi.Device),
	}
}

// Start starts the gRPC server of the device plugin
func (m *NvidiaDevicePlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)
	// Wait till grpc server is ready.
	for i := 0; i < 10; i++ {
		services := m.server.GetServiceInfo()
		if len(services) > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	go m.HealthCheck()

	return nil
}

// Stop stops the gRPC server
func (m *NvidiaDevicePlugin) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	close(m.stop)

	return m.cleanup()
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *NvidiaDevicePlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		return err
	}
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the Update call
func (m *NvidiaDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case updated := <-m.update:
			// FIXME: submit upstream patch.
			m.devs = updated
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

// Update allows the device plugin to send new devices through ListAndWatch
func (m *NvidiaDevicePlugin) Update(devs []*pluginapi.Device) {
	m.update <- devs
}

// Allocate which return list of devices.
func (m *NvidiaDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devs := m.devs
	var response pluginapi.AllocateResponse

	for i, id := range r.DevicesIDs {
		if !deviceExists(devs, id) {
			return nil, fmt.Errorf("Invalid allocation request: unknown device: %s", id)
		}

		devRuntime := new(pluginapi.DeviceRuntimeSpec)
		devRuntime.ID = id
		// Only add env vars to the first device.
		// Will be fixed by: https://github.com/kubernetes/kubernetes/pull/53031
		if i == 0 {
			devRuntime.Envs = make(map[string]string)
			devRuntime.Envs["NVIDIA_VISIBLE_DEVICES"] = strings.Join(r.DevicesIDs, ",")
		}

		response.Spec = append(response.Spec, devRuntime)
	}

	return &response, nil
}

func (m *NvidiaDevicePlugin) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *NvidiaDevicePlugin) HealthCheck() {
	eventSet := nvml.NewEventSet()
	defer nvml.DeleteEventSet(eventSet)

	err := nvml.RegisterEvent(eventSet, nvml.XidCriticalError)
	check(err)

	for {
		select {
		case <-m.stop:
			return
		default:
			// FIXME: there is a race condition if another goroutine calls m.Update concurrently.
			devs := m.devs
			healthy := checkXIDs(eventSet, devs)
			if !healthy {
				m.Update(devs)
			}
		}
	}
}

func (m *NvidiaDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		log.Printf("Could not start device plugin: %s", err)
		return err
	}
	log.Println("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, resourceName)
	if err != nil {
		log.Printf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	log.Println("Registered device plugin with Kubelet")

	return nil
}
