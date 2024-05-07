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

package mount

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/urfave/cli/v2"
	"k8s.io/mount-utils"
)

type options struct {
	shmSize string
}

// NewCommand constructs a mount command.
func NewCommand() *cli.Command {
	opts := options{}

	c := cli.Command{
		Name:  "mount-shm",
		Usage: "Set up the /dev/shm mount required by the MPS daemon",
		Before: func(ctx *cli.Context) error {
			return validateFlags(ctx, &opts)
		},
		Action: func(ctx *cli.Context) error {
			return mountShm(ctx, &opts)
		},
	}

	c.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "dev-shm-size",
			Usage:       "Specify the size of the tmpfs that will be created at for use by the MPS daemon",
			Value:       "65536k",
			Destination: &opts.shmSize,
			EnvVars:     []string{"MPS_DEV_SHM_SIZE"},
		},
	}

	return &c
}

func validateFlags(c *cli.Context, opts *options) error {
	format := "[0-9]+[kmg%]?"
	if !regexp.MustCompile(format).MatchString(opts.shmSize) {
		return fmt.Errorf("dev-shm-size does not match format %q: %q", format, opts.shmSize)
	}

	return nil
}

// mountShm creates a tmpfs mount at /mps/shm to be used by the mps control daemon.
func mountShm(c *cli.Context, opts *options) error {
	mountExecutable, err := exec.LookPath("mount")
	if err != nil {
		return fmt.Errorf("error finding 'mount' executable: %w", err)
	}
	mounter := mount.New(mountExecutable)

	// TODO: /mps should be configurable.
	shmDir := "/mps/shm"
	err = mount.CleanupMountPoint(shmDir, mounter, true)
	if err != nil {
		return fmt.Errorf("error unmounting %v: %w", shmDir, err)
	}

	if err := os.MkdirAll(shmDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %v: %w", shmDir, err)
	}

	sizeArg := fmt.Sprintf("size=%v", opts.shmSize)
	mountOptions := []string{"rw", "nosuid", "nodev", "noexec", "relatime", sizeArg}
	if err := mounter.Mount("shm", shmDir, "tmpfs", mountOptions); err != nil {
		return fmt.Errorf("error mounting %v as tmpfs: %w", shmDir, err)
	}

	return nil
}
