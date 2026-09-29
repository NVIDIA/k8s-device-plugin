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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/apimachinery/pkg/runtime"

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


// Regression test for https://github.com/NVIDIA/k8s-device-plugin/issues/2066:
// the first Get() must not return the empty zero value before the node
// informer's initial LIST has been observed.  The gated LIST reactor keeps the
// cache unsynced until released, so a Get() that returns early is the bug.
func TestContinuouslySyncConfigChangesWaitsForCacheSync(t *testing.T) {
	const labelKey = "nvidia.com/mig.config"
	const labelValue = "all-disabled"

	f := &Flags{
		NodeLabel:       labelKey,
		NodeName:        "test-node",
		Oneshot:         false,
		ConfigFileSrcdir: "/dev/null", // unused by the informer path
	}

	nodes := &v1.NodeList{
		Items: []v1.Node{{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-node",
				Labels: map[string]string{labelKey: labelValue},
			},
		}},
	}

	client := fake.NewSimpleClientset()
	releaseList := make(chan struct{})
	client.PrependReactor("list", "nodes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		// Gate the initial LIST so the informer cache stays unsynced until
		// the test releases it.
		<-releaseList
		return true, nodes, nil
	})

	config := NewSyncableConfig(f)
	stop := continuouslySyncConfigChanges(client, config, f)
	defer close(stop)

	// Give the controller goroutine time to reach the gated LIST.
	time.Sleep(100 * time.Millisecond)

	got := make(chan string, 1)
	go func() {
		got <- config.Get()
	}()

	select {
	case v := <-got:
		t.Fatalf("Get() returned %q before the informer cache synced; "+
			"labeled nodes would be flipped to the default config", v)
	case <-time.After(500 * time.Millisecond):
		// Expected: Get() blocks until the cache syncs.
	}

	close(releaseList)

	select {
	case v := <-got:
		if v != labelValue {
			t.Fatalf("Get() returned %q, want %q", v, labelValue)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Get() did not return after the informer cache synced")
	}
}