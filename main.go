// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"log"
	"syscall"

	"github.com/NVIDIA/nvidia-docker/src/nvml"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

func main() {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.Println("Failed to start nvml with error:", err)
		select{}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	log.Println("Fetching devices")
	if len(getDevices()) == 0 {
		log.Println("No devices found.")
		select{}
	}

	log.Println("Starting FS watcher")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	check(err)
	defer watcher.Close()

	log.Println("Starting OS watcher")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *NvidiaDevicePlugin

L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = NewNvidiaDevicePlugin()
			if err := devicePlugin.Serve(); err != nil {
				log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting")
				restart = true
			default:
				log.Printf("Received signal \"%v\", shutting down", s)
				devicePlugin.Stop()
				break L
			}
		}
	}
}
