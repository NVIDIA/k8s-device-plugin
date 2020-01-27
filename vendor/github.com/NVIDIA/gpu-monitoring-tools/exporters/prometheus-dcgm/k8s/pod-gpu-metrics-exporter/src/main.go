package main

import (
	"flag"
	"syscall"

	"github.com/golang/glog"
)

// http port serving metrics
const port = ":9400"

// res: curl localhost:9400/gpu/metrics
func main() {
	defer glog.Flush()
	flag.Parse()

	glog.Info("Starting OS watcher.")
	sigs := sigWatcher(syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// watch and write gpu metrics to dcgm-pod.prom
	go func() {
		glog.Info("Starting FS watcher.")
		watchAndWriteGPUmetrics()
	}()

	server := newHttpServer(port)
	defer stopHttp(server)

	// expose metrics to localhost:9400/gpu/metrics
	go func() {
		glog.V(1).Infof("Running http server on localhost%s", port)
		startHttp(server)
	}()

	sig := <-sigs
	glog.V(2).Infof("Received signal \"%v\", shutting down.", sig)
	return
}
