/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-orchestrated-devices/container-device-interface/specs-go"
	"gopkg.in/yaml.v2"
)

type cdiSpec specs.Spec

// write the CDI Spec to the file associated with it during instantiation
// by newSpec() or ReadSpec().
func (s *cdiSpec) write(path string) error {
	var (
		data []byte
		dir  string
		tmp  *os.File
		err  error
	)

	// TODO: Add validation of the Spec
	// err = validateSpec(s)
	// if err != nil {
	// 	return err
	// }

	if filepath.Ext(path) == ".yaml" {
		data, err = yaml.Marshal(s)
	} else {
		data, err = json.Marshal(s)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal Spec file: %w", err)
	}

	dir = filepath.Dir(path)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create Spec dir: %w", err)
	}

	tmp, err = os.CreateTemp(dir, "spec.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create Spec file: %w", err)
	}
	_, err = tmp.Write(data)
	tmp.Close()
	if err != nil {
		return fmt.Errorf("failed to write Spec file: %w", err)
	}

	err = renameIn(dir, filepath.Base(tmp.Name()), filepath.Base(path), true)

	if err != nil {
		os.Remove(tmp.Name())
		err = fmt.Errorf("failed to write Spec file: %w", err)
	}

	return err
}
