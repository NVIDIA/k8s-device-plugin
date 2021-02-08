// Copyright (c) 2021, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"fmt"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// MIGCapableDevices stores information about all devices on the node
type MIGCapableDevices struct {
	// devicesMap holds a list of devices, separated by whether they have MigEnabled or not
	devicesMap map[bool][]*nvml.Device
}

// NewMIGCapableDevices creates a new MIGCapableDevices struct and returns a pointer to it.
func NewMIGCapableDevices() *MIGCapableDevices {
	return &MIGCapableDevices{
		devicesMap: nil, // Is initialized on first use
	}
}

func (devices *MIGCapableDevices) getDevicesMap() (map[bool][]*nvml.Device, error) {
	if devices.devicesMap == nil {
		n, err := nvml.GetDeviceCount()
		if err != nil {
			return nil, err
		}

		migEnabledDevicesMap := make(map[bool][]*nvml.Device)
		for i := uint(0); i < n; i++ {
			d, err := nvml.NewDeviceLite(i)
			if err != nil {
				return nil, err
			}

			isMigEnabled, err := d.IsMigEnabled()
			if err != nil {
				return nil, err
			}

			migEnabledDevicesMap[isMigEnabled] = append(migEnabledDevicesMap[isMigEnabled], d)
		}

		devices.devicesMap = migEnabledDevicesMap
	}
	return devices.devicesMap, nil
}

// GetDevicesWithMigEnabled returns a list of devices with migEnabled=true
func (devices *MIGCapableDevices) GetDevicesWithMigEnabled() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}
	return devicesMap[true], nil
}

// GetDevicesWithMigDisabled returns a list of devices with migEnabled=false
func (devices *MIGCapableDevices) GetDevicesWithMigDisabled() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}
	return devicesMap[false], nil
}

// AssertAllMigEnabledDevicesAreValid ensures that all devices with migEnabled=true are valid. This means:
// * The have at least 1 mig devices associated with them
// Returns nill if the device is valid, or an error if these are not valid
func (devices *MIGCapableDevices) AssertAllMigEnabledDevicesAreValid() error {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return err
	}

	for _, d := range devicesMap[true] {
		migs, err := d.GetMigDevices()
		if err != nil {
			return err
		}
		if len(migs) == 0 {
			return fmt.Errorf("No MIG devices associated with %v", d)
		}
	}
	return nil
}

// GetAllMigDevices returns a list of all MIG devices.
func (devices *MIGCapableDevices) GetAllMigDevices() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}

	var migs []*nvml.Device
	for _, d := range devicesMap[true] {
		devs, err := d.GetMigDevices()
		if err != nil {
			return nil, err
		}
		migs = append(migs, devs...)
	}
	return migs, nil
}
