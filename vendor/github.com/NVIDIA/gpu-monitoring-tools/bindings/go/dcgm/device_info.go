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

type PCIInfo struct {
	BusID     string
	BAR1      *uint // MB
	FBTotal   *uint // MB
	Bandwidth *uint // MB/s
}

type DeviceIdentifiers struct {
	Brand               string
	Model               string
	Serial              string
	Vbios               string
	InforomImageVersion string
	DriverVersion       string
}

type Device struct {
	GPU           uint
	DCGMSupported string
	UUID          string
	Power         *uint // W
	PCI           PCIInfo
	Clocks        ClockInfo
	Identifiers   DeviceIdentifiers
	Topology      []P2PLink
	CPUAffinity   string
}

// getAllDeviceCount counts all GPUs on the system
func getAllDeviceCount() (gpuCount uint, err error) {
	var gpuIdList [C.DCGM_MAX_NUM_DEVICES]C.uint
	var count C.int

	result := C.dcgmGetAllDevices(handle.handle, &gpuIdList[0], &count)
	if err = errorString(result); err != nil {
		return gpuCount, fmt.Errorf("Error getting devices count: %s", err)
	}
	gpuCount = uint(count)
	return
}

// getSupportedDevices returns DCGM supported GPUs
func getSupportedDevices() (gpus []uint, err error) {
	var gpuIdList [C.DCGM_MAX_NUM_DEVICES]C.uint
	var count C.int

	result := C.dcgmGetAllSupportedDevices(handle.handle, &gpuIdList[0], &count)
	if err = errorString(result); err != nil {
		return gpus, fmt.Errorf("Error getting DCGM supported devices: %s", err)
	}

	numGpus := int(count)
	gpus = make([]uint, numGpus)
	for i := 0; i < numGpus; i++ {
		gpus[i] = uint(gpuIdList[i])
	}
	return
}

func getPciBandwidth(gpuId uint) (*uint, error) {
	const (
		maxLinkGen int = iota
		maxLinkWidth
		fieldsCount
	)

	pciFields := make([]C.ushort, fieldsCount)
	pciFields[maxLinkGen] = C.DCGM_FI_DEV_PCIE_MAX_LINK_GEN
	pciFields[maxLinkWidth] = C.DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH

	fieldsName := fmt.Sprintf("pciBandwidthFields%d", rand.Uint64())

	fieldsId, err := fieldGroupCreate(fieldsName, pciFields, fieldsCount)
	if err != nil {
		return nil, err
	}

	groupName := fmt.Sprintf("pciBandwidth%d", rand.Uint64())
	groupId, err := watchFields(gpuId, fieldsId, groupName)
	if err != nil {
		_ = fieldGroupDestroy(fieldsId)
		return nil, err
	}

	values := make([]C.dcgmFieldValue_t, fieldsCount)
	result := C.dcgmGetLatestValuesForFields(handle.handle, C.int(gpuId), &pciFields[0], C.uint(fieldsCount), &values[0])
	if err = errorString(result); err != nil {
		_ = fieldGroupDestroy(fieldsId)
		_ = destroyGroup(groupId)
		return nil, fmt.Errorf("Error getting Pcie bandwidth: %s", err)
	}

	gen := uintPtrUnsafe(unsafe.Pointer(&values[maxLinkGen].value))
	width := uintPtrUnsafe(unsafe.Pointer(&values[maxLinkWidth].value))

	_ = fieldGroupDestroy(fieldsId)
	_ = destroyGroup(groupId)

	genMap := map[uint]uint{
		1: 250, // MB/s
		2: 500,
		3: 985,
		4: 1969,
	}
	if gen == nil || width == nil {
		return nil, nil
	}
	bandwidth := genMap[*gen] * *width
	return &bandwidth, nil
}

func getDeviceInfo(gpuid uint) (deviceInfo Device, err error) {
	var device C.dcgmDeviceAttributes_t
	device.version = makeVersion1(unsafe.Sizeof(device))

	result := C.dcgmGetDeviceAttributes(handle.handle, C.uint(gpuid), &device)
	if err = errorString(result); err != nil {
		return deviceInfo, fmt.Errorf("Error getting device information: %s", err)
	}

	// check if the given GPU is DCGM supported
	gpus, err := getSupportedDevices()
	if err != nil {
		return
	}

	supported := "No"

	for _, gpu := range gpus {
		if gpuid == gpu {
			supported = "Yes"
			break
		}
	}

	busid := *stringPtr(&device.identifiers.pciBusId[0])

	cpuAffinity, err := getCPUAffinity(busid)
	if err != nil {
		return
	}

	var topology []P2PLink
	var bandwidth *uint
	// get device topology and bandwidth only if its a DCGM supported device
	if supported == "Yes" {
		topology, err = getDeviceTopology(gpuid)
		if err != nil {
			return
		}
		bandwidth, err = getPciBandwidth(gpuid)
		if err != nil {
			return
		}
	}

	uuid := *stringPtr(&device.identifiers.uuid[0])
	power := uintPtr(device.powerLimits.defaultPowerLimit)

	pci := PCIInfo{
		BusID:     busid,
		BAR1:      uintPtr(device.memoryUsage.bar1Total),
		FBTotal:   uintPtr(device.memoryUsage.fbTotal),
		Bandwidth: bandwidth,
	}

	var clocks ClockInfo
	if device.clockSets.count >= 1 {
		clocks.Memory = uintPtr(device.clockSets.clockSet[0].memClock)
		clocks.Cores = uintPtr(device.clockSets.clockSet[0].smClock)
	}

	identifiers := DeviceIdentifiers{
		Brand:               *stringPtr(&device.identifiers.brandName[0]),
		Model:               *stringPtr(&device.identifiers.deviceName[0]),
		Serial:              *stringPtr(&device.identifiers.serial[0]),
		Vbios:               *stringPtr(&device.identifiers.vbios[0]),
		InforomImageVersion: *stringPtr(&device.identifiers.inforomImageVersion[0]),
		DriverVersion:       *stringPtr(&device.identifiers.driverVersion[0]),
	}

	deviceInfo = Device{
		GPU:           gpuid,
		DCGMSupported: supported,
		UUID:          uuid,
		Power:         power,
		PCI:           pci,
		Clocks:        clocks,
		Identifiers:   identifiers,
		Topology:      topology,
		CPUAffinity:   cpuAffinity,
	}
	return
}
