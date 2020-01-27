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

type PerfState uint

const (
	PerfStateMax     = 0
	PerfStateMin     = 15
	PerfStateUnknown = 32
)

func (p PerfState) String() string {
	if p >= PerfStateMax && p <= PerfStateMin {
		return fmt.Sprintf("P%d", p)
	}
	return "Unknown"
}

type UtilizationInfo struct {
	GPU     *uint // %
	Memory  *uint // %
	Encoder *uint // %
	Decoder *uint // %
}

type ECCErrorsInfo struct {
	SingleBit *uint
	DoubleBit *uint
}

type MemoryInfo struct {
	GlobalUsed *uint64
	ECCErrors  ECCErrorsInfo
}

type ClockInfo struct {
	Cores  *uint // MHz
	Memory *uint // MHz
}

type PCIThroughputInfo struct {
	Rx      *uint64 // MB
	Tx      *uint64 // MB
	Replays *uint64
}

type PCIStatusInfo struct {
	BAR1Used   *uint // MB
	Throughput PCIThroughputInfo
	FBUsed     *uint
}

type DeviceStatus struct {
	Power       *float64 // W
	Temperature *uint    // °C
	Utilization UtilizationInfo
	Memory      MemoryInfo
	Clocks      ClockInfo
	PCI         PCIStatusInfo
	Performance PerfState
	FanSpeed    uint // %
}

func latestValuesForDevice(gpuId uint) (status DeviceStatus, err error) {
	const (
		pwr int = iota
		temp
		sm
		mem
		enc
		dec
		smClock
		memClock
		bar1Used
		pcieRxThroughput
		pcieTxThroughput
		pcieReplay
		fbUsed
		sbe
		dbe
		pstate
		fanSpeed
		fieldsCount
	)

	var deviceFields []C.ushort = make([]C.ushort, fieldsCount)
	deviceFields[pwr] = C.DCGM_FI_DEV_POWER_USAGE
	deviceFields[temp] = C.DCGM_FI_DEV_GPU_TEMP
	deviceFields[sm] = C.DCGM_FI_DEV_GPU_UTIL
	deviceFields[mem] = C.DCGM_FI_DEV_MEM_COPY_UTIL
	deviceFields[enc] = C.DCGM_FI_DEV_ENC_UTIL
	deviceFields[dec] = C.DCGM_FI_DEV_DEC_UTIL
	deviceFields[smClock] = C.DCGM_FI_DEV_SM_CLOCK
	deviceFields[memClock] = C.DCGM_FI_DEV_MEM_CLOCK
	deviceFields[bar1Used] = C.DCGM_FI_DEV_BAR1_USED
	deviceFields[pcieRxThroughput] = C.DCGM_FI_DEV_PCIE_RX_THROUGHPUT
	deviceFields[pcieTxThroughput] = C.DCGM_FI_DEV_PCIE_TX_THROUGHPUT
	deviceFields[pcieReplay] = C.DCGM_FI_DEV_PCIE_REPLAY_COUNTER
	deviceFields[fbUsed] = C.DCGM_FI_DEV_FB_USED
	deviceFields[sbe] = C.DCGM_FI_DEV_ECC_SBE_AGG_TOTAL
	deviceFields[dbe] = C.DCGM_FI_DEV_ECC_DBE_AGG_TOTAL
	deviceFields[pstate] = C.DCGM_FI_DEV_PSTATE
	deviceFields[fanSpeed] = C.DCGM_FI_DEV_FAN_SPEED

	fieldsName := fmt.Sprintf("devStatusFields%d", rand.Uint64())
	fieldsId, err := fieldGroupCreate(fieldsName, deviceFields, fieldsCount)
	if err != nil {
		return
	}

	groupName := fmt.Sprintf("devStatus%d", rand.Uint64())
	groupId, err := watchFields(gpuId, fieldsId, groupName)
	if err != nil {
		_ = fieldGroupDestroy(fieldsId)
		return
	}

	values := make([]C.dcgmFieldValue_t, fieldsCount)
	result := C.dcgmGetLatestValuesForFields(handle.handle, C.int(gpuId), &deviceFields[0], C.uint(fieldsCount), &values[0])

	if err = errorString(result); err != nil {
		_ = fieldGroupDestroy(fieldsId)
		_ = destroyGroup(groupId)
		return status, fmt.Errorf("Error getting device status: %s", err)
	}

	power := dblToFloatUnsafe(unsafe.Pointer(&values[pwr].value))

	gpuUtil := UtilizationInfo{
		GPU:     uintPtrUnsafe(unsafe.Pointer(&values[sm].value)),
		Memory:  uintPtrUnsafe(unsafe.Pointer(&values[mem].value)),
		Encoder: uintPtrUnsafe(unsafe.Pointer(&values[enc].value)),
		Decoder: uintPtrUnsafe(unsafe.Pointer(&values[dec].value)),
	}

	memory := MemoryInfo{
		ECCErrors: ECCErrorsInfo{
			SingleBit: uintPtrUnsafe(unsafe.Pointer(&values[sbe].value)),
			DoubleBit: uintPtrUnsafe(unsafe.Pointer(&values[dbe].value)),
		},
	}

	clocks := ClockInfo{
		Cores:  uintPtrUnsafe(unsafe.Pointer(&values[smClock].value)),
		Memory: uintPtrUnsafe(unsafe.Pointer(&values[memClock].value)),
	}

	pci := PCIStatusInfo{
		BAR1Used: uintPtrUnsafe(unsafe.Pointer(&values[bar1Used].value)),
		Throughput: PCIThroughputInfo{
			Rx:      uint64PtrUnsafe(unsafe.Pointer(&values[pcieRxThroughput].value)),
			Tx:      uint64PtrUnsafe(unsafe.Pointer(&values[pcieTxThroughput].value)),
			Replays: uint64PtrUnsafe(unsafe.Pointer(&values[pcieReplay].value)),
		},
		FBUsed: uintPtrUnsafe(unsafe.Pointer(&values[fbUsed].value)),
	}

	status = DeviceStatus{
		Power:       power,
		Temperature: uintPtrUnsafe(unsafe.Pointer(&values[temp].value)),
		Utilization: gpuUtil,
		Memory:      memory,
		Clocks:      clocks,
		PCI:         pci,
		Performance: PerfState(*uintPtrUnsafe(unsafe.Pointer(&values[pstate].value))),
		FanSpeed:    *uintPtrUnsafe(unsafe.Pointer(&values[fanSpeed].value)),
	}

	_ = fieldGroupDestroy(fieldsId)
	_ = destroyGroup(groupId)
	return
}
