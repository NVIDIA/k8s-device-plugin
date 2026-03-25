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

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSVFilesForRoot(t *testing.T) {
	testCases := []struct {
		description   string
		setup         func(t *testing.T) string // returns driverRoot
		expectedPaths func(driverRoot string) []string
	}{
		{
			description: "directory exists at driver root",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				csvDir := filepath.Join(root, defaultCSVMountSpecPath)
				require.NoError(t, os.MkdirAll(csvDir, 0755))
				return root
			},
			expectedPaths: func(driverRoot string) []string {
				csvDir := filepath.Join(driverRoot, defaultCSVMountSpecPath)
				return []string{
					filepath.Join(csvDir, "devices.csv"),
					filepath.Join(csvDir, "drivers.csv"),
					filepath.Join(csvDir, "l4t.csv"),
				}
			},
		},
		{
			description: "directory does not exist at driver root or absolute path",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			expectedPaths: func(_ string) []string {
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			driverRoot := tc.setup(t)
			got := csvFilesForRoot(driverRoot)
			require.Equal(t, tc.expectedPaths(driverRoot), got)
		})
	}
}
