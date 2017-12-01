// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"log"
	"os"
	"os/signal"
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

	if len(getDevices()) == 0 {
		log.Println("No devices found")
		select{}
	}

	devicePlugin := NewNvidiaDevicePlugin()
	devicePlugin.Serve()

	watcher, err := fsnotify.NewWatcher()
	check(err)
	defer watcher.Close()
	err = watcher.Add(pluginapi.DevicePluginPath)
	check(err)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

L:
	for {
		restart := false
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
		if restart {
			devicePlugin.Stop()
			devicePlugin = NewNvidiaDevicePlugin()
			devicePlugin.Serve()
		}
	}
}
