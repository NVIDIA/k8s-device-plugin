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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

const (
	ReadyFilePath = "/mps/.ready"
)

// ReadyFile represents a file used to store readyness of the MPS daemon.
type ReadyFile struct{}

// Remove the ready file.
func (f ReadyFile) Remove() error {
	err := os.Remove(ReadyFilePath)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Save writes the specified config to the ready file.
func (f ReadyFile) Save(config *spec.Config) error {
	readyFile, err := os.Create(ReadyFilePath)
	if err != nil {
		return fmt.Errorf("failed to create .ready file: %w", err)
	}
	defer readyFile.Close()

	data := &spec.ReplicatedResources{}
	if config != nil && config.Sharing.MPS != nil {
		data = config.Sharing.MPS
	}
	if err := json.NewEncoder(readyFile).Encode(data); err != nil {
		return fmt.Errorf("failed to write .ready file: %w", err)
	}
	return nil
}

// Load loads the contents of th read file.
func (f ReadyFile) Load() (*spec.Config, error) {
	readyFile, err := os.Open(ReadyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .ready file: %w", err)
	}
	defer readyFile.Close()

	var readyConfig spec.Config
	if err := json.NewDecoder(readyFile).Decode(&readyConfig); err != nil {
		return nil, fmt.Errorf("faled to load .ready config: %w", err)
	}
	return &readyConfig, nil
}

// Matches checks whether the contents of the ready file matches the specified config.
func (f ReadyFile) Matches(config *spec.Config) (bool, error) {
	readyConfig, err := f.Load()
	if err != nil {
		return false, err
	}
	if config == nil {
		return readyConfig == nil, nil
	}
	if readyConfig == nil {
		return false, nil
	}
	return reflect.DeepEqual(config.Sharing.MPS, readyConfig.Sharing.MPS), nil
}
