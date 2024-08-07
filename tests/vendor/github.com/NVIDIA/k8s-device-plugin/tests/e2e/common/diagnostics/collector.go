/**
# Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package diagnostics

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	"sigs.k8s.io/yaml"
)

type Collector interface {
	Collect(context.Context) error
}

type Config struct {
	Clientset kubernetes.Interface
	NfdClient *nfdclient.Clientset

	artifactDir string
	namespace   string

	log io.Writer
}

func (c *Config) createFile(fp string) (io.WriteCloser, error) {
	outfile, err := os.Create(filepath.Join(c.artifactDir, c.namespace, fp))
	if err != nil {
		return nil, fmt.Errorf("error creating %v: %w", fp, err)
	}
	return outfile, nil
}

func (c *Config) writeToFile(w io.Writer, data interface{}) error {
	// Marshal data to YAML format
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}

	// Write marshaled bytes to the provided io.Writer
	_, err = w.Write(yamlBytes)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

func (c *Config) outputTo(filename string, objects interface{}) error {
	outputfile, err := c.createFile(filename)
	if err != nil {
		return fmt.Errorf("error creating %v: %w", filename, err)
	}
	defer outputfile.Close()
	if err = c.writeToFile(outputfile, objects); err != nil {
		return fmt.Errorf("error writing to %v: %w", filename, err)
	}
	return nil
}

func (d *Diagnostic) Collect(ctx context.Context) error {
	// Create the artifact directory
	if err := os.MkdirAll(filepath.Join(d.Config.artifactDir, d.Config.namespace), os.ModePerm); err != nil {
		return fmt.Errorf("error creating artifact directory: %w", err)
	}

	// Redirect stdout and stderr to logs
	logFile, err := d.createFile("diagnostic_collector.log")
	if err != nil {
		return fmt.Errorf("error creating collector log file: %w", err)
	}
	defer logFile.Close()
	d.log = logFile

	// configure klog to write to the log file
	klog.SetOutput(d.log)

	if len(d.collectors) == 0 {
		klog.Warning("No collectors to run")
	}

	// Run the collectors
	for _, c := range d.collectors {
		if err := c.Collect(ctx); err != nil {
			klog.ErrorS(err, "Error running collector")
		}
	}

	return nil
}
