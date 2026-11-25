/**
# Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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

package logger_test

import (
	"bytes"
	"testing"

	nvinfo "github.com/NVIDIA/go-nvlib/pkg/nvlib/info"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/NVIDIA/k8s-device-plugin/internal/logger"
)

func TestNvInfoDebugLogging(t *testing.T) {
	out, level := logrus.StandardLogger().Out, logrus.GetLevel()
	t.Cleanup(func() {
		logrus.SetOutput(out)
		logrus.SetLevel(level)
	})

	output := &bytes.Buffer{}
	logrus.SetOutput(output)

	newInfolib := func() nvinfo.Interface {
		return nvinfo.New(
			nvinfo.WithLogger(logrus.StandardLogger()),
			nvinfo.WithPropertyExtractor(&nvinfo.PropertyExtractorMock{
				HasDXCoreFunc:          func() (bool, string) { return false, "dxcore not found" },
				HasTegraFilesFunc:      func() (bool, string) { return false, "tegra files not found" },
				HasNvmlFunc:            func() (bool, string) { return true, "nvml found" },
				HasAnIntegratedGPUFunc: func() (bool, string) { return false, "integrated GPU not found" },
			}),
		)
	}

	logger.SetLevel(false)
	require.Equal(t, nvinfo.PlatformNVML, newInfolib().ResolvePlatform())
	require.Empty(t, output.String(), "platform detection details should not be logged when debug is disabled")

	logger.SetLevel(true)
	require.Equal(t, nvinfo.PlatformNVML, newInfolib().ResolvePlatform())
	require.Contains(t, output.String(), "Is NVML-based system? true: nvml found")
	require.Contains(t, output.String(), "Is Tegra-based system? false: tegra files not found")
}
