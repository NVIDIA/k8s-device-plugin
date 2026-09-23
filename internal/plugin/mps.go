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

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
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

	// Skip resources with no shared (annotated) devices: the daemon manager
	// creates no daemon for them, so enabling MPS here would wait forever.
	if !rm.AnnotatedIDs(resourceManager.Devices().GetIDs()).AnyHasAnnotations() {
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
	// TODO: Check the .ready file here.
	// TODO: Have some retry strategy here.
	if err := m.daemon.AssertHealthy(); err != nil {
		return fmt.Errorf("error checking MPS daemon health: %w", err)
	}
	klog.InfoS("MPS daemon is healthy", "resource", m.resourceName)
	return nil
}

func (m *mpsOptions) updateReponse(response *pluginapi.ContainerAllocateResponse, ids []string) {
	if m == nil || !m.enabled {
		return
	}
	// TODO: We should check that the deviceIDs are shared using MPS.
	response.Envs["CUDA_MPS_PIPE_DIRECTORY"] = m.daemon.PipeDir()

	// Deliver the active thread percentage per client, since MPS has no
	// per-device thread command (unlike the pinned memory limit).
	if pct := m.activeThreadPercentage(ids); pct != "" {
		response.Envs["CUDA_MPS_ACTIVE_THREAD_PERCENTAGE"] = pct
	}

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

// activeThreadPercentage returns 100 / the largest replica count among ids, so a
// container spanning different counts is capped by its most-shared GPU. Empty if unknown.
func (m *mpsOptions) activeThreadPercentage(ids []string) string {
	devices := m.daemon.Devices()
	maxReplicas := 0
	for _, id := range ids {
		if d, ok := devices[id]; ok && d.Replicas > maxReplicas {
			maxReplicas = d.Replicas
		}
	}
	if maxReplicas < 1 {
		return ""
	}
	return fmt.Sprintf("%d", 100/maxReplicas)
}
