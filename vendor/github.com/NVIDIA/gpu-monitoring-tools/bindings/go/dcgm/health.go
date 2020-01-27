package dcgm

/*
#include "dcgm_agent.h"
#include "dcgm_structs.h"
*/
import "C"
import (
	"fmt"
	"math/rand"
	"unsafe"
)

type SystemWatch struct {
	Type   string
	Status string
	Error  string
}

type DeviceHealth struct {
	GPU     uint
	Status  string
	Watches []SystemWatch
}

func setHealthWatches(groupId groupHandle) (err error) {
	result := C.dcgmHealthSet(handle.handle, groupId.handle, C.DCGM_HEALTH_WATCH_ALL)
	if err = errorString(result); err != nil {
		return fmt.Errorf("Error setting health watches: %s", err)
	}
	return
}

func healthCheckByGpuId(gpuId uint) (deviceHealth DeviceHealth, err error) {
	name := fmt.Sprintf("health%d", rand.Uint64())
	groupId, err := createGroup(name)
	if err != nil {
		return
	}

	err = addToGroup(groupId, gpuId)
	if err != nil {
		return
	}

	err = setHealthWatches(groupId)
	if err != nil {
		return
	}

	var healthResults C.dcgmHealthResponse_v1
	healthResults.version = makeVersion1(unsafe.Sizeof(healthResults))

	result := C.dcgmHealthCheck(handle.handle, groupId.handle, (*C.dcgmHealthResponse_t)(unsafe.Pointer(&healthResults)))

	if err = errorString(result); err != nil {
		return deviceHealth, fmt.Errorf("Error checking GPU health: %s", err)
	}

	status := healthStatus(int8(healthResults.overallHealth))
	watches := []SystemWatch{}

	// only 1 gpu
	i := 0

	// number of watches that encountred error/warning
	incidents := uint(healthResults.gpu[i].incidentCount)

	for j := uint(0); j < incidents; j++ {
		watch := SystemWatch{
			Type:   systemWatch(int(healthResults.gpu[i].systems[j].system)),
			Status: healthStatus(int8(healthResults.gpu[i].systems[j].health)),

			Error: *stringPtr(&healthResults.gpu[i].systems[j].errorString[0]),
		}
		watches = append(watches, watch)
	}

	deviceHealth = DeviceHealth{
		GPU:     gpuId,
		Status:  status,
		Watches: watches,
	}
	_ = destroyGroup(groupId)
	return
}

func healthStatus(status int8) string {
	switch status {
	case 0:
		return "Healthy"
	case 10:
		return "Warning"
	case 20:
		return "Failure"
	}
	return "N/A"
}

func systemWatch(watch int) string {
	switch watch {
	case 1:
		return "PCIe watches"
	case 2:
		return "NVLINK watches"
	case 4:
		return "Power Managemnt unit watches"
	case 8:
		return "Microcontroller unit watches"
	case 16:
		return "Memory watches"
	case 32:
		return "Streaming Multiprocessor watches"
	case 64:
		return "Inforom watches"
	case 128:
		return "Temperature watches"
	case 256:
		return "Power watches"
	case 512:
		return "Driver-related watches"
	}
	return "N/A"
}
