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

package mps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadyFilePath(t *testing.T) {
	require.Equal(t, "/mps/.ready", ContainerRoot.ReadyFilePath())
	require.Equal(t, "/custom/root/.ready", Root("/custom/root").ReadyFilePath())
}

func TestDaemonReady(t *testing.T) {
	root := t.TempDir()
	d := &Daemon{root: Root(root)}

	ready, err := d.Ready()
	require.NoError(t, err)
	require.False(t, ready, "not ready before the .ready file exists")

	require.NoError(t, os.WriteFile(filepath.Join(root, ".ready"), nil, 0o644))
	ready, err = d.Ready()
	require.NoError(t, err)
	require.True(t, ready, "ready once the .ready file exists")

	require.NoError(t, os.Remove(filepath.Join(root, ".ready")))
	ready, err = d.Ready()
	require.NoError(t, err)
	require.False(t, ready, "not ready after the .ready file is removed")
}

// TestDaemonReadyStatError verifies a stat error other than not-exist is
// surfaced rather than reported as "not ready". The root is a regular file, so
// stat-ing a path beneath it fails with ENOTDIR.
func TestDaemonReadyStatError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(f, nil, 0o644))
	d := &Daemon{root: Root(f)}

	ready, err := d.Ready()
	require.Error(t, err)
	require.False(t, ready)
}
