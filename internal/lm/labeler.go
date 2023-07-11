/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package lm

import (
	"fmt"

	"github.com/NVIDIA/gpu-feature-discovery/internal/resource"
	"github.com/NVIDIA/gpu-feature-discovery/internal/vgpu"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// Labeler defines an interface for generating labels
type Labeler interface {
	Labels() (Labels, error)
}

// NewLabelers constructs the required labelers from the specified config
func NewLabelers(manager resource.Manager, vgpu vgpu.Interface, config *spec.Config) (Labeler, error) {
	nvmlLabeler, err := NewNVMLLabeler(manager, config)
	if err != nil {
		return nil, fmt.Errorf("error creating NVML labeler: %v", err)
	}

	l := Merge(
		nvmlLabeler,
		NewVGPULabeler(vgpu),
	)

	return l, nil
}
