package main

import (
	"fmt"
	"log"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

// dcgmi introspect --enable
// dcgmi introspect -s -H
func main() {
	if err := dcgm.Init(dcgm.Embedded); err != nil {
		log.Panicln(err)
	}
	defer func() {
		if err := dcgm.Shutdown(); err != nil {
			log.Panicln(err)
		}
	}()

	st, err := dcgm.Introspect()
	if err != nil {
		log.Panicln(err)
	}

	fmt.Printf("Memory %2s %v KB\nCPU %5s %.2f %s\n", ":", st.Memory, ":", st.CPU, "%")
}
