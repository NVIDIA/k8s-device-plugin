/**
# Copyright 2023 NVIDIA CORPORATION
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

package nvml

// Library defines a set of functions defined on the underlying dynamic library.
type Library interface {
	Lookup(string) error
}

// dynamicLibrary is an interface for abstacting the underlying library.
// This also allows for mocking and testing.

//go:generate moq -stub -out dynamicLibrary_mock.go . dynamicLibrary
type dynamicLibrary interface {
	Lookup(string) error
	Open() error
	Close() error
}

// Interface represents the interface for the NVML library.
type Interface interface {
	GetLibrary() Library
}
