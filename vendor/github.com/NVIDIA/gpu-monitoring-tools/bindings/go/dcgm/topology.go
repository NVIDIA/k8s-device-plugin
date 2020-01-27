package dcgm

/*
#include "dcgm_agent.h"
#include "dcgm_structs.h"
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"strings"
	"unsafe"
)

type P2PLinkType uint

const (
	P2PLinkUnknown P2PLinkType = iota
	P2PLinkCrossCPU
	P2PLinkSameCPU
	P2PLinkHostBridge
	P2PLinkMultiSwitch
	P2PLinkSingleSwitch
	P2PLinkSameBoard
	SingleNVLINKLink
	TwoNVLINKLinks
	ThreeNVLINKLinks
	FourNVLINKLinks
)

func (l P2PLinkType) PCIPaths() string {
	switch l {
	case P2PLinkSameBoard:
		return "PSB"
	case P2PLinkSingleSwitch:
		return "PIX"
	case P2PLinkMultiSwitch:
		return "PXB"
	case P2PLinkHostBridge:
		return "PHB"
	case P2PLinkSameCPU:
		return "NODE"
	case P2PLinkCrossCPU:
		return "SYS"
	case SingleNVLINKLink:
		return "NV1"
	case TwoNVLINKLinks:
		return "NV2"
	case ThreeNVLINKLinks:
		return "NV3"
	case FourNVLINKLinks:
		return "NV4"
	case P2PLinkUnknown:
	}
	return "N/A"
}

type P2PLink struct {
	GPU   uint
	BusID string
	Link  P2PLinkType
}

func getP2PLink(path uint) P2PLinkType {
	switch path {
	case C.DCGM_TOPOLOGY_BOARD:
		return P2PLinkSameBoard
	case C.DCGM_TOPOLOGY_SINGLE:
		return P2PLinkSingleSwitch
	case C.DCGM_TOPOLOGY_MULTIPLE:
		return P2PLinkMultiSwitch
	case C.DCGM_TOPOLOGY_HOSTBRIDGE:
		return P2PLinkHostBridge
	case C.DCGM_TOPOLOGY_CPU:
		return P2PLinkSameCPU
	case C.DCGM_TOPOLOGY_SYSTEM:
		return P2PLinkCrossCPU
	case C.DCGM_TOPOLOGY_NVLINK1:
		return SingleNVLINKLink
	case C.DCGM_TOPOLOGY_NVLINK2:
		return TwoNVLINKLinks
	case C.DCGM_TOPOLOGY_NVLINK3:
		return ThreeNVLINKLinks
	case C.DCGM_TOPOLOGY_NVLINK4:
		return FourNVLINKLinks
	}
	return P2PLinkUnknown
}

func getCPUAffinity(busid string) (string, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("/sys/bus/pci/devices/%s/local_cpulist", strings.ToLower(busid[4:])))
	if err != nil {
		return "", fmt.Errorf("Error getting device cpu affinity: %v", err)
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

func getBusid(gpuid uint) (string, error) {
	var device C.dcgmDeviceAttributes_t
	device.version = makeVersion1(unsafe.Sizeof(device))

	result := C.dcgmGetDeviceAttributes(handle.handle, C.uint(gpuid), &device)
	if err := errorString(result); err != nil {
		return "", fmt.Errorf("Error getting device busid: %s", err)
	}
	return *stringPtr(&device.identifiers.pciBusId[0]), nil
}

func getDeviceTopology(gpuid uint) (links []P2PLink, err error) {
	var topology C.dcgmDeviceTopology_t
	topology.version = makeVersion1(unsafe.Sizeof(topology))

	result := C.dcgmGetDeviceTopology(handle.handle, C.uint(gpuid), &topology)
	if result == C.DCGM_ST_NOT_SUPPORTED {
		return links, nil
	}
	if result != C.DCGM_ST_OK {
		return links, fmt.Errorf("Error getting device topology: %s", errorString(result))
	}

	busid, err := getBusid(gpuid)
	if err != nil {
		return
	}

	for i := uint(0); i < uint(topology.numGpus); i++ {
		gpu := topology.gpuPaths[i].gpuId
		p2pLink := P2PLink{
			GPU:   uint(gpu),
			BusID: busid,
			Link:  getP2PLink(uint(topology.gpuPaths[i].path)),
		}
		links = append(links, p2pLink)
	}
	return
}
