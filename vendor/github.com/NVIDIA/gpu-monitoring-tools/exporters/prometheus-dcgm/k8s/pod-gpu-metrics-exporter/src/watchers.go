// Copyright (c) 2018, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/golang/glog"
	"gopkg.in/fsnotify/fsnotify.v1"
)

const (
	socketPath        = "/var/lib/kubelet/pod-resources/kubelet.sock"
	gpuMetricsPath    = "/run/prometheus/"
	gpuMetrics        = gpuMetricsPath + "dcgm.prom"
	gpuPodMetricsPath = "/run/dcgm/"
	gpuPodMetrics     = gpuPodMetricsPath + "dcgm-pod.prom"
)

func watchAndWriteGPUmetrics() {
	watcher, err := watchDir(gpuMetricsPath)
	if err != nil {
		glog.Fatal(err)
	}
	defer watcher.Close()

	// create gpuPodMetrics dir
	err = createMetricsDir(gpuPodMetricsPath)
	if err != nil {
		glog.Fatal(err)
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Name == gpuMetrics && event.Op&fsnotify.Create == fsnotify.Create {
				glog.V(1).Infof("inotify: %s created, now adding device pod information.", gpuMetrics)
				podMap, err := getDevicePodInfo(socketPath)
				if err != nil {
					glog.Error(err)
					return
				}
				err = addPodInfoToMetrics(gpuPodMetricsPath, gpuMetrics, gpuPodMetrics, podMap)
				if err != nil {
					glog.Error(err)
					return
				}
			}

		case err := <-watcher.Errors:
			glog.Errorf("inotify: %s", err)

			// exit if there are no events for 20 seconds.
		case <-time.After(time.Second * 20):
			glog.Fatal("No events received. Make sure \"dcgm-exporter\" is running")
			return
		}
	}
}

func watchDir(path string) (*fsnotify.Watcher, error) {
	// Make sure the arg is a dir
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error getting information for %s: %v", path, err)
	}

	if !fi.Mode().IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create FS Watcher: %v", err)
	}

	err = watcher.Add(path)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to add %s to Watcher: %v", path, err)
	}
	return watcher, nil
}

func sigWatcher(sigs ...os.Signal) chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sigs...)
	return sigChan
}
