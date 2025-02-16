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

package cdi

import "github.com/NVIDIA/nvidia-container-toolkit/pkg/nvcdi/spec"

// Interface provides the API to the 'cdi' package
//
//go:generate moq -stub -out api_mock.go . Interface
type Interface interface {
	CreateSpecFile() error
	QualifiedName(string, string) string
}

type cdiSpecGenerator interface {
	GetSpec() (spec.Interface, error)
}
