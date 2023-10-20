/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

// FsWatcher is a wrapper helper for watching files
type FsWatcher struct {
	*fsnotify.Watcher

	Events    chan struct{}
	ratelimit time.Duration
	names     []string
	paths     map[string]struct{}
}

// CreateFsWatcher creates a new FsWatcher
func CreateFsWatcher(ratelimit time.Duration, names ...string) (*FsWatcher, error) {
	w := &FsWatcher{
		Events:    make(chan struct{}),
		names:     names,
		ratelimit: ratelimit,
	}

	if err := w.reset(names...); err != nil {
		return nil, err
	}

	go w.watch()

	return w, nil
}

// reset resets the file watches
func (w *FsWatcher) reset(names ...string) error {
	if err := w.initWatcher(); err != nil {
		return err
	}
	if err := w.add(names...); err != nil {
		return err
	}

	return nil
}

func (w *FsWatcher) initWatcher() error {
	if w.Watcher != nil {
		if err := w.Watcher.Close(); err != nil {
			return fmt.Errorf("failed to close fsnotify watcher: %v", err)
		}
	}
	w.paths = make(map[string]struct{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.Watcher = nil
		return fmt.Errorf("failed to create fsnotify watcher: %v", err)
	}
	w.Watcher = watcher

	return nil
}

func (w *FsWatcher) add(names ...string) error {
	for _, name := range names {
		if name == "" {
			continue
		}

		added := false
		// Add watches for all directory components so that we catch e.g. renames
		// upper in the tree
		for p := name; ; p = filepath.Dir(p) {
			if _, ok := w.paths[p]; !ok {
				if err := w.Add(p); err != nil {
					klog.V(1).Infof("failed to add fsnotify watch for %q: %v", p, err)
				} else {
					klog.V(1).Infof("added fsnotify watch %q", p)
					added = true
				}

				w.paths[p] = struct{}{}
			} else {
				added = true
			}
			if filepath.Dir(p) == p {
				break
			}
		}
		if !added {
			// Want to be sure that we watch something
			return fmt.Errorf("failed to add any watch")
		}
	}

	return nil
}

func (w *FsWatcher) watch() {
	var ratelimiter <-chan time.Time
	for {
		select {
		case e, ok := <-w.Watcher.Events:
			// Watcher has been closed
			if !ok {
				klog.Infof("watcher closed")
				return
			}

			// If any of our paths change
			name := filepath.Clean(e.Name)
			if _, ok := w.paths[filepath.Clean(name)]; ok {
				klog.V(2).Infof("fsnotify %s event in %q detected", e, name)

				// Rate limiter. In certain filesystem operations we get
				// numerous events in quick succession
				ratelimiter = time.After(w.ratelimit)
			}

		case e, ok := <-w.Watcher.Errors:
			// Watcher has been closed
			if !ok {
				klog.Infof("watcher closed")
				return
			}
			klog.Warningf("fswatcher error event detected: %v", e)

		case <-ratelimiter:
			// Blindly remove existing watch and add a new one
			if err := w.reset(w.names...); err != nil {
				klog.Errorf("%v, re-trying in 60 seconds...", err)
				ratelimiter = time.After(60 * time.Second)
			}

			w.Events <- struct{}{}
		}
	}
}
