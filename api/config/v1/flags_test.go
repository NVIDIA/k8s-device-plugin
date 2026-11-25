/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
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
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cli "github.com/urfave/cli/v2"
)

func TestUnmarshalFlags(t *testing.T) {
	testCases := []struct {
		input  string
		output Flags
		err    bool
	}{
		{
			input: ``,
			err:   true,
		},
		{
			input:  `{}`,
			output: Flags{},
		},
		{
			input: `{
				"gfd": {}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{},
				},
			},
		},
		{
			input: `{
				"gfd": {
					"sleepInterval": 0
				}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(0)),
					},
				},
			},
		},
		{
			input: `{
				"gfd": {
					"sleepInterval": "0s"
				}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(0)),
					},
				},
			},
		},
		{
			input: `{
				"gfd": {
					"sleepInterval": 5
				}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(5)),
					},
				},
			},
		},
		{
			input: `{
				"gfd": {
					"sleepInterval": "5s"
				}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(5 * time.Second)),
					},
				},
			},
		},
		{
			input: `{
				"gfd": {
					"sleepInterval": "infinite"
				}
			}`,
			output: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(math.MaxInt64)),
					},
				},
			},
		},
		{
			input: `{
				"plugin": {
					"deviceListStrategy": "envvar"
				}
			}`,
			output: Flags{
				CommandLineFlags{
					Plugin: &PluginCommandLineFlags{
						DeviceListStrategy: &deviceListStrategyFlag{"envvar"},
					},
				},
			},
		},
		{
			input: `{
				"plugin": {
					"deviceListStrategy": ["envvar", "cdi-annotations"]
				}
			}`,
			output: Flags{
				CommandLineFlags{
					Plugin: &PluginCommandLineFlags{
						DeviceListStrategy: &deviceListStrategyFlag{"envvar", "cdi-annotations"},
					},
				},
			},
		},
		{
			input: `{
				"debug": true
			}`,
			output: Flags{
				CommandLineFlags{
					Debug: new(true),
				},
			},
		},
		{
			input: `{
				"debug": false
			}`,
			output: Flags{
				CommandLineFlags{
					Debug: new(false),
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var output Flags
			err := json.Unmarshal([]byte(tc.input), &output)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestMarshalFlags(t *testing.T) {
	testCases := []struct {
		input  Flags
		output string
		err    bool
	}{
		{
			input: Flags{},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null
			}`,
		},
		{
			input: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(0)),
					},
				},
			},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null,
				"gfd": {
					"oneshot": null,
					"noTimestamp": null,
					"outputFile": null,
					"sleepInterval": "0s",
					"machineTypeFile": null
				}
			}`,
		},
		{
			input: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(5)),
					},
				},
			},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null,
				"gfd": {
					"oneshot": null,
					"noTimestamp": null,
					"outputFile": null,
					"sleepInterval": "5ns",
					"machineTypeFile": null
				}
			}`,
		},
		{
			input: Flags{
				CommandLineFlags{
					GFD: &GFDCommandLineFlags{
						SleepInterval: new(Duration(math.MaxInt64)),
					},
				},
			},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null,
				"gfd": {
					"oneshot": null,
					"noTimestamp": null,
					"outputFile": null,
					"sleepInterval": "infinite",
					"machineTypeFile": null
				}
			}`,
		},
		{
			input: Flags{
				CommandLineFlags{
					Debug: new(true),
				},
			},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null,
				"debug": true
			}`,
		},
		{
			input: Flags{
				CommandLineFlags{
					Debug: new(false),
				},
			},
			output: `{
				"migStrategy": null,
				"failOnInitError": null,
				"gdrcopyEnabled": null,
				"gdsEnabled": null,
				"mofedEnabled": null,
				"useNodeFeatureAPI": null,
				"deviceDiscoveryStrategy": null,
				"debug": false
			}`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output, err := json.Marshal(tc.input)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.output, string(output))
		})
	}
}

// TestDebugFromConfigFileAndCLI verifies how the debug flag is resolved from
// the config file and the command line. The environment variable wiring is
// covered separately to keep these cases independent of the environment.
func TestDebugFromConfigFileAndCLI(t *testing.T) {
	testCases := []struct {
		description string
		config      string
		cliValue    *string
		expected    bool
	}{
		{
			description: "debug is disabled by default",
		},
		{
			description: "config file enables debug",
			config:      `{"version":"v1","flags":{"debug":true}}`,
			expected:    true,
		},
		{
			description: "config file disables debug explicitly",
			config:      `{"version":"v1","flags":{"debug":false}}`,
		},
		{
			description: "command line disables debug set in the config file",
			config:      `{"version":"v1","flags":{"debug":true}}`,
			cliValue:    new("false"),
		},
		{
			description: "command line enables debug",
			cliValue:    new("true"),
			expected:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			flags := []cli.Flag{
				&cli.BoolFlag{
					Name: "debug",
				},
			}

			set := flag.NewFlagSet("test", flag.ContinueOnError)
			set.String("config-file", "", "")
			if tc.config != "" {
				configFile := filepath.Join(t.TempDir(), "config.json")
				require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))
				require.NoError(t, set.Set("config-file", configFile))
			}
			for _, f := range flags {
				require.NoError(t, f.Apply(set))
			}
			if tc.cliValue != nil {
				require.NoError(t, set.Set("debug", *tc.cliValue))
			}

			app := cli.NewApp()
			app.Flags = flags
			config, err := NewConfig(cli.NewContext(app, set, nil), flags)
			require.NoError(t, err)
			require.NotNil(t, config.Flags.Debug)
			require.Equal(t, tc.expected, *config.Flags.Debug)
		})
	}
}

// TestDebugFromEnvironment verifies that the DEBUG environment variable takes
// precedence over the value set in the config file.
func TestDebugFromEnvironment(t *testing.T) {
	testCases := []struct {
		description string
		env         string
		config      string
		expected    bool
	}{
		{
			description: "environment enables debug set to false in the config file",
			env:         "true",
			config:      `{"version":"v1","flags":{"debug":false}}`,
			expected:    true,
		},
		{
			description: "environment disables debug set to true in the config file",
			env:         "false",
			config:      `{"version":"v1","flags":{"debug":true}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Setenv("DEBUG", tc.env)

			flags := []cli.Flag{
				&cli.BoolFlag{
					Name:    "debug",
					EnvVars: []string{"DEBUG"},
				},
			}

			configFile := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(configFile, []byte(tc.config), 0o600))

			set := flag.NewFlagSet("test", flag.ContinueOnError)
			set.String("config-file", "", "")
			require.NoError(t, set.Set("config-file", configFile))
			for _, f := range flags {
				require.NoError(t, f.Apply(set))
			}

			app := cli.NewApp()
			app.Flags = flags
			config, err := NewConfig(cli.NewContext(app, set, nil), flags)
			require.NoError(t, err)
			require.NotNil(t, config.Flags.Debug)
			require.Equal(t, tc.expected, *config.Flags.Debug)
		})
	}
}
