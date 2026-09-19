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

package v1

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v2"
)

func TestNewDeviceListStrategies(t *testing.T) {
	testCases := []struct {
		description      string
		strategies       []string
		expectedError    bool
		expectedIncludes []string
		expectedAnyCDI   bool
		expectedAllCDI   bool
	}{
		{
			description:      "envvar",
			strategies:       []string{DeviceListStrategyEnvVar},
			expectedIncludes: []string{DeviceListStrategyEnvVar},
		},
		{
			description:      "volume mounts and cdi annotations",
			strategies:       []string{DeviceListStrategyVolumeMounts, DeviceListStrategyCDIAnnotations},
			expectedIncludes: []string{DeviceListStrategyVolumeMounts, DeviceListStrategyCDIAnnotations},
			expectedAnyCDI:   true,
		},
		{
			description:      "cdi only",
			strategies:       []string{DeviceListStrategyCDIAnnotations, DeviceListStrategyCDICRI},
			expectedIncludes: []string{DeviceListStrategyCDIAnnotations, DeviceListStrategyCDICRI},
			expectedAnyCDI:   true,
			expectedAllCDI:   true,
		},
		{
			description:   "unknown strategy",
			strategies:    []string{"not-a-strategy"},
			expectedError: true,
		},
		{
			description:   "empty list",
			strategies:    []string{},
			expectedError: true,
		},
		{
			description:   "nil list",
			strategies:    nil,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			strategies, err := NewDeviceListStrategies(tc.strategies)
			if tc.expectedError {
				require.Error(t, err)
				require.Nil(t, strategies)
				return
			}
			require.NoError(t, err)
			for _, s := range tc.expectedIncludes {
				require.Truef(t, strategies.Includes(s), "expected %v to be included", s)
			}
			require.Equal(t, tc.expectedAnyCDI, strategies.AnyCDIEnabled())
			require.Equal(t, tc.expectedAllCDI, strategies.AllCDIEnabled())
		})
	}
}

func TestAllCDIEnabledRequiresACDIStrategy(t *testing.T) {
	require.False(t, DeviceListStrategies{}.AllCDIEnabled())
	require.False(t, DeviceListStrategies{
		DeviceListStrategyEnvVar:         false,
		DeviceListStrategyVolumeMounts:   false,
		DeviceListStrategyCDIAnnotations: false,
		DeviceListStrategyCDICRI:         false,
	}.AllCDIEnabled())
}

func TestDeviceListStrategyFromConfigFile(t *testing.T) {
	testCases := []struct {
		description   string
		config        string
		expectedError bool
		expected      []string
	}{
		{
			description: "no plugin section",
			config:      `{"version":"v1"}`,
			expected:    []string{DeviceListStrategyEnvVar},
		},
		{
			description: "omitted strategy",
			config:      `{"version":"v1","flags":{"plugin":{}}}`,
			expected:    []string{DeviceListStrategyEnvVar},
		},
		{
			description:   "empty strategy",
			config:        `{"version":"v1","flags":{"plugin":{"deviceListStrategy":[]}}}`,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			configFile := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))

			flags := []cli.Flag{
				&cli.StringSliceFlag{
					Name:  "device-list-strategy",
					Value: cli.NewStringSlice(DeviceListStrategyEnvVar),
				},
			}
			set := flag.NewFlagSet("test", flag.ContinueOnError)
			set.String("config-file", "", "")
			require.NoError(t, set.Set("config-file", configFile))
			for _, f := range flags {
				require.NoError(t, f.Apply(set))
			}

			config, err := NewConfig(cli.NewContext(cli.NewApp(), set, nil), flags)
			require.NoError(t, err)
			require.NotNil(t, config.Flags.Plugin)
			require.NotNil(t, config.Flags.Plugin.DeviceListStrategy)

			strategies, err := NewDeviceListStrategies(*config.Flags.Plugin.DeviceListStrategy)
			if tc.expectedError {
				require.Empty(t, []string(*config.Flags.Plugin.DeviceListStrategy))
				require.Error(t, err)
				require.Nil(t, strategies)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, []string(*config.Flags.Plugin.DeviceListStrategy))
		})
	}
}
