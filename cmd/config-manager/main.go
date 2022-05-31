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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/prometheus/procfs"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// ResourceNodes represents the name the K8s resource 'nodes'
	ResourceNodes = "nodes"
)

// These constants represent the default value of flags to the CLI
const (
	DefaultOneshot         = false
	DefaultSendSignal      = true
	DefaultSignal          = int(syscall.SIGHUP)
	DefaultProcessToSignal = "nvidia-device-plugin"
	DefaultConfigLabel     = "nvidia.com/device-plugin.config"
)

// Flags holds configurable settings as set via the CLI
type Flags struct {
	Oneshot          bool
	Kubeconfig       string
	NodeName         string
	NodeLabel        string
	ConfigFileSrcdir string
	ConfigFileDst    string
	DefaultConfig    string
	SendSignal       bool
	Signal           int
	ProcessToSignal  string
}

// SyncableConfig is used to synchronize on changes to a configuration value
// That is, callers of Get() will block until a call to Set() is made.
// Multiple calls to Set() do not queue, meaning that only calls to Get() made
// *before* a call to Set() will be notified.
type SyncableConfig struct {
	cond         *sync.Cond
	mutex        sync.Mutex
	defaultValue string
	current      string
	lastRead     string
}

// NewSyncableConfig creates a new SyncableConfig
func NewSyncableConfig(f *Flags) *SyncableConfig {
	var m SyncableConfig
	m.cond = sync.NewCond(&m.mutex)
	m.defaultValue = f.DefaultConfig
	return &m
}

// Set sets the value of the config.
// All callers of Get() before the Set() will be unblocked.
func (m *SyncableConfig) Set(value string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if value == "" {
		log.Infof("No value set. Using default: %v", m.defaultValue)
		value = m.defaultValue
	}
	m.current = value
	m.cond.Broadcast()
}

// Get gets the value of the config.
// A call to Get() will block until a subsequent Set() call is made.
func (m *SyncableConfig) Get() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.lastRead == m.current {
		m.cond.Wait()
	}
	m.lastRead = m.current
	return m.lastRead
}

func main() {
	flags := Flags{}

	c := cli.NewApp()
	c.Before = func(c *cli.Context) error {
		return validateFlags(c, &flags)
	}
	c.Action = func(c *cli.Context) error {
		return start(c, &flags)
	}

	c.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "oneshot",
			Value:       DefaultOneshot,
			Usage:       "check and update the config only once and then exit",
			Destination: &flags.Oneshot,
			EnvVars:     []string{"ONESHOT"},
		},
		&cli.StringFlag{
			Name:        "kubeconfig",
			Value:       "",
			Usage:       "absolute path to the kubeconfig file",
			Destination: &flags.Kubeconfig,
			EnvVars:     []string{"KUBECONFIG"},
		},
		&cli.StringFlag{
			Name:        "node-name",
			Value:       "",
			Usage:       "the name of the node to watch for label changes on",
			Destination: &flags.NodeName,
			EnvVars:     []string{"NODE_NAME"},
		},
		&cli.StringFlag{
			Name:        "node-label",
			Value:       DefaultConfigLabel,
			Usage:       "the name of the node label to use for selecting a config",
			Destination: &flags.NodeLabel,
			EnvVars:     []string{"NODE_LABEL"},
		},
		&cli.StringFlag{
			Name:        "config-file-srcdir",
			Value:       "",
			Usage:       "the path to the directory containing available device configuration files",
			Destination: &flags.ConfigFileSrcdir,
			EnvVars:     []string{"CONFIG_FILE_SRCDIR"},
		},
		&cli.StringFlag{
			Name:        "config-file-dst",
			Value:       "",
			Usage:       "the path to destination device configuration file",
			Destination: &flags.ConfigFileDst,
			EnvVars:     []string{"CONFIG_FILE_DST"},
		},
		&cli.StringFlag{
			Name:        "default-config",
			Value:       "",
			Usage:       "the default config to use if no label is set",
			Destination: &flags.DefaultConfig,
			EnvVars:     []string{"DEFAULT_CONFIG"},
		},
		&cli.BoolFlag{
			Name:        "send-signal",
			Value:       DefaultSendSignal,
			Usage:       "send a signal to <process-to-signal> once a config change is made",
			Destination: &flags.SendSignal,
			EnvVars:     []string{"SEND_SIGNAL"},
		},
		&cli.IntFlag{
			Name:        "signal",
			Value:       DefaultSignal,
			Usage:       "the signal to sent to <process-to-signal> if <send-signal> is set",
			Destination: &flags.Signal,
			EnvVars:     []string{"SIGNAL"},
		},
		&cli.StringFlag{
			Name:        "process-to-signal",
			Value:       DefaultProcessToSignal,
			Usage:       "the name of the process to signal if <send-signal> is set",
			Destination: &flags.ProcessToSignal,
			EnvVars:     []string{"PROCESS_TO_SIGNAL"},
		},
	}

	err := c.Run(os.Args)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func validateFlags(c *cli.Context, f *Flags) error {
	if f.NodeName == "" {
		return fmt.Errorf("invalid <node-name>: must not be empty string")
	}
	if f.NodeLabel == "" {
		return fmt.Errorf("invalid <node-label>: must not be empty string")
	}
	if f.ConfigFileSrcdir == "" {
		return fmt.Errorf("invalid <config-file-srcdir>: must not be empty string")
	}
	if f.ConfigFileDst == "" {
		return fmt.Errorf("invalid <config-file-dst>: must not be empty string")
	}
	if f.DefaultConfig == "" {
		return fmt.Errorf("invalid <default-config>: must not be empty string")
	}
	return nil
}

