package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

const (
	header = `# gpu   pwr  temp    sm   mem   enc   dec  mclk  pclk
# Idx     W     C     %     %     %     %   MHz   MHz`
)

// modelled on nvidia-smi dmon
// dcgmi dmon -e 155,150,203,204,206,207,100,101
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

	fmt.Println(header)
	for {
		select {
		case <-ticker.C:
			for _, gpu := range gpus {
				st, err := dcgm.GetDeviceStatus(gpu)
				if err != nil {
					log.Panicln(err)
				}
				fmt.Printf("%5d %5d %5d %5d %5d %5d %5d %5d %5d\n",
					gpu, int64(*st.Power), *st.Temperature, *st.Utilization.GPU, *st.Utilization.Memory,
					*st.Utilization.Encoder, *st.Utilization.Decoder, *st.Clocks.Memory, *st.Clocks.Cores)
			}

		case <-sigs:
			return
		}
	}
}
