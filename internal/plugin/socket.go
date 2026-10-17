/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package plugin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/version"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// unixSocketPathMax is the largest path that fits in sockaddr_un.sun_path,
// which is 108 bytes including the terminating NUL.
const unixSocketPathMax = 107

// minKubeletForUniqueSockets is the first kubelet that tracks a device plugin
// by socket path, so two plugins for one resource can be registered together.
const minKubeletForUniqueSockets = "v1.37.0"

// pluginSocketPath returns the socket for resource. unique appends a random
// 8-hex suffix so a second pod does not unlink the socket the first pod is
// still serving. An empty suffix keeps the historical nvidia-<resource>.sock
// name, including when that path is longer than a Unix socket allows.
func pluginSocketPath(resource spec.ResourceName, unique bool) (string, error) {
	suffix := ""
	if unique {
		var err error
		suffix, err = newSocketSuffix()
		if err != nil {
			return "", err
		}
	}
	return pluginSocketPathWithSuffix(resource, suffix), nil
}

// pluginSocketPathWithSuffix builds the socket path. suffix is 8 hex
// characters, or empty for the stable name.
func pluginSocketPathWithSuffix(resource spec.ResourceName, suffix string) string {
	_, name := resource.Split()
	if suffix == "" {
		return joinSocket("nvidia-" + name)
	}

	suffixPart := "-" + suffix
	plain := joinSocket("nvidia-" + name + suffixPart)
	if len(plain) <= unixSocketPathMax {
		return plain
	}

	// Two long resource names can truncate to the same prefix. Mix in a hash
	// of the full resource name so they stay distinct. The random suffix
	// already separates two starts of the same resource.
	sum := sha256.Sum256([]byte(resource))
	hash := hex.EncodeToString(sum[:4])
	room := unixSocketPathMax - len(joinSocket("nvidia-"+suffixPart))
	if room <= 0 {
		return plain
	}
	if room <= len(hash) {
		return joinSocket("nvidia-" + hash[:room] + suffixPart)
	}
	keep := room - len(hash)
	if keep > len(name) {
		keep = len(name)
	}
	return joinSocket("nvidia-" + name[:keep] + hash + suffixPart)
}

func joinSocket(pluginName string) string {
	return filepath.Join(pluginapi.DevicePluginPath, pluginName) + ".sock"
}

func newSocketSuffix() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// KubeletSupportsUniqueSockets reports whether kubeletVersion is new enough
// to keep two registrations for one resource, keyed by socket path. A version
// that cannot be parsed does not.
func KubeletSupportsUniqueSockets(kubeletVersion string) bool {
	parsed, err := version.ParseGeneric(kubeletVersion)
	if err != nil {
		return false
	}
	return parsed.AtLeast(version.MustParseGeneric(minKubeletForUniqueSockets))
}
