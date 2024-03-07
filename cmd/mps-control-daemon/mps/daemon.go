/**
# Copyright 2024 NVIDIA CORPORATION
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

package mps

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"k8s.io/klog/v2"

	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

type computeMode string

const (
	mpsControlBin = "nvidia-cuda-mps-control"

	computeModeExclusiveProcess = computeMode("EXCLUSIVE_PROCESS")
	computeModeDefault          = computeMode("DEFAULT")
)

// Daemon represents an MPS daemon.
// It is associated with a specific kubernets resource and is responsible for
// starting and stopping the deamon as well as ensuring that the memory and
// thread limits are set for the devices that the resource makes available.
type Daemon struct {
	rm rm.ResourceManager
	// root represents the root at which the files and folders controlled by the
	// daemon are created. These include the log and pipe directories.
	root Root
}

// NewDaemon creates an MPS daemon instance.
func NewDaemon(rm rm.ResourceManager, root Root) *Daemon {
	return &Daemon{
		rm:   rm,
		root: root,
	}
}

// Devices returns the list of devices under the control of this MPS daemon.
func (d *Daemon) Devices() rm.Devices {
	return d.rm.Devices()
}

type envvars map[string]string

func (e envvars) toSlice() []string {
	var envs []string
	for k, v := range e {
		envs = append(envs, k+"="+v)
	}
	return envs
}

// Envvars returns the environment variables required for the daemon.
// These should be passed to clients consuming the device shared using MPS.
// TODO: Set CUDA_VISIBLE_DEVICES to include only the devices for this resource type.
func (d *Daemon) Envvars() envvars {
	return map[string]string{
		"CUDA_MPS_PIPE_DIRECTORY": d.PipeDir(),
		"CUDA_MPS_LOG_DIRECTORY":  d.LogDir(),
	}
}

// Start starts the MPS deamon as a background process.
func (d *Daemon) Start() error {
	if err := d.setComputeMode(computeModeExclusiveProcess); err != nil {
		return fmt.Errorf("error setting compute mode %v: %w", computeModeExclusiveProcess, err)
	}

	klog.InfoS("Staring MPS daemon", "resource", d.rm.Resource())

	pipeDir := d.PipeDir()
	if err := os.MkdirAll(pipeDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %v: %w", pipeDir, err)
	}

	logDir := d.LogDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %v: %w", logDir, err)
	}

	mpsDaemon := exec.Command(mpsControlBin, "-d")
	mpsDaemon.Env = append(mpsDaemon.Env, d.Envvars().toSlice()...)
	if err := mpsDaemon.Run(); err != nil {
		return err
	}

	statusFile, err := os.Create(d.startedFile())
	if err != nil {
		return err
	}
	defer statusFile.Close()

	return nil
}

// Stop ensures that the MPS daemon is quit.
func (d *Daemon) Stop() error {
	output, err := d.EchoPipeToControl("quit")
	if err != nil {
		return fmt.Errorf("error sending quit message: %w", err)
	}
	klog.InfoS("Shut down MPS", "output", output)

	if err := d.setComputeMode(computeModeDefault); err != nil {
		return fmt.Errorf("error setting compute mode %v: %w", computeModeDefault, err)
	}

	err = os.Remove(d.startedFile())
	if err != nil && err != os.ErrNotExist {
		return fmt.Errorf("failed to remove started file: %w", err)
	}
	return nil
}

func (d *Daemon) LogDir() string {
	return d.root.LogDir(d.rm.Resource())
}

func (d *Daemon) PipeDir() string {
	return d.root.PipeDir(d.rm.Resource())
}

func (d *Daemon) ShmDir() string {
	return "/dev/shm"
}

func (d *Daemon) startedFile() string {
	return d.root.startedFile(d.rm.Resource())
}

// AssertHealthy checks that the MPS control daemon is healthy.
func (d *Daemon) AssertHealthy() error {
	_, err := d.EchoPipeToControl("get_default_active_thread_percentage")
	return err
}

// EchoPipeToControl sends the specified command to the MPS control daemon.
func (d *Daemon) EchoPipeToControl(command string) (string, error) {
	var out bytes.Buffer
	reader, writer := io.Pipe()
	defer writer.Close()
	defer reader.Close()

	mpsDaemon := exec.Command(mpsControlBin)
	mpsDaemon.Env = append(mpsDaemon.Env, d.Envvars().toSlice()...)

	mpsDaemon.Stdin = reader
	mpsDaemon.Stdout = &out

	if err := mpsDaemon.Start(); err != nil {
		return "", fmt.Errorf("failed to start NVIDIA MPS command: %w", err)
	}

	if _, err := writer.Write([]byte(command)); err != nil {
		return "", fmt.Errorf("failed to write message to pipe: %w", err)
	}
	_ = writer.Close()

	if err := mpsDaemon.Wait(); err != nil {
		return "", fmt.Errorf("failed to send command to MPS daemon: %w", err)
	}
	return out.String(), nil
}

func (d *Daemon) setComputeMode(mode computeMode) error {
	for _, uuid := range d.Devices().GetUUIDs() {
		cmd := exec.Command(
			"nvidia-smi",
			"-i", uuid,
			"-c", string(mode))
		output, err := cmd.CombinedOutput()
		if err != nil {
			klog.Errorf("\n%v", string(output))
			return fmt.Errorf("error running nvidia-smi: %w", err)
		}
	}
	return nil
}
