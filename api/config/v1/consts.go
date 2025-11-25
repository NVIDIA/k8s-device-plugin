/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

package v1

import (
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
)

// Constants related to resource names
const (
	ResourceNamePrefix              = "nvidia.com"
	DefaultSharedResourceNameSuffix = ".shared"
	MaxResourceNameLength           = 63
)

// Constants representing the various MIG strategies
const (
	MigStrategyNone   = "none"
	MigStrategySingle = "single"
	MigStrategyMixed  = "mixed"
)

// Constants to represent the various device list strategies
const (
	DeviceListStrategyEnvVar         = "envvar"
	DeviceListStrategyVolumeMounts   = "volume-mounts"
	DeviceListStrategyCDIAnnotations = "cdi-annotations"
	DeviceListStrategyCDICRI         = "cdi-cri"
)

// Constants to represent the various device id strategies
const (
	DeviceIDStrategyUUID  = "uuid"
	DeviceIDStrategyIndex = "index"
)

// Constants related to generating CDI specifications
const (
	DefaultCDIAnnotationPrefix = cdiapi.AnnotationPrefix
	DefaultNvidiaCTKPath       = "/usr/bin/nvidia-ctk"
	DefaultContainerDriverRoot = "/driver-root"
)

// Command line flag names - Common flags
const (
	FlagMigStrategy             = "mig-strategy"
	FlagFailOnInitError         = "fail-on-init-error"
	FlagMpsRoot                 = "mps-root"
	FlagDriverRoot              = "driver-root"
	FlagNvidiaDriverRoot        = "nvidia-driver-root"
	FlagDevRoot                 = "dev-root"
	FlagNvidiaDevRoot           = "nvidia-dev-root"
	FlagGDRCopyEnabled          = "gdrcopy-enabled"
	FlagGDSEnabled              = "gds-enabled"
	FlagMOFEDEnabled            = "mofed-enabled"
	FlagUseNodeFeatureAPI       = "use-node-feature-api"
	FlagDeviceDiscoveryStrategy = "device-discovery-strategy"
	FlagConfigFile              = "config-file"
)

// Command line flag names - Plugin specific flags
const (
	FlagPassDeviceSpecs     = "pass-device-specs"
	FlagDeviceListStrategy  = "device-list-strategy"
	FlagDeviceIDStrategy    = "device-id-strategy"
	FlagCDIAnnotationPrefix = "cdi-annotation-prefix"
	FlagNvidiaCDIHookPath   = "nvidia-cdi-hook-path"
	FlagNvidiaCTKPath       = "nvidia-ctk-path"
	FlagContainerDriverRoot = "container-driver-root"
	FlagDriverRootCtrPath   = "driver-root-ctr-path"
)

// Command line flag names - GFD specific flags
const (
	FlagOneshot         = "oneshot"
	FlagNoTimestamp     = "no-timestamp"
	FlagSleepInterval   = "sleep-interval"
	FlagOutputFile      = "output-file"
	FlagMachineTypeFile = "machine-type-file"
)

// Command line flag names - IMEX specific flags
const (
	FlagImexChannelIDs = "imex-channel-ids"
	FlagImexRequired   = "imex-required"
)

// Command line flag names - Plugin additional flags
const (
	FlagKubeletSocket   = "kubelet-socket"
	FlagCDIFeatureFlags = "cdi-feature-flags"
)

// Command line flag names - Config manager specific flags
const (
	FlagKubeconfig         = "kubeconfig"
	FlagNodeName           = "node-name"
	FlagNodeLabel          = "node-label"
	FlagConfigFileSrcdir   = "config-file-srcdir"
	FlagConfigFileDst      = "config-file-dst"
	FlagDefaultConfig      = "default-config"
	FlagFallbackStrategies = "fallback-strategies"
	FlagSendSignal         = "send-signal"
	FlagSignal             = "signal"
	FlagProcessToSignal    = "process-to-signal"
)
