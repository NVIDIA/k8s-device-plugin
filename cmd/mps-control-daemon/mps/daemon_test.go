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

package mps_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/cmd/mps-control-daemon/mps"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
)

// newTestDaemon installs fake MPS binaries in a directory that is only
// reachable through the parent's PATH and returns a daemon with no devices
// (so that no nvidia-smi calls are made) along with the file that the fake
// server writes its observations to.
//
// The fake nvidia-cuda-mps-control resolves nvidia-cuda-mps-server by name,
// mirroring the real control daemon. This only succeeds when the child
// inherits the parent's PATH, since a shell's fallback PATH never includes
// a test temp directory.
func newTestDaemon(t *testing.T) (*mps.Daemon, string) {
	t.Helper()

	binDir := t.TempDir()
	record := filepath.Join(t.TempDir(), "record")

	control := "#!/bin/sh\nexec nvidia-cuda-mps-server \"$@\"\n"
	server := `#!/bin/sh
read -r input || true
{
	echo "args=$*"
	echo "input=$input"
	echo "CUDA_MPS_PIPE_DIRECTORY=$CUDA_MPS_PIPE_DIRECTORY"
	echo "CUDA_MPS_LOG_DIRECTORY=$CUDA_MPS_LOG_DIRECTORY"
} >> '` + record + `'
`
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "nvidia-cuda-mps-control"), []byte(control), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "nvidia-cuda-mps-server"), []byte(server), 0755))

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Stale values inherited from the parent must not win over the
	// per-resource directories of the daemon.
	t.Setenv("CUDA_MPS_PIPE_DIRECTORY", "/stale/pipe")
	t.Setenv("CUDA_MPS_LOG_DIRECTORY", "/stale/log")

	resourceManager := &rm.ResourceManagerMock{
		ResourceFunc: func() spec.ResourceName { return "nvidia.com/gpu" },
		DevicesFunc:  func() rm.Devices { return nil },
	}
	return mps.NewDaemon(resourceManager, mps.Root(t.TempDir())), record
}

func readRecord(t *testing.T, record string) string {
	t.Helper()
	contents, err := os.ReadFile(record)
	require.NoError(t, err)
	return string(contents)
}

func TestDaemonStartInheritsEnvironment(t *testing.T) {
	d, record := newTestDaemon(t)

	// Pre-create the log so that the log tailer started by Start has a file
	// to follow until Stop terminates it.
	require.NoError(t, os.MkdirAll(d.LogDir(), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(d.LogDir(), "control.log"), nil, 0600))

	require.NoError(t, d.Start())
	t.Cleanup(func() { require.NoError(t, d.Stop()) })

	require.Equal(t, strings.Join([]string{
		"args=-d",
		"input=",
		"CUDA_MPS_PIPE_DIRECTORY=" + d.PipeDir(),
		"CUDA_MPS_LOG_DIRECTORY=" + d.LogDir(),
	}, "\n")+"\n", readRecord(t, record))
}

func TestDaemonEchoPipeToControlInheritsEnvironment(t *testing.T) {
	d, record := newTestDaemon(t)

	_, err := d.EchoPipeToControl("get_default_active_thread_percentage")
	require.NoError(t, err)

	require.Equal(t, strings.Join([]string{
		"args=",
		"input=get_default_active_thread_percentage",
		"CUDA_MPS_PIPE_DIRECTORY=" + d.PipeDir(),
		"CUDA_MPS_LOG_DIRECTORY=" + d.LogDir(),
	}, "\n")+"\n", readRecord(t, record))
}
