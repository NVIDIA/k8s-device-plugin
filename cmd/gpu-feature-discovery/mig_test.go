// Copyright (c) 2019, NVIDIA CORPORATION. All rights reserved.

package main

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/flags"
	"github.com/NVIDIA/k8s-device-plugin/internal/lm"
	"github.com/NVIDIA/k8s-device-plugin/internal/resource"
	rt "github.com/NVIDIA/k8s-device-plugin/internal/resource/testing"
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

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

func TestMigStrategySingleWithCustomPrefix(t *testing.T) {
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
				MigStrategy:       ptr("single"),
				ResourceNamePrefix: ptr("custom.domain"),
				FailOnInitError:   ptr(true),
				GFD: &spec.GFDCommandLineFlags{
					Oneshot:         ptr(true),
					OutputFile:      ptr("./gfd-test-mig-single-custom"),
					SleepInterval:   ptr(spec.Duration(time.Second)),
					NoTimestamp:     ptr(false),
					MachineTypeFile: ptr(testMachineTypeFile),
				},
			},
		},
	}

	setupMachineFile(t)
	defer removeMachineFile(t)

	labelOutputer, err := lm.NewOutputer(conf, flags.NodeConfig{}, flags.ClientSets{})
	require.NoError(t, err)

	d := gfd{
		manager:       nvmlMock,
		vgpu:          vgpuMock,
		config:        conf,
		labelOutputer: labelOutputer,
	}
	restart, err := d.run(nil)
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

	labels, err := buildLabelMapFromOutput(output)
	require.NoError(t, err, "Building map of labels from output file")

	// Verify custom prefix is used in labels
	require.Equal(t, labels["custom.domain/mig.strategy"], "single", "Incorrect label")
	require.Equal(t, labels["custom.domain/gpu.count"], "2", "Incorrect label")
	require.Equal(t, labels["custom.domain/gpu.product"], "MOCKMODEL-MIG-3g.20gb", "Incorrect label")
	require.Equal(t, labels["custom.domain/gpu.memory"], "20", "Incorrect label")

	// Verify default nvidia.com labels are NOT present
	require.NotContains(t, labels, "nvidia.com/mig.strategy", "Default prefix should not be present")
	require.NotContains(t, labels, "nvidia.com/gpu.count", "Default prefix should not be present")
}
