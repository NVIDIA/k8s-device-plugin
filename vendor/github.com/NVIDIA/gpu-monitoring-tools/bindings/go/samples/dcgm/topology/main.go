package main

import (
	"fmt"
	"log"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

const (
	legend = `
Legend:
 X    = Self
 SYS  = Connection traversing PCIe as well as the SMP interconnect between NUMA nodes (e.g., QPI/UPI)
 NODE = Connection traversing PCIe as well as the interconnect between PCIe Host Bridges within a NUMA node
 PHB  = Connection traversing PCIe as well as a PCIe Host Bridge (typically the CPU)
 PXB  = Connection traversing multiple PCIe switches (without traversing the PCIe Host Bridge)
 PIX  = Connection traversing a single PCIe switch
 PSB  = Connection traversing a single on-board PCIe switch
 NV#  = Connection traversing a bonded set of # NVLinks`
)

// based on nvidia-smi topo -m
// dcgmi topo
func main() {
	// choose dcgm hostengine running mode
	// 1. dcgm.Embedded
	// 2. dcgm.Standalone
	// 3. dcgm.StartHostengine
	if err := dcgm.Init(dcgm.StartHostengine); err != nil {
		log.Panicln(err)
	}
	defer func() {
		if err := dcgm.Shutdown(); err != nil {
			log.Panicln(err)
		}
	}()

	gpus, err := dcgm.GetSupportedDevices()
	if err != nil {
		log.Panicln(err)
	}

	for _, gpu := range gpus {
		fmt.Printf("%9s%d", "GPU", gpu)
	}
	fmt.Printf("%5s\n", "CPUAffinity")

	numGpus := len(gpus)
	gpuTopo := make([]string, numGpus)
	for i := 0; i < numGpus; i++ {
		topo, err := dcgm.GetDeviceTopology(gpus[i])
		if err != nil {
			log.Panicln(err)
		}

		fmt.Printf("GPU%d", gpus[i])
		for j := 0; j < len(topo); j++ {
			// skip current GPU
			gpuTopo[topo[j].GPU] = topo[j].Link.PCIPaths()
		}
		gpuTopo[i] = "X"
		for j := 0; j < numGpus; j++ {
			fmt.Printf("%5s", gpuTopo[j])
		}
		deviceInfo, err := dcgm.GetDeviceInfo(gpus[i])
		if err != nil {
			log.Panicln(err)
		}
		fmt.Printf("%5s\n", deviceInfo.CPUAffinity)
	}
	fmt.Println(legend)
}
