// Copyright (c) 2017, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"strings"

	"github.com/NVIDIA/nvidia-docker/src/nvidia"
	"github.com/NVIDIA/nvidia-docker/src/nvml"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha1"
)

func getDevices() []*pluginapi.Device {
	nvdevs, err := nvidia.LookupDevices()
	check(err)

	var devs []*pluginapi.Device
	for _, d := range nvdevs {
		devs = append(devs, &pluginapi.Device{
			ID:     d.UUID,
			Health: pluginapi.Healthy,
		})
	}

	return devs
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

func checkXIDs(xidEventSet nvml.EventSet, devs []*pluginapi.Device) bool {
	e, err := nvml.WaitForEvent(xidEventSet, 5000)
	if err != nil && strings.Contains(err.Error(), "Timeout") {
		return true
	}

	if e.UUID == nil || len(*e.UUID) == 0 {
		for _, d := range devs {
			d.Health = pluginapi.Unhealthy
		}
		return false
	}

	for _, d := range devs {
		if d.ID == *e.UUID {
			d.Health = pluginapi.Unhealthy
			break
		}
	}

	return false
}
