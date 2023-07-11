// Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/NVIDIA/gpu-feature-discovery/internal/resource"
	rt "github.com/NVIDIA/gpu-feature-discovery/internal/resource/testing"
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/stretchr/testify/require"
)

func TestMigStrategyNone(t *testing.T) {
	devices := []resource.Device{
		rt.NewMigEnabledDevice(
			rt.NewMigDevice(3, 0, 20000),
			rt.NewMigDevice(3, 0, 20000),
		),
	}
	nvmlMock := rt.NewManagerMockWithDevices(devices...)
	// create VGPU mock library with empty vgpu devices
	vgpuMock := NewTestVGPUMock()

	conf := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				MigStrategy:     ptr("none"),
				FailOnInitError: ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-none"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	restart, err := run(nvmlMock, vgpuMock, conf, nil)
	require.NoError(t, err, "Error from run function")
	require.False(t, restart)

	outFile, err := os.Open(*conf.Flags.GFD.OutputFile)
	require.NoError(t, err, "Opening output file")

	defer func() {
		err = outFile.Close()
		require.NoError(t, err, "Closing output file")
		err = os.Remove(*conf.Flags.GFD.OutputFile)
		require.NoError(t, err, "Removing output file")
	}()

	output, err := io.ReadAll(outFile)
	require.NoError(t, err, "Reading output file")

	err = checkResult(output, cfg.Path("tests/expected-output-mig-none.txt"), false)
	require.NoError(t, err, "Checking result")

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	require.Equal(t, labels["nvidia.com/gpu.count"], "1", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.product"], "MOCKMODEL", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.memory"], "300", "Incorrect label")
}

func TestMigStrategySingleForNoMigDevices(t *testing.T) {
	nvmlMock := NewTestNvmlMock()
	// create VGPU mock library with empty vgpu devices
	vgpuMock := NewTestVGPUMock()

	conf := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				MigStrategy:     ptr("single"),
				FailOnInitError: ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-single-no-mig"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	restart, err := run(nvmlMock, vgpuMock, conf, nil)
	require.NoError(t, err, "Error from run function")
	require.False(t, restart)

	outFile, err := os.Open(*conf.Flags.GFD.OutputFile)
	require.NoError(t, err, "Opening output file")

	defer func() {
		err = outFile.Close()
		require.NoError(t, err, "Closing output file")
		err = os.Remove(*conf.Flags.GFD.OutputFile)
		require.NoError(t, err, "Removing output file")
	}()

	output, err := io.ReadAll(outFile)
	require.NoError(t, err, "Reading output file")

	err = checkResult(output, cfg.Path("tests/expected-output-mig-single.txt"), false)
	require.NoError(t, err, "Checking result")

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	require.Equal(t, labels["nvidia.com/mig.strategy"], "single", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.count"], "1", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.product"], "MOCKMODEL", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.memory"], "300", "Incorrect label")
}

func TestMigStrategySingleForMigDeviceMigDisabled(t *testing.T) {
	// create VGPU mock library with empty vgpu devices
	vgpuMock := NewTestVGPUMock()
	devices := []resource.Device{
		rt.NewDeviceMock(false).WithMigDevices(
			rt.NewMigDevice(3, 0, 20000),
			rt.NewMigDevice(3, 0, 20000),
		),
	}
	nvmlMock := rt.NewManagerMockWithDevices(devices...)

	conf := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				MigStrategy:     ptr("single"),
				FailOnInitError: ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-single-no-mig"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	restart, err := run(nvmlMock, vgpuMock, conf, nil)
	require.NoError(t, err, "Error from run function")
	require.False(t, restart)

	outFile, err := os.Open(*conf.Flags.GFD.OutputFile)
	require.NoError(t, err, "Opening output file")

	defer func() {
		err = outFile.Close()
		require.NoError(t, err, "Closing output file")
		err = os.Remove(*conf.Flags.GFD.OutputFile)
		require.NoError(t, err, "Removing output file")
	}()

	output, err := io.ReadAll(outFile)
	require.NoError(t, err, "Reading output file")

	err = checkResult(output, cfg.Path("tests/expected-output-mig-single.txt"), false)
	require.NoError(t, err, "Checking result")

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	require.Equal(t, labels["nvidia.com/mig.strategy"], "single", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.count"], "1", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.product"], "MOCKMODEL", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.memory"], "300", "Incorrect label")
}

func TestMigStrategySingle(t *testing.T) {
	// create VGPU mock library with empty vgpu devices
	vgpuMock := NewTestVGPUMock()
	devices := []resource.Device{
		rt.NewMigEnabledDevice(
			rt.NewMigDevice(3, 0, 20),
			rt.NewMigDevice(3, 0, 20),
		),
	}
	nvmlMock := rt.NewManagerMockWithDevices(devices...)

	conf := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				MigStrategy:     ptr("single"),
				FailOnInitError: ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-single"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	restart, err := run(nvmlMock, vgpuMock, conf, nil)
	require.NoError(t, err, "Error from run function")
	require.False(t, restart)

	outFile, err := os.Open(*conf.Flags.GFD.OutputFile)
	require.NoError(t, err, "Opening output file")

	defer func() {
		err = outFile.Close()
		require.NoError(t, err, "Closing output file")
		err = os.Remove(*conf.Flags.GFD.OutputFile)
		require.NoError(t, err, "Removing output file")
	}()

	output, err := io.ReadAll(outFile)
	require.NoError(t, err, "Reading output file")

	err = checkResult(output, cfg.Path("tests/expected-output-mig-single.txt"), false)
	require.NoError(t, err, "Checking result")

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	require.Equal(t, labels["nvidia.com/mig.strategy"], "single", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.count"], "2", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.product"], "MOCKMODEL-MIG-3g.20gb", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.memory"], "20", "Incorrect label")
}

func TestMigStrategyMixed(t *testing.T) {
	// create VGPU mock library with empty vgpu devices
	vgpuMock := NewTestVGPUMock()
	devices := []resource.Device{
		rt.NewMigEnabledDevice(
			rt.NewMigDevice(3, 0, 20),
			rt.NewMigDevice(1, 0, 5),
		),
	}

	nvmlMock := rt.NewManagerMockWithDevices(devices...)

	conf := &spec.Config{
		Flags: spec.Flags{
			CommandLineFlags: spec.CommandLineFlags{
				MigStrategy:     ptr("mixed"),
				FailOnInitError: ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-mixed"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	restart, err := run(nvmlMock, vgpuMock, conf, nil)
	require.NoError(t, err, "Error from run function")
	require.False(t, restart)

	outFile, err := os.Open(*conf.Flags.GFD.OutputFile)
	require.NoError(t, err, "Opening output file")

	defer func() {
		err = outFile.Close()
		require.NoError(t, err, "Closing output file")
		err = os.Remove(*conf.Flags.GFD.OutputFile)
		require.NoError(t, err, "Removing output file")
	}()

	output, err := io.ReadAll(outFile)
	require.NoError(t, err, "Reading output file")

	err = checkResult(output, cfg.Path("tests/expected-output-mig-mixed.txt"), false)
	require.NoError(t, err, "Checking result")

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	require.Equal(t, labels["nvidia.com/mig.strategy"], "mixed", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.count"], "1", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.product"], "MOCKMODEL", "Incorrect label")
	require.Equal(t, labels["nvidia.com/gpu.memory"], "300", "Incorrect label")
	require.Contains(t, labels, "nvidia.com/mig-3g.20gb.count", "Missing label")
	require.Contains(t, labels, "nvidia.com/mig-1g.5gb.count", "Missing label")
}
