/**
# Copyright 2026 NVIDIA CORPORATION
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

	// No .ready file yet: the daemon must not report ready. This is the window
	// in which AssertHealthy can already succeed (control pipe responsive)
	// while per-device memory/thread configuration is not yet applied.
	require.False(t, d.Ready(), "daemon must not be ready before the .ready file exists")

	// Once the MPS control daemon has finished initialization it creates the
	// .ready file; the daemon must then report ready.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".ready"), nil, 0o644))
	require.True(t, d.Ready(), "daemon must be ready once the .ready file exists")

	// Removing the file (e.g. on daemon stop) flips readiness back to false.
	require.NoError(t, os.Remove(filepath.Join(root, ".ready")))
	require.False(t, d.Ready(), "daemon must not be ready after the .ready file is removed")
}
