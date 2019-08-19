package main

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// Topology defined the whole topology for the node
type Topology struct {
	SystemInfo HostSystemInfo   `json:"systemInfo"`
	CPUInfo    HostCPUInfo      `json:"cpuInfo"`
	CPUPkg     []HostCPUPackage `json:"cpuPkg"`
	MemorySize int64            `json:"memorySize"`
	NumaInfo   *HostNumaInfo    `json:"numaInfo,omitempty"`
	SmcPresent *bool            `json:"smcPresent"`
	GPUDevice  []*nvml.Device   `json:"gpuDevice,omitempty"`
}

// HostSystemInfo define system info
type HostSystemInfo struct {
	Vendor       string `json:"vendor"`
	Model        string `json:"model"`
	UUID         string `json:"uuid"`
	SerialNumber string `json:"serialNumber,omitempty"`
	OSName       string `json:"osName"`
	OSRelease    string `json:"osRelease"`
	OSVersion    string `json:"osVersion"`
	Architecture string `json:"architecture"`
}

// HostCPUInfo define CPU info
type HostCPUInfo struct {
	NumCPUPackages int16
	NumCPUCores    int16
	NumCPUThreads  int16
	Hz             int64
}

// HostCPUPackage define CPU package
type HostCPUPackage struct {
	Index        int16
	Vendor       string
	FamilyNumber int16
	ModelNumber  int16
	Model        string
	Stepping     int16
}

// HostCPUCacheType define CPU cache type
type HostCPUCacheType int

const (
	// HostCPUL1Cache Level 1 Data (or Unified) Cache.
	HostCPUL1Cache HostCPUCacheType = iota
	// HostCPUL2Cache Level 2 Data (or Unified) Cache.
	HostCPUL2Cache
	// HostCPUL3Cache Level 3 Data (or Unified) Cache.
	HostCPUL3Cache
	// HostCPUL4Cache Level 4 Data (or Unified) Cache.
	HostCPUL4Cache
	// HostCPUL5Cache Level 5 Data (or Unified) Cache.
	HostCPUL5Cache
	// HostCPUL1iCache Level 1 instruction Cache.
	HostCPUL1iCache
	// HostCPUL2iCache Level 2 instruction Cache.
	HostCPUL2iCache
	// HostCPUL3iCache Level 3 instruction Cache.
	HostCPUL3iCache
)

type HostCPUCache struct {
	Size          uint64
	Depth         int
	LineSize      int
	Associativity int
	Type          HostCPUCacheType
}

type HostCPUCore struct {
	ProcessUnits []HostCPUProcessUnit
}

type HostCPUProcessUnit struct {
}

type HostNumaInfo struct {
	Type     string         `json:"type"`
	NumNodes int32          `json:"numNodes"`
	NumaNode []HostNumaNode `json:"numaNode,omitempty"`
}

// HostNumaNode defined numa node spec
type HostNumaNode struct {
	TypeID            byte    `json:"typeId"`
	CPUID             []int16 `json:"cpuID"`
	MemoryRangeBegin  int64   `json:"memoryRangeBegin"`
	MemoryRangeLength int64   `json:"memoryRangeLength"`
}
