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

package lm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestMachineTypeLabeler(t *testing.T) {
	testCases := []struct {
		productName string
		expected    string
	}{
		{
			productName: "PowerEdge XE9680",
			expected:    "PowerEdge-XE9680",
		},
		{
			productName: "ThinkSystem SR650 -[7X06CTO1WW]-",
			expected:    "ThinkSystem-SR650--7X06CTO1WW",
		},
		{
			productName: "To Be Filled By O.E.M.",
			expected:    "To-Be-Filled-By-O.E.M",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.productName, func(t *testing.T) {
			machineTypeFile := filepath.Join(t.TempDir(), "product_name")
			require.NoError(t, os.WriteFile(machineTypeFile, []byte(tc.productName+"\n"), 0600))

			l, err := newMachineTypeLabeler(machineTypeFile)
			require.NoError(t, err)
			labels, err := l.Labels()
			require.NoError(t, err)

			value := labels["nvidia.com/gpu.machine"]
			require.Empty(t, validation.IsValidLabelValue(value))
			require.Equal(t, tc.expected, value)
		})
	}
}
