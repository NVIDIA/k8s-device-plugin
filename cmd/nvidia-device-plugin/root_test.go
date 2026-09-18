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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTryResolveLibrary(t *testing.T) {
	const libraryName = "libnvidia-ml.so.1"

	testCases := []struct {
		description string
		// dirs, files and target are created relative to the test root before resolving.
		dirs   []string
		files  []string
		link   string
		target string
		// expected is the resolved path relative to the test root, or "" if
		// the input library name is expected to be returned as is.
		expected string
	}{
		{
			description: "library in first search path",
			files:       []string{"usr/lib64/" + libraryName},
			expected:    "usr/lib64/" + libraryName,
		},
		{
			description: "directory in earlier search path does not shadow library",
			dirs:        []string{"usr/lib64/" + libraryName},
			files:       []string{"usr/lib/x86_64-linux-gnu/" + libraryName},
			expected:    "usr/lib/x86_64-linux-gnu/" + libraryName,
		},
		{
			description: "only directories found",
			dirs:        []string{"usr/lib64/" + libraryName},
			expected:    "",
		},
		{
			description: "symlink to a library is resolved",
			files:       []string{"usr/lib/x86_64-linux-gnu/" + libraryName + ".580.126.09"},
			link:        "usr/lib64/" + libraryName,
			target:      "usr/lib/x86_64-linux-gnu/" + libraryName + ".580.126.09",
			expected:    "usr/lib/x86_64-linux-gnu/" + libraryName + ".580.126.09",
		},
		{
			description: "library not found",
			expected:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rootDir := t.TempDir()

			for _, d := range tc.dirs {
				require.NoError(t, os.MkdirAll(filepath.Join(rootDir, d), 0755))
			}
			for _, f := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(rootDir, f)), 0755))
				require.NoError(t, os.WriteFile(filepath.Join(rootDir, f), []byte{}, 0644))
			}
			if tc.link != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(rootDir, tc.link)), 0755))
				require.NoError(t, os.Symlink(filepath.Join(rootDir, tc.target), filepath.Join(rootDir, tc.link)))
			}

			r := root(rootDir)
			resolved := r.tryResolveLibrary(libraryName)

			if tc.expected == "" {
				require.Equal(t, libraryName, resolved)
				return
			}
			require.Equal(t, filepath.Join(rootDir, tc.expected), resolved)
		})
	}
}
