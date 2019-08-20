package main

import (
	"k8s.io/api/core/v1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// IsGPUTopoPod determines if it's the pod for GPU topology
func IsGPUTopoPod(pod *v1.Pod) bool {
	return GetGPUTopoNum(pod) > 0
}

func GetGPUTopoNum(pod *v1.Pod) int64 {

	res := &schedulernodeinfo.Resource{}
	for _, container := range pod.Spec.Containers {
		res.Add(container.Resources.Requests)
	}

	// take max_resource(sum_pod, any_init_container)
	for _, container := range pod.Spec.InitContainers {
		res.SetMaxResource(container.Resources.Requests)
	}

	resList := res.ResourceList()
	gpuTopo := resList["nvidia.com/gpu-topo"]
	gpuTopoNum, _ := gpuTopo.AsInt64()

	return gpuTopoNum
}
