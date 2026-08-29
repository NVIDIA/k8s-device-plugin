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

func TestUpdateSymlinkDanglingDestination(t *testing.T) {
	srcdir := t.TempDir()
	dst := filepath.Join(t.TempDir(), "config.yaml")
	f := newTestFlags(srcdir, dst)

	testCases := []struct {
		description string
		config      string
		wantChanged bool
	}{
		{
			description: "create dangling symlink",
			config:      "missing-config",
			wantChanged: true,
		},
		{
			description: "dangling symlink already pointing at config is a no operation",
			config:      "missing-config",
			wantChanged: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			changed, err := updateSymlink(tc.config, f)
			require.NoError(t, err)
			require.Equal(t, tc.wantChanged, changed)
		})
	}

	link, err := os.Readlink(dst)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(srcdir, "missing-config"), link)
}
