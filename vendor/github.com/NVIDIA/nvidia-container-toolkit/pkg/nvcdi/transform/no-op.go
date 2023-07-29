/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
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

package transform

import (
	"github.com/container-orchestrated-devices/container-device-interface/specs-go"
)

type noop struct{}

var _ Transformer = (*noop)(nil)

// NewNoopTransformer returns a no-op transformer
func NewNoopTransformer() Transformer {
	return noop{}
}

// Transform is a no-op
func (n noop) Transform(spec *specs.Spec) error {
	return nil
}
