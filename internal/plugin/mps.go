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

package plugin

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

const (
	// mpsReadyCheckInterval is how often we poll for MPS daemon readiness.
	mpsReadyCheckInterval = 5 * time.Second
	// mpsReadyCheckTimeout bounds how long we wait for the MPS daemon to become
	// ready before giving up. On timeout the caller (plugin startup) fails and
	// is retried by the plugin manager, so this is an upper bound per attempt.
	mpsReadyCheckTimeout = 5 * time.Minute
)

type mpsOptions struct {
	enabled      bool
	resourceName spec.ResourceName
	daemon       *mps.Daemon
	hostRoot     mps.Root
}

// getMPSOptions returns the MPS options specified for the resource manager.
// If MPS is not configured and empty set of options is returned.
func (o *options) getMPSOptions(resourceManager rm.ResourceManager) (mpsOptions, error) {
	if o.config.Sharing.SharingStrategy() != spec.SharingStrategyMPS {
		return mpsOptions{}, nil
	}

	// TODO: It might make sense to pull this logic into a resource manager.
	for _, device := range resourceManager.Devices() {
		if device.IsMigDevice() {
			return mpsOptions{}, errors.New("sharing using MPS is not supported for MIG devices")
		}
	}

	m := mpsOptions{
		enabled:      true,
		resourceName: resourceManager.Resource(),
		daemon:       mps.NewDaemon(resourceManager, mps.ContainerRoot),
		hostRoot:     mps.Root(*o.config.Flags.MpsRoot),
	}
	return m, nil
}

func (m *mpsOptions) waitForDaemon() error {
	if m == nil || !m.enabled {
		return nil
	}

	deadline := time.Now().Add(mpsReadyCheckTimeout)
	for {
		err := m.checkDaemonReady()
		if err == nil {
			klog.InfoS("MPS daemon is ready", "resource", m.resourceName)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for MPS daemon for %v: %w", m.resourceName, err)
		}
		klog.InfoS("Waiting for MPS daemon to be ready", "resource", m.resourceName, "reason", err)
		time.Sleep(mpsReadyCheckInterval)
	}
}

// checkDaemonReady reports whether the MPS daemon has finished initialization.
// It requires both that the daemon has signalled readiness via its .ready file
// (created only after compute mode, pinned memory limits, and thread
// percentages are applied) and that the control pipe is responsive. Checking
// only AssertHealthy is insufficient: the pipe becomes responsive before the
// per-device configuration is in place, so a pod scheduled in that window
// could run without the configured MPS memory/thread limits.
func (m *mpsOptions) checkDaemonReady() error {
	if !m.daemon.Ready() {
		return fmt.Errorf("MPS daemon has not signalled readiness")
	}
	if err := m.daemon.AssertHealthy(); err != nil {
		return fmt.Errorf("MPS daemon is not healthy: %w", err)
	}
	return nil
}

func (m *mpsOptions) updateReponse(response *pluginapi.ContainerAllocateResponse) {
	if m == nil || !m.enabled {
		return
	}
	// TODO: We should check that the deviceIDs are shared using MPS.
	response.Envs["CUDA_MPS_PIPE_DIRECTORY"] = m.daemon.PipeDir()

	response.Mounts = append(response.Mounts,
		&pluginapi.Mount{
			ContainerPath: m.daemon.PipeDir(),
			HostPath:      m.hostRoot.PipeDir(m.resourceName),
		},
		&pluginapi.Mount{
			ContainerPath: m.daemon.ShmDir(),
			HostPath:      m.hostRoot.ShmDir(m.resourceName),
		},
	)
}
