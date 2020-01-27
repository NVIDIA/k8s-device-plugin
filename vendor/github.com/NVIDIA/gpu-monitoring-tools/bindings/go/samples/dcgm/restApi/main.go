package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/dcgm"
)

// res: curl localhost:8070/dcgm/device/info/id/0

func main() {
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, syscall.SIGINT, syscall.SIGTERM)

	if err := dcgm.Init(dcgm.Embedded); err != nil {
		log.Panicln(err)
	}
	defer func() {
		if err := dcgm.Shutdown(); err != nil {
			log.Panicln(err)
		}
	}()

	addr := ":8070"
	server := newHttpServer(addr)

	go func() {
		log.Printf("Running http server on localhost%s", addr)
		server.serve()
	}()
	defer server.stop()

	<-stopSig
	return
}
