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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

func TestDaemonStop(t *testing.T) {
	// Put a stand-in for the MPS control binary that accepts any command,
	// including quit, at the front of the PATH.
	binDir := t.TempDir()
	script := []byte("#!/bin/sh\ncat >/dev/null\n")
	require.NoError(t, os.WriteFile(filepath.Join(binDir, mpsControlBin), script, 0755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	resource := spec.ResourceName("example.com/gpu")

	testCases := []struct {
		description string
		logTailer   *tailer
		// startedFileIsDir makes removing the started file fail.
		startedFileIsDir bool
		expectedError    string
	}{
		{
			description: "start did not finish: no log tailer and no started file",
		},
		{
			description:      "started file cannot be removed",
			logTailer:        newTailer("control.log"),
			startedFileIsDir: true,
			expectedError:    "failed to remove started file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			root := Root(t.TempDir())
			d := NewDaemon(&rm.ResourceManagerMock{
				ResourceFunc: func() spec.ResourceName { return resource },
				DevicesFunc:  func() rm.Devices { return rm.Devices{} },
			}, root)
			d.logTailer = tc.logTailer

			if tc.startedFileIsDir {
				require.NoError(t, os.MkdirAll(filepath.Join(root.startedFile(resource), "child"), 0755))
			}

			var err error
			require.NotPanics(t, func() { err = d.Stop() })

			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
		})
	}
}
