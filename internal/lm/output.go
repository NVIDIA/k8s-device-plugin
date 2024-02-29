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

package lm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/flags"
)

// Outputer defines a mechanism to output labels.
type Outputer interface {
	Output(Labels) error
}

// TODO: Replace this with functional options.
func NewOutputer(config *spec.Config, nodeConfig flags.NodeConfig, clientSets flags.ClientSets) Outputer {
	if config.Flags.UseNodeFeatureAPI == nil || !*config.Flags.UseNodeFeatureAPI {
		return ToFile(*config.Flags.GFD.OutputFile)
	}

	return &nodeFeatureObject{
		nodeConfig:   nodeConfig,
		nfdClientset: clientSets.NFD,
	}
}

func ToFile(path string) Outputer {
	if path == "" {
		return &toWriter{os.Stdout}
	}

	o := toFile(path)
	return &o
}

// toFile writes to the specified file.
type toFile string

// toWriter writes to the specified writer
type toWriter struct {
	io.Writer
}

func (path *toFile) Output(labels Labels) error {
	klog.Infof("Writing labels to output file %v", *path)

	buffer := new(bytes.Buffer)
	output := &toWriter{buffer}
	if err := output.Output(labels); err != nil {
		return fmt.Errorf("error writing labels to buffer: %v", err)
	}
	err := writeFileAtomically(string(*path), buffer.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error atomically writing file '%s': %w", *path, err)
	}
	return nil
}

func (output *toWriter) Output(labels Labels) error {
	for k, v := range labels {
		_, err := fmt.Fprintf(output, "%s=%s\n", k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFileAtomically(path string, contents []byte, perm os.FileMode) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to retrieve absolute path of output file: %v", err)
	}

	absDir := filepath.Dir(absPath)
	tmpDir := filepath.Join(absDir, "gfd-tmp")

	err = os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()

	tmpFile, err := os.CreateTemp(tmpDir, "gfd-")
	if err != nil {
		return fmt.Errorf("fail to create temporary output file: %v", err)
	}
	defer func() {
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	}()

	err = os.WriteFile(tmpFile.Name(), contents, perm)
	if err != nil {
		return fmt.Errorf("error writing temporary file '%v': %v", tmpFile.Name(), err)
	}

	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		return fmt.Errorf("error moving temporary file to '%v': %v", path, err)
	}

	err = os.Chmod(path, perm)
	if err != nil {
		return fmt.Errorf("error setting permissions on '%v': %v", path, err)
	}

	return nil
}

const nodeFeatureVendorPrefix = "nvidia-features-for"

type nodeFeatureObject struct {
	nodeConfig   flags.NodeConfig
	nfdClientset nfdclientset.Interface
}

// UpdateNodeFeatureObject creates/updates the node-specific NodeFeature custom resource.
func (n *nodeFeatureObject) Output(labels Labels) error {
	nodename := n.nodeConfig.Name
	namespace := n.nodeConfig.Namespace
	nodeFeatureName := strings.Join([]string{nodeFeatureVendorPrefix, nodename}, "-")

	if nfr, err := n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Get(context.TODO(), nodeFeatureName, metav1.GetOptions{}); errors.IsNotFound(err) {
		klog.Infof("creating NodeFeature object %s", nodeFeatureName)
		nfr = &nfdv1alpha1.NodeFeature{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{Name: nodeFeatureName, Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}},
			Spec:       nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels},
		}

		nfrCreated, err := n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Create(context.TODO(), nfr, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeFeature object %q: %w", nfr.Name, err)
		}

		klog.Infof("NodeFeature object created: %v", nfrCreated)
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFeature object: %w", err)
	} else {
		nfrUpdated := nfr.DeepCopy()
		nfrUpdated.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}
		nfrUpdated.Spec = nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels}

		if !apiequality.Semantic.DeepEqual(nfr, nfrUpdated) {
			klog.Infof("updating NodeFeature object %s", nodeFeatureName)
			nfrUpdated, err = n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Update(context.TODO(), nfrUpdated, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update NodeFeature object %q: %w", nfr.Name, err)
			}
			klog.Infof("NodeFeature object updated: %v", nfrUpdated)
		} else {
			klog.Infof("no changes in NodeFeature object, not updating")
		}
	}
	return nil
}
