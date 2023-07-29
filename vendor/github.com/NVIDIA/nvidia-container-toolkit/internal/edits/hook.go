/**
# Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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

package edits

import (
	"github.com/NVIDIA/nvidia-container-toolkit/internal/discover"
	"github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/container-orchestrated-devices/container-device-interface/specs-go"
)

type hook discover.Hook

// toEdits converts a discovered hook to CDI Container Edits.
func (d hook) toEdits() *cdi.ContainerEdits {
	e := cdi.ContainerEdits{
		ContainerEdits: &specs.ContainerEdits{
			Hooks: []*specs.Hook{d.toSpec()},
		},
	}
	return &e
}

// toSpec converts a discovered Hook to a CDI Spec Hook. Note
// that missing info is filled in when edits are applied by querying the Hook node.
func (d hook) toSpec() *specs.Hook {
	s := specs.Hook{
		HookName: d.Lifecycle,
		Path:     d.Path,
		Args:     d.Args,
	}

	return &s
}
