package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

const (
	healthStatus = `GPU                : {{.GPU}}
Status             : {{.Status}}
{{range .Watches}}
Type               : {{.Type}}
Status             : {{.Status}}
Error              : {{.Error}}
{{end}}
`
)

// create group: dcgmi group -c "name" --default
// enable watches: dcgmi health -s a
// check: dcgmi health -g 1 -c
func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

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

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	t := template.Must(template.New("Health").Parse(healthStatus))
	for {
		select {
		case <-ticker.C:
			for _, gpu := range gpus {
				h, err := dcgm.HealthCheckByGpuId(gpu)
				if err != nil {
					log.Panicln(err)
				}

				if err = t.Execute(os.Stdout, h); err != nil {
					log.Panicln("Template error:", err)
				}
			}
		case <-sigs:
			return
		}
	}
}
