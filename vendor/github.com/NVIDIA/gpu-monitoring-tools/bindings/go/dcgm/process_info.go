package dcgm

/*
#include "dcgm_agent.h"
#include "dcgm_structs.h"
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"
	"unsafe"
)

type Time uint64

func (t Time) String() string {
	if t == 0 {
		return "Running"
	}
	tm := time.Unix(int64(t), 0)
	return tm.String()
}

type ProcessUtilInfo struct {
	StartTime      Time
	EndTime        Time
	EnergyConsumed *uint64 // Joules
	SmUtil         *float64
	MemUtil        *float64
}

// ViolationTime measures amount of time (in ms) GPU was at reduced clocks
type ViolationTime struct {
	Power          *uint64
	Thermal        *uint64
	Reliability    *uint64
	BoardLimit     *uint64
	LowUtilization *uint64
	SyncBoost      *uint64
}

type XIDErrorInfo struct {
	NumErrors int
	Timestamp []uint64
}

type ProcessInfo struct {
	GPU                uint
	PID                uint
	Name               string
	ProcessUtilization ProcessUtilInfo
	PCI                PCIStatusInfo
	Memory             MemoryInfo
	GpuUtilization     UtilizationInfo
	Clocks             ClockInfo
	Violations         ViolationTime
	XIDErrors          XIDErrorInfo
}

func watchPidFields(gpus ...uint) (groupId groupHandle, err error) {
	groupName := fmt.Sprintf("watchPids%d", rand.Uint64())
	group, err := createGroup(groupName)
	if err != nil {
		return
	}
	numGpus := len(gpus)

	if numGpus == 0 {
		gpus, err = getSupportedDevices()
		if err != nil {
			return
		}
	}

	for _, gpu := range gpus {
		err = addToGroup(group, gpu)
		if err != nil {
			return
		}

	}

	result := C.dcgmWatchPidFields(handle.handle, group.handle, C.longlong(updateFreq), C.double(maxKeepAge), C.int(maxKeepSamples))

	if err = errorString(result); err != nil {
		return groupId, fmt.Errorf("Error watching process fields: %s", err)
	}
	_ = updateAllFields()
	return group, nil
}

func getProcessInfo(groupId groupHandle, pid uint) (processInfo []ProcessInfo, err error) {
	var pidInfo C.dcgmPidInfo_t
	pidInfo.version = makeVersion1(unsafe.Sizeof(pidInfo))
	pidInfo.pid = C.uint(pid)

	result := C.dcgmGetPidInfo(handle.handle, groupId.handle, &pidInfo)

	if err = errorString(result); err != nil {
		return processInfo, fmt.Errorf("Error getting process info: %s", err)
	}

	name, err := processName(pid)
	if err != nil {
		return processInfo, fmt.Errorf("Error getting process name: %s", err)
	}

	for i := 0; i < int(pidInfo.numGpus); i++ {

		var energy uint64
		e := blank64(uint64Ptr(pidInfo.gpus[i].energyConsumed))
		if e != nil {
			energy = *e / 1000 // mWs to joules
		}

		processUtil := ProcessUtilInfo{
			StartTime:      Time(uint64(pidInfo.gpus[i].startTime) / 1000000),
			EndTime:        Time(uint64(pidInfo.gpus[i].endTime) / 1000000),
			EnergyConsumed: &energy,
			SmUtil:         roundFloat(dblToFloat(pidInfo.gpus[i].processUtilization.smUtil)),
			MemUtil:        roundFloat(dblToFloat(pidInfo.gpus[i].processUtilization.memUtil)),
		}

		pci := PCIStatusInfo{
			Throughput: PCIThroughputInfo{
				Rx:      blank64(uint64Ptr(pidInfo.gpus[i].pcieRxBandwidth.average)),
				Tx:      blank64(uint64Ptr(pidInfo.gpus[i].pcieTxBandwidth.average)),
				Replays: blank64(uint64Ptr(pidInfo.gpus[i].pcieReplays)),
			},
		}

		memory := MemoryInfo{
			GlobalUsed: blank64(uint64Ptr(pidInfo.gpus[i].maxGpuMemoryUsed)), // max gpu memory used for this process
			ECCErrors: ECCErrorsInfo{
				SingleBit: blank32(uintPtr(pidInfo.gpus[i].eccSingleBit)),
				DoubleBit: blank32(uintPtr(pidInfo.gpus[i].eccDoubleBit)),
			},
		}

		gpuUtil := UtilizationInfo{
			GPU:    blank32(uintPtrInt(pidInfo.gpus[i].smUtilization.average)),
			Memory: blank32(uintPtrInt(pidInfo.gpus[i].memoryUtilization.average)),
		}

		violations := ViolationTime{
			Power:          blank64(uint64Ptr(pidInfo.gpus[i].powerViolationTime)),
			Thermal:        blank64(uint64Ptr(pidInfo.gpus[i].thermalViolationTime)),
			Reliability:    blank64(uint64Ptr(pidInfo.gpus[i].reliabilityViolationTime)),
			BoardLimit:     blank64(uint64Ptr(pidInfo.gpus[i].boardLimitViolationTime)),
			LowUtilization: blank64(uint64Ptr(pidInfo.gpus[i].lowUtilizationTime)),
			SyncBoost:      blank64(uint64Ptr(pidInfo.gpus[i].syncBoostTime)),
		}

		clocks := ClockInfo{
			Cores:  blank32(uintPtrInt(pidInfo.gpus[i].smClock.average)),
			Memory: blank32(uintPtrInt(pidInfo.gpus[i].memoryClock.average)),
		}

		numErrs := int(pidInfo.gpus[i].numXidCriticalErrors)
		ts := make([]uint64, numErrs)
		for i := 0; i < numErrs; i++ {
			ts[i] = uint64(pidInfo.gpus[i].xidCriticalErrorsTs[i])
		}
		xidErrs := XIDErrorInfo{
			NumErrors: numErrs,
			Timestamp: ts,
		}

		pInfo := ProcessInfo{
			GPU:                uint(pidInfo.summary.gpuId),
			PID:                uint(pidInfo.pid),
			Name:               name,
			ProcessUtilization: processUtil,
			PCI:                pci,
			Memory:             memory,
			GpuUtilization:     gpuUtil,
			Clocks:             clocks,
			Violations:         violations,
			XIDErrors:          xidErrs,
		}
		processInfo = append(processInfo, pInfo)
	}
	_ = destroyGroup(groupId)
	return
}

func processName(pid uint) (string, error) {
	f := fmt.Sprintf("/proc/%d/comm", pid)
	b, err := ioutil.ReadFile(f)
	if err != nil {
		// TOCTOU: process terminated
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}
