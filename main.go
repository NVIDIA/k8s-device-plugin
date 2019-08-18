// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"flag"
	"os"
	"syscall"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/fsnotify/fsnotify"
	"k8s.io/client-go/informers"
	"k8s.io/klog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	nodeName      = flag.String("node-name", os.Getenv("NODE_NAME"), "Set node name for this node")
	topoScheduler = flag.String("topo-sched-endpoint", os.Getenv("TOPO_SCHED_ENDPOINT"), "The topology scheduler endpoint to register")
)

func main() {
	klog.InitFlags(nil)
	klog.Infoln("Loading NVML")
	flag.Parse()
	if err := nvml.Init(); err != nil {
		klog.Infof("Failed to initialize NVML: %s.", err)
		klog.Infof("If this is a GPU node, did you set the docker default runtime to `nvidia`?")
		klog.Infof("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		klog.Infof("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")

		select {}
	}
	defer func() { klog.Infoln("Shutdown of NVML returned:", nvml.Shutdown()) }()

	klog.Infoln("Fetching devices.")
	if len(getDevices()) == 0 {
		klog.Infoln("No devices found. Waiting indefinitely.")
		select {}
	}

	klog.Infoln("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		klog.Infoln("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	klog.Infoln("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *NvidiaDevicePlugin
	var stopCh = make(chan struct{})

L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = NewNvidiaDevicePlugin(*nodeName)
			if err := devicePlugin.buildPciDeviceTree(); err != nil {
				klog.Fatalf("Failed to build PCI device tree: %v", err)
			}
			updateTree(devicePlugin.root, true)
			if klog.V(2) {
				printDeviceTree(devicePlugin.root)
			}
			if err := devicePlugin.Serve(); err != nil {
				klog.Infoln("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
				klog.Infof("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
				klog.Infof("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
			} else {
				restart = false
			}
			kubeClient := kubeInit()
			informerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
			controller, err := newController(kubeClient, informerFactory, stopCh)
			if err != nil {
				klog.Fatalf("Failed to start due to %v", err)
			}
			controller.Start(informerFactory, stopCh)

			devicePlugin.RegisterToSched(*topoScheduler)
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				klog.Infof("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			klog.Infof("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				klog.Infoln("Received SIGHUP, restarting.")
				restart = true
			default:
				klog.Infof("Received signal \"%v\", shutting down.", s)
				devicePlugin.Stop()
				close(stopCh)
				break L
			}
		}
	}
}
