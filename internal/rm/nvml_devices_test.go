/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeNumaNodeFile(t *testing.T, root, busID, content string) {
	t.Helper()
	dir := filepath.Join(root, "bus", "pci", "devices", busID)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "numa_node"), []byte(content), 0o644))
}

func TestReadNumaNodeFromSysfs(t *testing.T) {
	root := t.TempDir()

	t.Run("valid node", func(t *testing.T) {
		writeNumaNodeFile(t, root, "0000:03:00.0", "1\n")
		hasNuma, node, err := readNumaNodeFromSysfs(root, "0000:03:00.0")
		require.NoError(t, err)
		require.True(t, hasNuma)
		require.Equal(t, 1, node)
	})

	t.Run("missing file returns no numa", func(t *testing.T) {
		hasNuma, node, err := readNumaNodeFromSysfs(root, "0000:99:00.0")
		require.NoError(t, err)
		require.False(t, hasNuma)
		require.Equal(t, 0, node)
	})

	t.Run("negative one returns no numa", func(t *testing.T) {
		writeNumaNodeFile(t, root, "0000:04:00.0", "-1\n")
		hasNuma, node, err := readNumaNodeFromSysfs(root, "0000:04:00.0")
		require.NoError(t, err)
		require.False(t, hasNuma)
		require.Equal(t, 0, node)
	})

	t.Run("invalid content returns error", func(t *testing.T) {
		writeNumaNodeFile(t, root, "0000:05:00.0", "not-a-number\n")
		_, _, err := readNumaNodeFromSysfs(root, "0000:05:00.0")
		require.Error(t, err)
	})
}
