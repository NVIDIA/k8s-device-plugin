/**
# Copyright 2023 NVIDIA CORPORATION
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
	"fmt"
	"path/filepath"
)

type root string

// getDriverLibraryPath returns path to `libnvidia-ml.so.1` in the driver root.
// The folder for this file is also expected to be the location of other driver files.
func (r root) getDriverLibraryPath() (string, error) {
	librarySearchPaths := []string{
		"/usr/lib64",
		"/usr/lib/x86_64-linux-gnu",
		"/usr/lib/aarch64-linux-gnu",
		"/lib64",
		"/lib/x86_64-linux-gnu",
		"/lib/aarch64-linux-gnu",
	}

	libraryPath, err := r.findFile("libnvidia-ml.so.1", librarySearchPaths...)
	if err != nil {
		return "", err
	}

	return libraryPath, nil
}

// findFile searches the root for a specified file.
// A number of folders can be specified to search in addition to the root itself.
// If the file represents a symlink, this is resolved and the final path is returned.
func (r root) findFile(name string, searchIn ...string) (string, error) {

	for _, d := range append([]string{"/"}, searchIn...) {
		l := filepath.Join(string(r), d, name)
		candidate, err := resolveLink(l)
		if err != nil {
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf("error locating %q", name)
}

// resolveLink finds the target of a symlink or the file itself in the
// case of a regular file.
// This is equivalent to running `readlink -f ${l}`.
func resolveLink(l string) (string, error) {
	resolved, err := filepath.EvalSymlinks(l)
	if err != nil {
		return "", fmt.Errorf("error resolving link '%v': %v", l, err)
	}
	return resolved, nil
}
