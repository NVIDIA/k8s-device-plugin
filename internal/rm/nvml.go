/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY Type, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/dl"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

const (
	nvmlXidCriticalError = nvml.EventTypeXidCriticalError
)

type nvmlEvent struct {
	UUID              *string
	GpuInstanceID     *uint
	ComputeInstanceID *uint
	Etype             uint64
	Edata             uint64
}

func uintPtr(c uint32) *uint {
	i := uint(c)
	return &i
}

func nvmlLookupSymbol(symbol string) error {
	lib := dl.New("libnvidia-ml.so.1", dl.RTLD_LAZY|dl.RTLD_GLOBAL)
	if lib == nil {
		return fmt.Errorf("error instantiating DynamicLibrary for NVML")
	}
	err := lib.Open()
	if err != nil {
		return fmt.Errorf("error opening DynamicLibrary for NVML: %v", err)
	}
	defer lib.Close()
	return lib.Lookup(symbol)
}

func nvmlNewEventSet() nvml.EventSet {
	set, _ := nvml.EventSetCreate()
	return set
}

func nvmlDeleteEventSet(es nvml.EventSet) {
	es.Free()
}

func nvmlWaitForEvent(es nvml.EventSet, timeout uint) (nvmlEvent, error) {
	data, ret := es.Wait(uint32(timeout))
	if ret != nvml.SUCCESS {
		return nvmlEvent{}, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	uuid, ret := data.Device.GetUUID()
	if ret != nvml.SUCCESS {
		return nvmlEvent{}, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	isMig, ret := data.Device.IsMigDeviceHandle()
	if ret != nvml.SUCCESS {
		return nvmlEvent{}, fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	if !isMig {
		data.GpuInstanceId = 0xFFFFFFFF
		data.ComputeInstanceId = 0xFFFFFFFF
	}

	event := nvmlEvent{
		UUID:              &uuid,
		Etype:             uint64(data.EventType),
		Edata:             uint64(data.EventData),
		GpuInstanceID:     uintPtr(data.GpuInstanceId),
		ComputeInstanceID: uintPtr(data.ComputeInstanceId),
	}

	return event, nil
}

func nvmlRegisterEventForDevice(es nvml.EventSet, event int, uuid string) error {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("%v", nvml.ErrorString(ret))
	}

	for i := 0; i < count; i++ {
		d, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("%v", nvml.ErrorString(ret))
		}

		duuid, ret := d.GetUUID()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("%v", nvml.ErrorString(ret))
		}

		if duuid != uuid {
			continue
		}

		ret = d.RegisterEvents(uint64(event), es)
		if ret != nvml.SUCCESS {
			return fmt.Errorf("%v", nvml.ErrorString(ret))
		}

		return nil
	}

	return fmt.Errorf("nvml: device not found")
}
