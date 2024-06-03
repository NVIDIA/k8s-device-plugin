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

package resource

import (
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
	"github.com/NVIDIA/go-nvlib/pkg/nvlib/info"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"k8s.io/klog/v2"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// NewManager is a factory method that creates a resource Manager based on the specified config.
func NewManager(infolib info.Interface, nvmllib nvml.Interface, devicelib device.Interface, config *spec.Config) Manager {
	return WithConfig(getManager(infolib, nvmllib, devicelib, *config.Flags.Mode), config)
}

// WithConfig modifies a manager depending on the specified config.
// If failure on a call to init is allowed, the manager is wrapped to allow fallback to a Null manager.
func WithConfig(manager Manager, config *spec.Config) Manager {
	if *config.Flags.FailOnInitError {
		return manager
	}

	return NewFallbackToNullOnInitError(manager)
}

// getManager returns the resource manager depending on the system configuration.
func getManager(infolib info.Interface, nvmllib nvml.Interface, devicelib device.Interface, mode string) Manager {

	resolved := resolveMode(infolib, mode)
	switch resolved {
	case "nvml":
		klog.Info("Using NVML manager")
		return NewNVMLManager(nvmllib, devicelib)
	case "tegra":
		klog.Info("Using CUDA manager")
		return NewCudaManager()
	case "vfio":
		klog.Info("Using Vfio manager")
		return NewVfioManager()
	}

	klog.Warningf("Unsupported mode detected: %v using empty manager.", resolved)
	return NewNullManager()
}

func resolveMode(infolib info.Interface, mode string) string {
	if mode != "" && mode != "auto" {
		return mode
	}

	// logWithReason logs the output of the has* / is* checks from the info.Interface
	logWithReason := func(f func() (bool, string), tag string) bool {
		is, reason := f()
		if !is {
			tag = "non-" + tag
		}
		klog.Infof("Detected %v platform: %v", tag, reason)
		return is
	}

	hasNVML := logWithReason(infolib.HasNvml, "NVML")
	isTegra := logWithReason(infolib.HasTegraFiles, "Tegra")

	// The NVIDIA container stack does not yet support the use of integrated AND discrete GPUs on the same node.
	if hasNVML && isTegra {
		klog.Warning("Disabling Tegra-based resources on NVML system")
		isTegra = false
	}

	if hasNVML {
		return "nvml"
	}

	if isTegra {
		return "tegra"
	}
	return mode
}
