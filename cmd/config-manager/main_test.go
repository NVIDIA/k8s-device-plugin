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
	"time"

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

func TestCmdlineMatchesProcessTarget(t *testing.T) {
	testCases := []struct {
		description   string
		argv0         string
		target        string
		expectedMatch bool
	}{
		{
			description:   "identical bare names",
			argv0:         "mps-control-daemon",
			target:        "mps-control-daemon",
			expectedMatch: true,
		},
		{
			description:   "identical absolute paths",
			argv0:         "/usr/bin/mps-control-daemon",
			target:        "/usr/bin/mps-control-daemon",
			expectedMatch: true,
		},
		{
			description:   "bare argv0 matches absolute target",
			argv0:         "mps-control-daemon",
			target:        "/usr/bin/mps-control-daemon",
			expectedMatch: true,
		},
		{
			description:   "absolute argv0 matches bare target",
			argv0:         "/usr/bin/mps-control-daemon",
			target:        "mps-control-daemon",
			expectedMatch: true,
		},
		{
			description:   "different directories with the same basename",
			argv0:         "/usr/local/bin/mps-control-daemon",
			target:        "/usr/bin/mps-control-daemon",
			expectedMatch: true,
		},
		{
			description:   "different basenames",
			argv0:         "/usr/bin/nvidia-device-plugin",
			target:        "/usr/bin/mps-control-daemon",
			expectedMatch: false,
		},
		{
			description:   "basename is a prefix of the target",
			argv0:         "mps-control",
			target:        "mps-control-daemon",
			expectedMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			require.Equal(t, tc.expectedMatch, cmdlineMatchesProcessTarget(tc.argv0, tc.target))
		})
	}
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

// Regression test for https://github.com/NVIDIA/k8s-device-plugin/issues/2066:
// the first Get() must block until the informer has observed the node, instead
// of returning the empty zero value from before the cache synced.
func TestGetBlocksUntilFirstSet(t *testing.T) {
	config := NewSyncableConfig(&Flags{})

	result := make(chan string, 1)
	go func() {
		result <- config.Get()
	}()

	select {
	case v := <-result:
		t.Fatalf("Get() returned %q before the informer observed the node", v)
	case <-time.After(100 * time.Millisecond):
	}

	config.Set("all-disabled")

	select {
	case v := <-result:
		require.Equal(t, "all-disabled", v)
	case <-time.After(5 * time.Second):
		t.Fatal("Get() did not return after Set()")
	}
}

// An observed node without the label at all still unblocks Get() with an empty
// value so the caller selects the default config; only "not observed yet"
// blocks.
func TestGetReturnsEmptyForObservedNodeWithoutLabel(t *testing.T) {
	config := NewSyncableConfig(&Flags{})
	config.Set("")

	result := make(chan string, 1)
	go func() {
		result <- config.Get()
	}()

	select {
	case v := <-result:
		require.Equal(t, "", v)
	case <-time.After(5 * time.Second):
		t.Fatal("Get() did not return for an observed node without a label")
	}
}
