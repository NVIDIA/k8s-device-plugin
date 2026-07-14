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

func TestNewConfigDefaultsMPSFailRequestsGreaterThanOne(t *testing.T) {
	config, err := newConfigForTest(t, `
version: v1
sharing:
  mps:
    resources:
      - name: nvidia.com/gpu
        replicas: 2
`)
	require.NoError(t, err)
	require.NotNil(t, config.Sharing.MPS)
	require.NotNil(t, config.Sharing.MPS.FailRequestsGreaterThanOne)
	require.True(t, *config.Sharing.MPS.FailRequestsGreaterThanOne)
}

func TestNewConfigHonorsExplicitMPSFailRequestsGreaterThanOneFalse(t *testing.T) {
	config, err := newConfigForTest(t, `
version: v1
sharing:
  mps:
    failRequestsGreaterThanOne: false
    resources:
      - name: nvidia.com/gpu
        replicas: 2
`)
	require.NoError(t, err)
	require.NotNil(t, config.Sharing.MPS)
	require.NotNil(t, config.Sharing.MPS.FailRequestsGreaterThanOne)
	require.False(t, *config.Sharing.MPS.FailRequestsGreaterThanOne)
}

func TestDisableResourceNamingInConfig_PreservesPerDeviceTimeSlicingSelection(t *testing.T) {
	cfg := &Config{
		Sharing: Sharing{
			TimeSlicing: ReplicatedResources{
				Resources: []ReplicatedResource{
					{
						Name:     ResourceName("nvidia.com/gpu"),
						Rename:   ResourceName("nvidia.com/gpu-light"),
						Devices:  ReplicatedDevices{List: []ReplicatedDeviceRef{"0"}},
						Replicas: 2,
					},
					{
						Name:     ResourceName("nvidia.com/gpu"),
						Rename:   ResourceName("nvidia.com/gpu-heavy"),
						Devices:  ReplicatedDevices{List: []ReplicatedDeviceRef{"1"}},
						Replicas: 4,
					},
				},
			},
		},
	}

	DisableResourceNamingInConfig(cfg)

	require.Len(t, cfg.Sharing.TimeSlicing.Resources, 2)

	first := cfg.Sharing.TimeSlicing.Resources[0]
	require.Equal(t, ResourceName("nvidia.com/gpu-light"), first.Rename,
		"expected per-entry Rename to be preserved")
	require.False(t, first.Devices.All,
		"expected per-entry Devices.List to remain (Devices.All should NOT be forced)")
	require.Equal(t, []ReplicatedDeviceRef{"0"}, first.Devices.List,
		"expected Devices.List to be preserved")

	second := cfg.Sharing.TimeSlicing.Resources[1]
	require.Equal(t, ResourceName("nvidia.com/gpu-heavy"), second.Rename)
	require.False(t, second.Devices.All)
	require.Equal(t, []ReplicatedDeviceRef{"1"}, second.Devices.List)
}

func TestDisableResourceNamingInConfig_PreservesPerDeviceMPSSelection(t *testing.T) {
	cfg := &Config{
		Sharing: Sharing{
			MPS: &ReplicatedResources{
				Resources: []ReplicatedResource{
					{
						Name:     ResourceName("nvidia.com/gpu"),
						Rename:   ResourceName("nvidia.com/gpu-light"),
						Devices:  ReplicatedDevices{List: []ReplicatedDeviceRef{"0"}},
						Replicas: 2,
					},
					{
						Name:     ResourceName("nvidia.com/gpu"),
						Rename:   ResourceName("nvidia.com/gpu-heavy"),
						Devices:  ReplicatedDevices{List: []ReplicatedDeviceRef{"1"}},
						Replicas: 4,
					},
				},
			},
		},
	}

	DisableResourceNamingInConfig(cfg)

	require.NotNil(t, cfg.Sharing.MPS)
	require.Len(t, cfg.Sharing.MPS.Resources, 2)

	first := cfg.Sharing.MPS.Resources[0]
	require.Equal(t, ResourceName("nvidia.com/gpu-light"), first.Rename)
	require.False(t, first.Devices.All)
	require.Equal(t, []ReplicatedDeviceRef{"0"}, first.Devices.List)

	second := cfg.Sharing.MPS.Resources[1]
	require.Equal(t, ResourceName("nvidia.com/gpu-heavy"), second.Rename)
	require.False(t, second.Devices.All)
	require.Equal(t, []ReplicatedDeviceRef{"1"}, second.Devices.List)
}

func newConfigForTest(t *testing.T, contents string) (*Config, error) {
	t.Helper()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configFile, []byte(contents), 0o600)
	require.NoError(t, err)

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.String("config-file", "", "")
	err = set.Set("config-file", configFile)
	require.NoError(t, err)

	ctx := cli.NewContext(cli.NewApp(), set, nil)
	return NewConfig(ctx, nil)
}
