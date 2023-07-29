/**
# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
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

package lookup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

type symlinkChain struct {
	file
}

type symlink struct {
	file
}

// NewSymlinkChainLocator creats a locator that can be used for locating files through symlinks.
// A logger can also be specified.
func NewSymlinkChainLocator(logger *logrus.Logger, root string) Locator {
	f := newFileLocator(WithLogger(logger), WithRoot(root))
	l := symlinkChain{
		file: *f,
	}

	return &l
}

// NewSymlinkLocator creats a locator that can be used for locating files through symlinks.
// A logger can also be specified.
func NewSymlinkLocator(logger *logrus.Logger, root string) Locator {
	f := newFileLocator(WithLogger(logger), WithRoot(root))
	l := symlink{
		file: *f,
	}

	return &l
}

// Locate finds the specified pattern at the specified root.
// If the file is a symlink, the link is followed and all candidates to the final target are returned.
func (p symlinkChain) Locate(pattern string) ([]string, error) {
	candidates, err := p.file.Locate(pattern)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return candidates, nil
	}

	found := make(map[string]bool)
	for len(candidates) > 0 {
		candidate := candidates[0]
		candidates = candidates[:len(candidates)-1]
		if found[candidate] {
			continue
		}
		found[candidate] = true

		info, err := os.Lstat(candidate)
		if err != nil {
			return nil, fmt.Errorf("failed to get file info: %v", info)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(candidate)
		if err != nil {
			return nil, fmt.Errorf("error checking symlink: %v", err)
		}

		if !filepath.IsAbs(target) {
			target, err = filepath.Abs(filepath.Join(filepath.Dir(candidate), target))
			if err != nil {
				return nil, fmt.Errorf("failed to construct absolute path: %v", err)
			}
		}

		p.logger.Debugf("Resolved link: '%v' => '%v'", candidate, target)
		if !found[target] {
			candidates = append(candidates, target)
		}
	}

	var filenames []string
	for f := range found {
		filenames = append(filenames, f)
	}
	return filenames, nil
}

// Locate finds the specified pattern at the specified root.
// If the file is a symlink, the link is resolved and the target returned.
func (p symlink) Locate(pattern string) ([]string, error) {
	candidates, err := p.file.Locate(pattern)
	if err != nil {
		return nil, err
	}
	if len(candidates) != 1 {
		return nil, fmt.Errorf("failed to uniquely resolve symlink %v: %v", pattern, candidates)
	}

	target, err := filepath.EvalSymlinks(candidates[0])
	if err != nil {
		return nil, fmt.Errorf("failed to resolve link: %v", err)
	}

	return []string{target}, err
}
