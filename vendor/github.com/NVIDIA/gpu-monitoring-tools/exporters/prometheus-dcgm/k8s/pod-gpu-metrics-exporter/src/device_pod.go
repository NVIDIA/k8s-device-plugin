// Copyright (c) 2018, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"
)

const nvidiaResourceName = "nvidia.com/gpu"

type devicePodInfo struct {
	name      string
	namespace string
	container string
}

// Helper function that creates a map of pod info for each device
func createDevicePodMap(devicePods podresourcesapi.ListPodResourcesResponse) map[string]devicePodInfo {
	deviceToPodMap := make(map[string]devicePodInfo)

	for _, pod := range devicePods.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if device.GetResourceName() == nvidiaResourceName {
					podInfo := devicePodInfo{
						name:      pod.GetName(),
						namespace: pod.GetNamespace(),
						container: container.GetName(),
					}
					for _, uuid := range device.GetDeviceIds() {
						deviceToPodMap[uuid] = podInfo
					}
				}
			}
		}
	}
	return deviceToPodMap
}

func getDevicePodInfo(socket string) (map[string]devicePodInfo, error) {
	devicePods, err := getListOfPods(socket)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices Pod information: %v", err)
	}
	return createDevicePodMap(*devicePods), nil

}

func addPodInfoToMetrics(dir string, srcFile string, destFile string, deviceToPodMap map[string]devicePodInfo) error {
	readFI, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", srcFile, err)
	}
	defer readFI.Close()
	reader := bufio.NewReader(readFI)

	tmpPrefix := "pod"
	tmpF, err := ioutil.TempFile(dir, tmpPrefix)
	if err != nil {
		return fmt.Errorf("error creating temp file: %v", err)
	}

	tmpFname := tmpF.Name()
	defer func() {
		tmpF.Close()
		os.Remove(tmpFname)
	}()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(line) == 0 {
				return writeDestFile(tmpFname, destFile)
			}
			return fmt.Errorf("error reading %s: %v", srcFile, err)
		}

		// Skip comments and add pod info
		if string(line[0]) != "#" {
			uuid := strings.Split(strings.Split(line, ",")[1], "\"")[1]
			if pod, exists := deviceToPodMap[uuid]; exists {
				splitLine := strings.Split(line, "}")
				line = fmt.Sprintf("%s,pod_name=\"%s\",pod_namespace=\"%s\",container_name=\"%s\"}%s", splitLine[0], pod.name, pod.namespace, pod.container, splitLine[1])
			}
		}

		_, err = tmpF.WriteString(line)
		if err != nil {
			return fmt.Errorf("error writing to %s: %v", tmpFname, err)
		}
	}
}
