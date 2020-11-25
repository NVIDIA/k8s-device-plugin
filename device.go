/*
 * Copyright (c) 2019, 2020 NVIDIA CORPORATION.  All rights reserved.
 * Copyright (c) 2020 Adaptant Solutions AG
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

package main

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"log"
	"os"
)

type DeviceType int

const (
	DeviceTypeFullGPU DeviceType = iota
	DeviceTypeTegra
)

type DeviceControl interface {
	Init() error
	Shutdown()
}

type NvmlDeviceControl struct {}

func (n NvmlDeviceControl) Init() error {
	log.Println("Loading NVML")
	if err := nvml.Init(); err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("Failed to initialize NVML: %v.", err)
		return err
	}

	return nil
}

func (n NvmlDeviceControl) Shutdown() {
	log.Println("Shutdown of NVML returned:", nvml.Shutdown())
}

type TegraDeviceControl struct {}

func (t TegraDeviceControl) Init() error {
	return nil
}

func (t TegraDeviceControl) Shutdown() {
	// Nothing to do
}