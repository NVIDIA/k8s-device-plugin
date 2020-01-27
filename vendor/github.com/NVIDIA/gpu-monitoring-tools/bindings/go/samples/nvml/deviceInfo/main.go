package main

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

const (
	DEVICEINFO = `UUID           : {{.UUID}}
Model          : {{or .Model "N/A"}}
Path           : {{.Path}}
Power          : {{if .Power}}{{.Power}} W{{else}}N/A{{end}}
Memory         : {{if .Memory}}{{.Memory}} MiB{{else}}N/A{{end}}
CudaComputeCap : {{if .CudaComputeCapability.Major}}{{.CudaComputeCapability.Major}}.{{.CudaComputeCapability.Minor}}{{else}}N/A{{end}}
CPU Affinity   : {{if .CPUAffinity}}NUMA node{{.CPUAffinity}}{{else}}N/A{{end}}
Bus ID         : {{.PCI.BusID}}
BAR1           : {{if .PCI.BAR1}}{{.PCI.BAR1}} MiB{{else}}N/A{{end}}
Bandwidth      : {{if .PCI.Bandwidth}}{{.PCI.Bandwidth}} MB/s{{else}}N/A{{end}}
Cores          : {{if .Clocks.Cores}}{{.Clocks.Cores}} MHz{{else}}N/A{{end}}
Memory         : {{if .Clocks.Memory}}{{.Clocks.Memory}} MHz{{else}}N/A{{end}}
P2P Available  : {{if not .Topology}}None{{else}}{{range .Topology}}
                      {{.BusID}} - {{(.Link.String)}}{{end}}{{end}}
---------------------------------------------------------------------
`
)

func main() {
	nvml.Init()
	defer nvml.Shutdown()

	count, err := nvml.GetDeviceCount()
	if err != nil {
		log.Panicln("Error getting device count:", err)
	}

	driverVersion, err := nvml.GetDriverVersion()
	if err != nil {
		log.Panicln("Error getting driver version:", err)
	}

	t := template.Must(template.New("Device").Parse(DEVICEINFO))

	fmt.Printf("Driver Version : %5v\n", driverVersion)
	for i := uint(0); i < count; i++ {
		device, err := nvml.NewDevice(i)
		if err != nil {
			log.Panicf("Error getting device %d: %v\n", i, err)
		}

		fmt.Printf("GPU %12s %d\n", ":", i)
		err = t.Execute(os.Stdout, device)
		if err != nil {
			log.Panicln("Template error:", err)
		}
	}
}
