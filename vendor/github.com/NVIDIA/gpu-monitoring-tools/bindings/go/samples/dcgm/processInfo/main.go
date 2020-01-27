package main

import (
	"flag"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

const (
	processInfo = `----------------------------------------------------------------------
GPU ID			     : {{.GPU}}
----------Execution Stats---------------------------------------------
PID                          : {{.PID}}
Name                         : {{or .Name "N/A"}}
Start Time                   : {{.ProcessUtilization.StartTime.String}}
End Time                     : {{.ProcessUtilization.EndTime.String}}
----------Performance Stats-------------------------------------------
Energy Consumed (Joules)     : {{or .ProcessUtilization.EnergyConsumed "N/A"}}
Max GPU Memory Used (bytes)  : {{or .Memory.GlobalUsed "N/A"}}
Avg SM Clock (MHz)           : {{or .Clocks.Cores "N/A"}}
Avg Memory Clock (MHz)       : {{or .Clocks.Memory "N/A"}}
Avg SM Utilization (%)       : {{or .GpuUtilization.Memory "N/A"}}
Avg Memory Utilization (%)   : {{or .GpuUtilization.GPU "N/A"}}
Avg PCIe Rx Bandwidth (MB)   : {{or .PCI.Throughput.Rx "N/A"}}
Avg PCIe Tx Bandwidth (MB)   : {{or .PCI.Throughput.Tx "N/A"}}
----------Event Stats-------------------------------------------------
Single Bit ECC Errors        : {{or .Memory.ECCErrors.SingleBit "N/A"}}
Double Bit ECC Errors        : {{or .Memory.ECCErrors.DoubleBit "N/A"}}
Critical XID Errors          : {{.XIDErrors.NumErrors}}
----------Slowdown Stats----------------------------------------------
Due to - Power (%)           : {{or .Violations.Power "N/A"}}
       - Thermal (%)         : {{or .Violations.Thermal "N/A"}}
       - Reliability (%)     : {{or .Violations.Reliability "N/A"}}
       - Board Limit (%)     : {{or .Violations.BoardLimit "N/A"}}
       - Low Utilization (%) : {{or .Violations.LowUtilization "N/A"}}
       - Sync Boost (%)      : {{or .Violations.SyncBoost "N/A"}}
----------Process Utilization-----------------------------------------
Avg SM Utilization (%)       : {{or .ProcessUtilization.SmUtil "N/A"}}
Avg Memory Utilization (%)   : {{or .ProcessUtilization.MemUtil "N/A"}}
----------------------------------------------------------------------
`
)

var process = flag.Uint("pid", 0, "Provide pid to get this process information.")

// run as root, for enabling health watches
// dcgmi stats -e
// dcgmi stats --pid ENTERPID -v
// sample: sudo ./processInfo -pid PID
func main() {
	if err := dcgm.Init(dcgm.Embedded); err != nil {
		log.Panicln(err)
	}
	defer func() {
		if err := dcgm.Shutdown(); err != nil {
			log.Panicln(err)
		}
	}()

	// Request DCGM to start recording stats for GPU process fields
	group, err := dcgm.WatchPidFields()
	if err != nil {
		log.Panicln(err)
	}

	// Before retrieving process stats, wait few seconds for watches to be enabled and collect data
	log.Println("Enabling DCGM watches to start collecting process stats. This may take a few seconds....")
	time.Sleep(3000 * time.Millisecond)

	flag.Parse()
	pidInfo, err := dcgm.GetProcessInfo(group, *process)
	if err != nil {
		log.Panicln(err)
	}

	t := template.Must(template.New("Process").Parse(processInfo))
	for _, gpu := range pidInfo {

		if err = t.Execute(os.Stdout, gpu); err != nil {
			log.Panicln("Template error:", err)
		}
	}
}
