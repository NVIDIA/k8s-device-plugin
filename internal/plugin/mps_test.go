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

package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
)

// TestCheckDaemonReadyRequiresReadyFile is the regression for this PR's race:
// even when the control pipe would be healthy, readiness must be withheld until
// the .ready file exists. With no .ready file, checkDaemonReady returns
// not-ready before ever consulting the pipe (AssertHealthy).
func TestCheckDaemonReadyRequiresReadyFile(t *testing.T) {
	root := t.TempDir() // no .ready file
	m := &mpsOptions{
		enabled: true,
		daemon:  mps.NewDaemon(nil, mps.Root(root)),
	}

	ready, err := m.checkDaemonReady(context.Background())
	require.NoError(t, err, "a missing .ready file is not-ready-yet, not an error")
	require.False(t, ready)
}

// TestCheckDaemonReadyStatError verifies a genuine stat error on the .ready file
// is surfaced (so the poll aborts), not swallowed as not-ready. The root is a
// regular file, so stat-ing a path beneath it fails with ENOTDIR.
func TestCheckDaemonReadyStatError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(f, nil, 0o644))
	m := &mpsOptions{enabled: true, daemon: mps.NewDaemon(nil, mps.Root(f))}

	ready, err := m.checkDaemonReady(context.Background())
	require.Error(t, err)
	require.False(t, ready)
}
