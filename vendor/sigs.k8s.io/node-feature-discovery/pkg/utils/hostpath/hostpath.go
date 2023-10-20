/*
Copyright 2020 The Kubernetes Authors.

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

package hostpath

import (
	"path/filepath"
)

var (
	pathPrefix = "/"
	// BootDir is where the /boot directory of the system to be inspected is located
	BootDir = HostDir(pathPrefix + "boot")
	// EtcDir is where the /etc directory of the system to be inspected is located
	EtcDir = HostDir(pathPrefix + "etc")
	// SysfsDir is where the /sys directory of the system to be inspected is located
	SysfsDir = HostDir(pathPrefix + "sys")
	// UsrDir is where the /usr directory of the system to be inspected is located
	UsrDir = HostDir(pathPrefix + "usr")
	// VarDir is where the /var directory of the system to be inspected is located
	VarDir = HostDir(pathPrefix + "var")
	// LibDir is where the /lib directory of the system to be inspected is located
	LibDir = HostDir(pathPrefix + "lib")
)

// HostDir is a helper for handling host system directories
type HostDir string

// Path returns a full path to a file under HostDir
func (d HostDir) Path(elem ...string) string {
	return filepath.Join(append([]string{string(d)}, elem...)...)
}
