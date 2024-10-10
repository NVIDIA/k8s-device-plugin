/**
# Copyright 2024 NVIDIA CORPORATION
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

package imex

import (
	"os"
	"path/filepath"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
)

// Channels represents a set of IMEX channels.
type Channels []*Channel

// Channel represents an IMEX channel.
type Channel struct {
	id      string
	devRoot string
}

// GetPaths returns the paths for all IMEX channels in a set of IMEX channels.
func (c *Channels) GetPaths() []string {
	if c == nil {
		return nil
	}
	var paths []string
	for _, channel := range *c {
		paths = append(paths, channel.Path())
	}
	return paths
}

// GetChannels returns the set of channels for the given config.
// If the selection of the default IMEX channel is disabled no channels are returned.
func GetChannels(config *spec.Config, devRoot string) Channels {
	if channel := resolveDefaultChannel(config, devRoot); channel != nil {
		return []*Channel{channel}
	}

	return nil
}

// resolveDefaultChannel checks the default imex channel strategy.
// If a strategy of 'auto' is configured, IMEX is only enabled if the device
// node for the default channel exists.
func resolveDefaultChannel(config *spec.Config, devRoot string) *Channel {
	channel := defaultChannel(devRoot)

	switch config.Imex.DefaultChannelStrategy {
	case spec.DefaultChannelStrategyEnabled:
		return &channel
	case spec.DefaultChannelStrategyAuto:
		if !channel.exists() {
			return nil
		}
		return &channel
	}

	return nil
}

// defaultChannel constructs a default IMEX channel for the specified devRoot.
func defaultChannel(devRoot string) Channel {
	return Channel{
		id:      "0",
		devRoot: devRoot,
	}
}

// ID returns an identifier for the channel.
func (c Channel) ID() string {
	return c.id
}

// Name returns the channel name.
func (c Channel) Name() string {
	return "channel" + c.id
}

// Path returns the absolute path to an IMEX channel.
func (c Channel) Path() string {
	return filepath.Join("/dev/nvidia-caps-imex-channels", c.Name())
}

// HostPath returns the path to the IMEX channel adjusted for the configured devRoot.
func (c Channel) HostPath() string {
	return filepath.Join(c.devRoot, c.Path())
}

// exists checks whether the channel exists at the host path.
func (c Channel) exists() bool {
	info, err := os.Stat(c.HostPath())
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		// TODO: We may want to log this error instead.
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