func start(c *cli.Context, f *Flags) error {
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", f.Kubeconfig)
	if err != nil {
		return fmt.Errorf("error building kubernetes clientcmd config: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error building kubernetes clientset from config: %s", err)
	}

	config := NewSyncableConfig(f)

	stop := continuouslySyncConfigChanges(clientset, config, f)
	defer close(stop)

	for {
		log.Infof("Waiting for change to '%s' label", f.NodeLabel)
		err := updateConfig(config.Get(), f)
		if f.Oneshot {
			return err
		}
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

func continuouslySyncConfigChanges(clientset *kubernetes.Clientset, config *SyncableConfig, f *Flags) chan struct{} {
	listWatch := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		ResourceNodes,
		v1.NamespaceAll,
		fields.OneTermEqualSelector("metadata.name", f.NodeName),
	)

	_, controller := cache.NewInformer(
		listWatch, &v1.Node{}, 0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				config.Set(obj.(*v1.Node).Labels[f.NodeLabel])
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldLabel := oldObj.(*v1.Node).Labels[f.NodeLabel]
				newLabel := newObj.(*v1.Node).Labels[f.NodeLabel]
				if oldLabel != newLabel {
					config.Set(newLabel)
				}
			},
			DeleteFunc: func(obj interface{}) {
				oldLabel := obj.(*v1.Node).Labels[f.NodeLabel]
				if oldLabel != "" {
					config.Set("")
				}
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	return stop
}

func updateConfig(config string, f *Flags) error {
	log.Infof("Updating to config: %s", config)
	updated, err := updateSymlink(config, f)
	if err != nil {
		return err
	}
	if !updated {
		log.Infof("Already configured. Skipping update...")
		return nil
	}
	log.Infof("Successfully updated config: %s", config)

	if f.SendSignal {
		log.Infof("Sending signal '%s' to '%s'", syscall.Signal(f.Signal), f.ProcessToSignal)
		err := signalProcess(f)
		if err != nil {
			return err
		}
		log.Infof("Successfully sent signal")
	}

	return nil
}

func updateSymlink(config string, f *Flags) (bool, error) {
	src := filepath.Join(f.ConfigFileSrcdir, config)

	exists, err := fileExists(src)
	if err != nil {
		return false, fmt.Errorf("error checking if file '%s' exists: %v", src, err)
	}
	if !exists {
		return false, fmt.Errorf("unknown configuration: %v", config)
	}

	exists, err = fileExists(f.ConfigFileDst)
	if err != nil {
		return false, fmt.Errorf("error checking if file '%s' exists: %v", f.ConfigFileDst, err)
	}
	if exists {
		realpath, err := filepath.EvalSymlinks(f.ConfigFileDst)
		if err != nil {
			return false, fmt.Errorf("error evaluating realpath of '%v': %v", f.ConfigFileDst, err)
		}

		base := filepath.Base(realpath)
		if config == strings.TrimSuffix(base, filepath.Ext(base)) {
			return false, nil
		}

		err = os.Remove(f.ConfigFileDst)
		if err != nil {
			return false, fmt.Errorf("error removing existing config: %v", err)
		}
	}

	err = os.Symlink(src, f.ConfigFileDst)
	if err != nil {
		return false, fmt.Errorf("error creating symlink: %v", err)
	}

	return true, nil
}

func signalProcess(f *Flags) error {
	pid, err := findPidToSignal(f)
	if err != nil {
		return fmt.Errorf("error finding pid: %v", err)
	}
	err = syscall.Kill(pid, syscall.Signal(f.Signal))
	if err != nil {
		return fmt.Errorf("error sending signal: %v", err)
	}
	return nil
}

func findPidToSignal(f *Flags) (int, error) {
	procs, err := procfs.AllProcs()
	if err != nil {
		return -1, fmt.Errorf("error getting list of all procs: %v", err)
	}
	for _, p := range procs {
		cmdline, err := p.CmdLine()
		if err != nil {
			return -1, fmt.Errorf("error getting cmdline: %v", err)
		}
		if cmdline[0] == f.ProcessToSignal {
			return p.PID, nil
		}
	}
	return -1, fmt.Errorf("no process found")
}

func fileExists(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}
