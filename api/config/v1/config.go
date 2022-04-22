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
	"fmt"
	"io"
	"io/ioutil"
	"os"

	cli "github.com/urfave/cli/v2"
	altsrc "github.com/urfave/cli/v2/altsrc"

	"sigs.k8s.io/yaml"
)

// Version indicates the version of the 'Config' struct used to hold configuration information.
const Version = "v1"

// Config is a versioned struct used to hold configuration information.
type Config struct {
	Version string `json:"version"         yaml:"version"`
	Flags   Flags  `json:"flags,omitempty" yaml:"flags"`
}

// CommandLineFlags holds the list of command line flags used to configure the device plugin.
type CommandLineFlags struct {
	MigStrategy        string `json:"migStrategy"        yaml:"migStrategy"`
	FailOnInitError    bool   `json:"failOnInitError"    yaml:"failOnInitError"`
	PassDeviceSpecs    bool   `json:"passDeviceSpecs"    yaml:"passDeviceSpecs"`
	DeviceListStrategy string `json:"deviceListStrategy" yaml:"deviceListStrategy"`
	DeviceIDStrategy   string `json:"deviceIDStrategy"   yaml:"deviceIDStrategy"`
	NvidiaDriverRoot   string `json:"nvidiaDriverRoot"   yaml:"nvidiaDriverRoot"`
}

// Flags holds the full list of flags used to configure the device plugin.
type Flags struct {
	*CommandLineFlags
}

// parseConfig parses a config file as either YAML of JSON and unmarshals it into a Config struct.
func parseConfig(configFile string) (*Config, error) {
	reader, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer reader.Close()

	config, err := parseConfigFrom(reader)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return config, nil
}

func parseConfigFrom(reader io.Reader) (*Config, error) {
	var err error
	var configYaml []byte

	configYaml, err = ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(configYaml, &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal error: %v", err)
	}

	if config.Version == "" {
		return nil, fmt.Errorf("missing version field")
	}

	if config.Version != Version {
		return nil, fmt.Errorf("unknown version: %v", config.Version)
	}

	return &config, nil
}

// NewCommandLineFlags builds out a CommandLineFlags struct from the flags in cli.Context.
func NewCommandLineFlags(c *cli.Context) *CommandLineFlags {
	return &CommandLineFlags{
		MigStrategy:        c.String("mig-strategy"),
		FailOnInitError:    c.Bool("fail-on-init-error"),
		PassDeviceSpecs:    c.Bool("pass-device-specs"),
		DeviceListStrategy: c.String("device-list-strategy"),
		DeviceIDStrategy:   c.String("device-id-strategy"),
		NvidiaDriverRoot:   c.String("nvidia-driver-root"),
	}
}

// NewConfig builds out a Config struct from a config file (or command line flags).
// The data stored in the config will be populated in order of precedence from
// (1) command line, (2) environment variable, (3) config file.
func NewConfig(c *cli.Context, flags []cli.Flag) (*Config, error) {
	config := &Config{
		Version: Version,
		Flags:   Flags{NewCommandLineFlags(c)},
	}

	configFile := c.String("config-file")
	if configFile == "" {
		return config, nil
	}

	config, err := parseConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config file: %v", err)
	}

	commandLineFlagsFromConfig := map[interface{}]interface{}{
		"mig-strategy":         config.Flags.MigStrategy,
		"fail-on-init-error":   config.Flags.FailOnInitError,
		"pass-device-specs":    config.Flags.PassDeviceSpecs,
		"device-list-strategy": config.Flags.DeviceListStrategy,
		"device-id-strategy":   config.Flags.DeviceIDStrategy,
		"nvidia-driver-root":   config.Flags.NvidiaDriverRoot,
	}
	commandLineFlagsInputSource := altsrc.NewMapInputSource(configFile, commandLineFlagsFromConfig)

	err = altsrc.ApplyInputSourceValues(c, commandLineFlagsInputSource, flags)
	if err != nil {
		return nil, fmt.Errorf("unable to load command line flags from config: %v", err)
	}
	config.Flags.CommandLineFlags = NewCommandLineFlags(c)

	return config, nil
}
