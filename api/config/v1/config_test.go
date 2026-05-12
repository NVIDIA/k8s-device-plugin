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
