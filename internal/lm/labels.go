/**
# Copyright (c) NVIDIA CORPORATION.  All rights reserved.
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
	"log"
	"os"
	"path/filepath"
	"strings"

	k8s "github.com/NVIDIA/gpu-feature-discovery/internal/kubernetes"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const nodeFeatureVendorPrefix = "nvidia-features-for"

// Labels defines a type for labels
type Labels map[string]string

// Labels also implements the Labeler interface
func (labels Labels) Labels() (Labels, error) {
	return labels, nil
}

// Output creates labels according to the specified output format.
func (labels Labels) Output(path string, nodeFeatureAPI bool) error {
	if nodeFeatureAPI {
		log.Print("Writing labels to NodeFeature CR")
		return labels.UpdateNodeFeatureObject()
	}

	return labels.UpdateFile(path)
}

// UpdateFile writes labels to the specified path. The file is written atomocally
func (labels Labels) UpdateFile(path string) error {
	log.Printf("Writing labels to output file %s", path)

	if path == "" {
		_, err := labels.WriteTo(os.Stdout)
		return err
	}

	output := new(bytes.Buffer)
	if _, err := labels.WriteTo(output); err != nil {
		return fmt.Errorf("error writing labels to buffer: %v", err)
	}
	err := writeFileAtomically(path, output.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error atomically writing file '%s': %v", path, err)
	}
	return nil
}

// WriteTo writes labels to the specified writer
func (labels Labels) WriteTo(output io.Writer) (int64, error) {
	var total int64
	for k, v := range labels {
		n, err := fmt.Fprintf(output, "%s=%s\n", k, v)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}

	return total, nil
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

// UpdateNodeFeatureObject creates/updates the node-specific NodeFeature custom resource.
func (labels Labels) UpdateNodeFeatureObject() error {
	cli, err := k8s.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %v", err)
	}

	nodename := k8s.NodeName()
	namespace := k8s.GetKubernetesNamespace()
	nodeFeatureName := strings.Join([]string{nodeFeatureVendorPrefix, nodename}, "-")

	if nfr, err := cli.NfdV1alpha1().NodeFeatures(namespace).Get(context.TODO(), nodeFeatureName, metav1.GetOptions{}); errors.IsNotFound(err) {
		log.Printf("creating NodeFeature object %s", nodeFeatureName)
		nfr = &nfdv1alpha1.NodeFeature{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{Name: nodeFeatureName, Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}},
			Spec:       nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels},
		}

		nfrCreated, err := cli.NfdV1alpha1().NodeFeatures(namespace).Create(context.TODO(), nfr, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeFeature object %q: %w", nfr.Name, err)
		}

		log.Printf("NodeFeature object created: %v", nfrCreated)
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFeature object: %w", err)
	} else {
		nfrUpdated := nfr.DeepCopy()
		nfrUpdated.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}
		nfrUpdated.Spec = nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels}

		if !apiequality.Semantic.DeepEqual(nfr, nfrUpdated) {
			log.Printf("updating NodeFeature object %s", nodeFeatureName)
			nfrUpdated, err = cli.NfdV1alpha1().NodeFeatures(namespace).Update(context.TODO(), nfrUpdated, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update NodeFeature object %q: %w", nfr.Name, err)
			}
			log.Printf("NodeFeature object updated: %v", nfrUpdated)
		} else {
			log.Printf("no changes in NodeFeature object, not updating")
		}
	}
	return nil
}
