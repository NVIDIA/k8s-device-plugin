// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/NVIDIA/nvidia-docker/src/nvidia"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

func exit() {
	if err := recover(); err != nil {
		if _, ok := err.(runtime.Error); ok {
			log.Println(err)
		}
		if os.Getenv("NV_DEBUG") != "" {
			log.Printf("%s", debug.Stack())
		}
		os.Exit(1)
	}

	os.Exit(0)
}

func main() {
	defer exit()

	log.Println("Loading NVIDIA management library")
	check(nvidia.Init())
	defer func() { check(nvidia.Shutdown()) }()

	// Should it be in the device plugin Serve?
	if len(getDevices()) == 0 {
		log.Println("No devices found. Looping")
		select {}
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
				log.Printf("Received signal %d, shutting down", s)
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
