package main

import (
	"fmt"
	"log"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

// dcgmi group -c "name" --default
// dcgmi policy -g GROUPID --set 0,0 -x -n -p -e -P 250 -T 100 -M 10
// dcgmi policy -g GROUPID --reg
func main() {
	if err := dcgm.Init(dcgm.Embedded); err != nil {
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

	// Choose policy conditions to register violation callback.
	// Note: Need to be root for some options
	// Available options are:
	// 1. dcgm.DbePolicy
	// 2. dcgm.PCIePolicy
	// 3. dcgm.MaxRtPgPolicy
	// 4. dcgm.ThermalPolicy
	// 5. dcgm.PowerPolicy
	// 6. dcgm.NvlinkPolicy
	// 7. dcgm.XidPolicy
	for _, gpu := range gpus {
		c, err := dcgm.Policy(gpu, dcgm.XidPolicy)
		if err != nil {
			log.Panicln(err)
		}

		pe := <-c
		fmt.Printf("GPU %8s %v\nError %6s %v\nTimestamp %2s %v\nData %7s %v\n",
			":", gpu, ":", pe.Condition, ":", pe.Timestamp, ":", pe.Data)
	}
}
