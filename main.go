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
	"log"
	"os"
	"syscall"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

// PluginResourceManager wraps an underlying ResourceManager with the set of
// parameters we need to pass to the instantiation of an NvidiaDevicePlugin for
// a specific resource type.
type PluginResourceManager struct {
	ResourceManager
	AllocateEnvvar string
	Socket         string
}

// resources maps a set of resource types (e.g. nvidia.com/gpu) to the set of
// PluginResourceManagers needed for the NvidiaDevicePlugin of that type.
var resources = map[string]PluginResourceManager{
	"nvidia.com/gpu": {
		NewGpuDeviceManager(),
		"NVIDIA_VISIBLE_DEVICES",
		pluginapi.DevicePluginPath + "nvidia.sock",
	},
}

func main() {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.Printf("Failed to initialize NVML: %s.", err)
		log.Printf("If this is a GPU node, did you set the docker default runtime to `nvidia`?")
		log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
		log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")

		select {}
	}
	defer func() { log.Println("Shutdown of NVML returned:", nvml.Shutdown()) }()

	log.Println("Fetching devices.")
	if len(resources["nvidia.com/gpu"].Devices()) == 0 {
		log.Println("No devices found. Waiting indefinitely.")
		select {}
	}

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Create a map of plugins keyed by the same resource types present in
	// 'resources'. For now, we just initialize each of these plugins to
	// 'nil'. They will be properly initialized to a real plugin in the
	// infinite loop below.
	plugins := map[string]*NvidiaDevicePlugin{}
	for r := range resources {
		plugins[r] = nil
	}

restart:
	// Loop through all plugins, idempotently stopping them,
	// initializing them to a new plugin instance, and then starting
	// them. If even one plugin fails to start properly, go back to the
	// top of the loop and try starting them all again.
	for r := range resources {
		plugins[r].Stop()

		// If there are no devices associated with resource type 'r',
		// don't create a plugin for it or start it.
		if len(resources[r].Devices()) == 0 {
			continue
		}

		// Create a plugin for resource type 'r' and start up its gRPC
		// server to connect with the kubelet.
		plugins[r] = NewNvidiaDevicePlugin(r, resources[r], resources[r].AllocateEnvvar, resources[r].Socket)
		if err := plugins[r].Serve(); err != nil {
			log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			log.Printf("You can check the prerequisites at: https://github.com/NVIDIA/k8s-device-plugin#prerequisites")
			log.Printf("You can learn how to set the runtime at: https://github.com/NVIDIA/k8s-device-plugin#quick-start")
			goto restart
		}
	}

events:
	// Start an infinite loop, waiting for several indicators to either log
	// some messages, trigger a restart of the plugins, or exit the program.
	for {
		select {
		// Detect a kubelet restart by watching for a newly created
		// 'pluginapi.KubeletSocket' file. When this occurs, restart this loop,
		// reinitializing all of the plugins in the process.
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				goto restart
			}

		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		// Watch for any signals from the OS. On SIGHUP, restart this loop,
		// reinitializing all of the plugins in the process. On all other
		// signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				goto restart
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				for _, p := range plugins {
					p.Stop()
				}
				break events
			}
		}
	}
}
