package main

import (
	"flag"
	"log"
	"os"
	"text/template"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

const (
	deviceInfo = `Driver Version         : {{.Identifiers.DriverVersion}}
GPU		       : {{.GPU}}
DCGMSupported          : {{.DCGMSupported}}
UUID                   : {{.UUID}}
Brand                  : {{.Identifiers.Brand}}
Model                  : {{.Identifiers.Model}}
Serial Number          : {{.Identifiers.Serial}}
Vbios                  : {{or .Identifiers.Vbios "N/A"}}
InforomImage Version   : {{.Identifiers.InforomImageVersion}}
Bus ID                 : {{.PCI.BusID}}
BAR1 (MB)              : {{or .PCI.BAR1 "N/A"}}
FrameBuffer Memory (MB): {{or .PCI.FBTotal "N/A"}}
Bandwidth (MB/s)       : {{or .PCI.Bandwidth "N/A"}}
Cores (MHz)            : {{or .Clocks.Cores "N/A"}}
Memory (MHz)           : {{or .Clocks.Memory "N/A"}}
Power (W)              : {{or .Power "N/A"}}
CPUAffinity            : {{or .CPUAffinity "N/A"}}
P2P Available          : {{if not .Topology}}None{{else}}{{range .Topology}}
    GPU{{.GPU}} - (BusID){{.BusID}} - {{.Link.PCIPaths}}{{end}}{{end}}
---------------------------------------------------------------------
`
)

var (
	connectAddr = flag.String("connect", "localhost", "Provide nv-hostengine connection address.")
	isSocket    = flag.String("socket", "0", "Connecting to Unix socket?")
)

// mini version of nvidia-smi -q
// dcgmi discovery -i apc
func main() {
	// choose dcgm hostengine running mode
	// 1. dcgm.Embedded
	// 2. dcgm.Standalone -connect "addr", -socket "isSocket"
	// 3. dcgm.StartHostengine
	flag.Parse()
	if err := dcgm.Init(dcgm.Standalone, *connectAddr, *isSocket); err != nil {
		log.Panicln(err)
	}

	defer func() {
		if err := dcgm.Shutdown(); err != nil {
			log.Panicln(err)
		}
	}()

	count, err := dcgm.GetAllDeviceCount()
	if err != nil {
		log.Panicln(err)
	}

	t := template.Must(template.New("Device").Parse(deviceInfo))

	for i := uint(0); i < count; i++ {
		deviceInfo, err := dcgm.GetDeviceInfo(i)
		if err != nil {
			log.Panicln(err)
		}

		if err = t.Execute(os.Stdout, deviceInfo); err != nil {
			log.Panicln("Template error:", err)
		}
	}
}
