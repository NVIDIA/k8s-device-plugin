// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/gpucloud/gohwloc/topology"
	"google.golang.org/grpc"
	"k8s.io/klog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	resourceName           = "nvidia.com/gpu-topo"
	serverSock             = pluginapi.DevicePluginPath + "nvidia-topo.sock"
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
	allHealthChecks        = "xids"
)

// NvidiaDevicePlugin implements the Kubernetes device plugin API
type NvidiaDevicePlugin struct {
	nodeName string
	devs     []*nvml.Device
	socket   string

	stop   chan interface{}
	health chan *pluginapi.Device

	server *grpc.Server

	root *pciDevice

	topo *topology.Topology
}

// NewNvidiaDevicePlugin returns an initialized NvidiaDevicePlugin
func NewNvidiaDevicePlugin(name string) *NvidiaDevicePlugin {
	if name == "" {
		klog.Fatalf("Failed due to undefined node name")
	}
	return &NvidiaDevicePlugin{
		nodeName: name,
		devs:     getDevices(),
		socket:   serverSock,

		stop:   make(chan interface{}),
		health: make(chan *pluginapi.Device),
	}
}

func (m *NvidiaDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		klog.Errorf("dail error: %v", err)
		return nil, err
	}

	return c, nil
}

// Start starts the gRPC server of the device plugin
func (m *NvidiaDevicePlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		klog.Errorf("net.Listen error: %v", err)
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	go m.healthcheck()

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
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

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

// ListAndWatch lists devices and update that list according to the health status
func (m *NvidiaDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	klog.Infof("Started ListAndWatch for GPU: %v", len(m.getPluginDevices()))
	if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: m.getPluginDevices()}); err != nil {
		klog.Errorf("Send failed: %v", err)
	}

	for {
		select {
		case <-m.stop:
			klog.Info("m.stop")
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			klog.Warningf("Device %v is unhealthy", d)
			d.Health = pluginapi.Unhealthy
			if err := s.Send(&pluginapi.ListAndWatchResponse{Devices: m.getPluginDevices()}); err != nil {
				klog.Errorf("Send failed: %v", err)
			}
		}
	}
}

func (m *NvidiaDevicePlugin) unhealthy(dev *pluginapi.Device) {
	m.health <- dev
}

// Allocate which return list of devices.
func (m *NvidiaDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devs := m.devs
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		topoDevs := m.findBestDevice(resourceName, len(req.DevicesIDs))
		if len(devs) == 0 {
			topoDevs = req.DevicesIDs
		}
		response := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{
				"NVIDIA_VISIBLE_DEVICES": strings.Join(topoDevs, ","),
			},
			Annotations: map[string]string{
				resourceName: strings.Join(topoDevs, ","),
			},
		}

		for _, id := range req.DevicesIDs {
			if !deviceExists(devs, id) {
				return nil, fmt.Errorf("invalid allocation request: unknown device: %s", id)
			}
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	klog.Infof("Allocate response: %#v", responses)
	return &responses, nil
}

func (m *NvidiaDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *NvidiaDevicePlugin) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *NvidiaDevicePlugin) healthcheck() {
	disableHealthChecks := strings.ToLower(os.Getenv(envDisableHealthChecks))
	if disableHealthChecks == "all" {
		disableHealthChecks = allHealthChecks
	}

	ctx, cancel := context.WithCancel(context.Background())

	var xids chan *pluginapi.Device
	if !strings.Contains(disableHealthChecks, "xids") {
		xids = make(chan *pluginapi.Device)
		go watchXIDs(ctx, m.getPluginDevices(), xids)
	}

	for {
		select {
		case <-m.stop:
			cancel()
			return
		case dev := <-xids:
			m.unhealthy(dev)
		}
	}
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *NvidiaDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		klog.Infof("Could not start device plugin: %s", err)
		return err
	}
	klog.Infoln("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, resourceName)
	if err != nil {
		klog.Infof("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	klog.Infoln("Registered device plugin with Kubelet")

	return nil
}

func (m *NvidiaDevicePlugin) getPluginDevices() []*pluginapi.Device {
	var devs = []*pluginapi.Device{}
	for _, d := range m.devs {
		devs = append(devs, &pluginapi.Device{
			ID:     d.UUID,
			Health: pluginapi.Healthy,
		})
	}
	return devs
}
