/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package common

import (
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Define the Job
var GPUJob = &batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Name: "gpu-job",
	},
	Spec: batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gpu-pod",
			},
			Spec: v1.PodSpec{
				RestartPolicy: "Never",
				Containers: []v1.Container{
					{
						Name:  "cuda-container",
						Image: "nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda10.2",
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								"nvidia.com/gpu": resource.MustParse("1"),
							},
						},
					},
				},
				Tolerations: []v1.Toleration{
					{
						Key:      "nvidia.com/gpu",
						Operator: "Exists",
						Effect:   "NoSchedule",
					},
				},
			},
		},
	},
}
