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
	"fmt"

	"github.com/NVIDIA/go-nvlib/pkg/nvml"
	"k8s.io/klog/v2"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

type Manager interface {
	Daemons() ([]*Daemon, error)
}

type manager struct {
	config  *spec.Config
	nvmllib nvml.Interface
}

type nullManager struct{}

// Daemons creates the required set of MPS daemons for the specified options.
func NewDaemons(opts ...Option) ([]*Daemon, error) {
	manager, err := New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create MPS manager: %w", err)
	}
	return manager.Daemons()
}

// New creates a manager for MPS daemons.
// If MPS is not configured, a manager is returned that manages no daemons.
func New(opts ...Option) (Manager, error) {
	m := &manager{}
	for _, opt := range opts {
		opt(m)
	}

	if strategy := m.config.Sharing.SharingStrategy(); strategy != spec.SharingStrategyMPS {
		klog.InfoS("Sharing strategy is not MPS; skipping MPS manager creation", "strategy", strategy)
		return &nullManager{}, nil
	}

	// TODO: This should be controllable via an option
	if m.nvmllib == nil {
		driverLibraryPath, err := root("/driver-root").getDriverLibraryPath()
		if err != nil {
			return nil, fmt.Errorf("failed to locate driver libraries: %w", err)
		}
		m.nvmllib = nvml.New(nvml.WithLibraryPath(driverLibraryPath))
	}

	return m, nil
}

func (m *manager) Daemons() ([]*Daemon, error) {
	resourceManagers, err := rm.NewNVMLResourceManagers(m.nvmllib, m.config)
	if err != nil {
		return nil, err
	}
	var daemons []*Daemon
	for _, resourceManager := range resourceManagers {
		// We don't create daemons if there are no devices associated with the resource manager.
		if len(resourceManager.Devices()) == 0 {
			klog.InfoS("No devices associated with resource", "resource", resourceManager.Resource())
			continue
		}
		// Check if the resources are shared.
		// TODO: We should add a more explicit check for MPS specifically
		if !rm.AnnotatedIDs(resourceManager.Devices().GetIDs()).AnyHasAnnotations() {
			klog.InfoS("Resource is not shared", "resource", "resource", resourceManager.Resource())
			continue
		}
		daemon := NewDaemon(resourceManager)
		daemons = append(daemons, daemon)
	}

	return daemons, nil
}

// Daemons always returns an empty slice for a nullManager.
func (m *nullManager) Daemons() ([]*Daemon, error) {
	return nil, nil
}
