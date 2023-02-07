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

package cdi

import (
	"log"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"gitlab.com/nvidia/cloud-native/go-nvlib/pkg/nvlib/info"
)

// New is a factory method that creates a CDI handler for creating CDI specs.
func New(flags spec.Flags, opts ...Option) (Interface, error) {
	if *flags.Plugin.DeviceListStrategy != spec.DeviceListStrategyCDIAnnotations {
		log.Println("CDI is not enabled; using an empty CDI handler")
		return newNullHandler(), nil
	}

	infolib := info.New()

	hasNVML, _ := infolib.HasNvml()
	if hasNVML {
		log.Println("Creating a CDI handler")
		return newHandler(opts...)
	}

	log.Println("No valid resources detected; using an empty CDI handler")
	return newNullHandler(), nil
}
