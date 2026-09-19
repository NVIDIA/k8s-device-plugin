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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestFlags(srcdir, dst string) *Flags {
	return &Flags{
		ConfigFileSrcdir: srcdir,
		ConfigFileDst:    dst,
	}
}

func newSymlinkTestFixture(t *testing.T) (string, *Flags) {
	srcdir := t.TempDir()
	dst := filepath.Join(t.TempDir(), "config.yaml")
	return srcdir, newTestFlags(srcdir, dst)
}

func TestUpdateSymlinkDanglingDestination(t *testing.T) {
	t.Run("create dangling symlink", func(t *testing.T) {
		srcdir, f := newSymlinkTestFixture(t)

		changed, err := updateSymlink("missing-config", f)
		require.NoError(t, err)
		require.True(t, changed)

		link, err := os.Readlink(f.ConfigFileDst)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(srcdir, "missing-config"), link)
	})

	t.Run("dangling symlink already pointing at config is a no operation", func(t *testing.T) {
		srcdir, f := newSymlinkTestFixture(t)

		_, err := updateSymlink("missing-config", f)
		require.NoError(t, err)

		changed, err := updateSymlink("missing-config", f)
		require.NoError(t, err)
		require.False(t, changed)

		link, err := os.Readlink(f.ConfigFileDst)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(srcdir, "missing-config"), link)
	})
}

func TestUpdateConfigName(t *testing.T) {
	testCases := []struct {
		name           string
		config         string
		defaultConfig  string
		expectedConfig string
		expectedError  string
	}{
		{
			name:          "missing default configuration",
			defaultConfig: "missing-profile",
			expectedError: "specified config missing-profile does not exist",
		},
		{
			name:           "existing default configuration",
			defaultConfig:  "default-profile",
			expectedConfig: "default-profile",
		},
		{
			name:           "node configuration overrides default",
			config:         "node-profile",
			defaultConfig:  "default-profile",
			expectedConfig: "node-profile",
		},
		{
			name:           "node configuration overrides missing default",
			config:         "node-profile",
			defaultConfig:  "missing-profile",
			expectedConfig: "node-profile",
		},
		{
			name:          "missing node configuration does not fall back to default",
			config:        "missing-node-profile",
			defaultConfig: "default-profile",
			expectedError: "specified config missing-node-profile does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srcdir := t.TempDir()
			for _, name := range []string{"default-profile", "node-profile"} {
				require.NoError(t, os.WriteFile(filepath.Join(srcdir, name), []byte("version: v1"), 0600))
			}
			f := &Flags{
				ConfigFileSrcdir: srcdir,
				DefaultConfig:    tc.defaultConfig,
			}

			config, err := updateConfigName(tc.config, f)
			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedConfig, config)
		})
	}
}
