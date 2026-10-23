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
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

func TestPluginSocketPathStableName(t *testing.T) {
	got := pluginSocketPathWithSuffix("nvidia.com/gpu", "")
	require.Equal(t, filepath.Join(pluginapi.DevicePluginPath, "nvidia-gpu")+".sock", got)
	require.Equal(t, "nvidia-gpu.sock", path.Base(got))
}

func TestPluginSocketPathUniqueSuffixesDiffer(t *testing.T) {
	first, err := pluginSocketPath("nvidia.com/gpu", true)
	require.NoError(t, err)
	second, err := pluginSocketPath("nvidia.com/gpu", false)
	require.NoError(t, err)
	third, err := pluginSocketPath("nvidia.com/gpu", true)
	require.NoError(t, err)

	require.Equal(t, filepath.Join(pluginapi.DevicePluginPath, "nvidia-gpu")+".sock", second)
	require.NotEqual(t, first, third)
	require.True(t, strings.HasSuffix(first, ".sock"))
	require.True(t, strings.HasSuffix(third, ".sock"))
	require.Equal(t, path.Base(first), filepath.Base(first))
	require.NotContains(t, path.Base(first), "/")
	require.LessOrEqual(t, len(first), unixSocketPathMax)
	require.LessOrEqual(t, len(third), unixSocketPathMax)
}

func TestPluginSocketPathTruncatesLongSharedName(t *testing.T) {
	// A 52-character resource plus renameByDefault's ".shared" is a 103-byte
	// path. The 8-hex suffix adds 9 bytes and would pass the Unix limit.
	name := strings.Repeat("a", 52) + ".shared"
	// Same 46-character prefix as name, so truncation without a resource hash
	// would build one basename for both.
	other := strings.Repeat("a", 51) + "b.shared"
	resource := spec.ResourceName("example.com/" + name)
	otherResource := spec.ResourceName("example.com/" + other)

	stable := pluginSocketPathWithSuffix(resource, "")
	require.Equal(t, 103, len(stable))
	_, namePart := resource.Split()
	untruncated := filepath.Join(pluginapi.DevicePluginPath, "nvidia-"+namePart+"-abcdef01") + ".sock"
	require.Equal(t, 112, len(untruncated))

	const suffix = "abcdef01"
	got := pluginSocketPathWithSuffix(resource, suffix)
	otherGot := pluginSocketPathWithSuffix(otherResource, suffix)
	require.LessOrEqual(t, len(got), unixSocketPathMax)
	require.LessOrEqual(t, len(otherGot), unixSocketPathMax)
	require.NotEqual(t, got, otherGot)
	require.True(t, strings.HasSuffix(path.Base(got), "-"+suffix+".sock"))
	require.Equal(t, path.Base(got), filepath.Base(got))
}

func TestPluginSocketPathStableNameIsNotTruncated(t *testing.T) {
	resource := spec.ResourceName("example.com/" + strings.Repeat("n", 64))
	got := pluginSocketPathWithSuffix(resource, "")
	require.Greater(t, len(got), unixSocketPathMax)
	require.Equal(t, filepath.Join(pluginapi.DevicePluginPath, "nvidia-"+strings.Repeat("n", 64))+".sock", got)
}

func TestKubeletSupportsUniqueSockets(t *testing.T) {
	testCases := []struct {
		version string
		want    bool
	}{
		{version: "v1.36.9", want: false},
		{version: "v1.36.9-eks-1", want: false},
		{version: "v1.37.0", want: true},
		{version: "v1.37.0-eks-123", want: true},
		{version: "v1.40.1", want: true},
		{version: "v1.37", want: true},
		{version: "", want: false},
		{version: "not-a-version", want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			require.Equal(t, tc.want, KubeletSupportsUniqueSockets(tc.version))
		})
	}
}
