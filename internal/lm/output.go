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
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"

	"github.com/google/renameio"

	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/flags"
)

// Outputer defines a mechanism to output labels.
type Outputer interface {
	Output(Labels) error
}

// TODO: Replace this with functional options.
func NewOutputer(config *spec.Config, nodeConfig flags.NodeConfig, clientSets flags.ClientSets) (Outputer, error) {
	if config.Flags.UseNodeFeatureAPI == nil || !*config.Flags.UseNodeFeatureAPI {
		return ToFile(*config.Flags.GFD.OutputFile), nil
	}

	if nodeConfig.Name == "" {
		return nil, fmt.Errorf("required flag node-name not set")
	}
	if nodeConfig.Namespace == "" {
		return nil, fmt.Errorf("required flag namespace not set")
	}

	ownerRefs, err := getOwnerReferences(context.TODO(), clientSets.Core, nodeConfig.Namespace, nodeConfig.PodName)
	if err != nil {
		// Log the error but continue without owner references.
		klog.Warningf("Failed to resolve owner references: %v", err)
	}

	o := nodeFeatureObject{
		nodeConfig:   nodeConfig,
		nfdClientset: clientSets.NFD,
		ownerRefs:    ownerRefs,
	}
	return &o, nil
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
	// write file atomically
	if err := renameio.WriteFile(string(*path), buffer.Bytes(), 0644); err != nil {
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

const nodeFeatureVendorPrefix = "nvidia-features-for"

type nodeFeatureObject struct {
	nodeConfig   flags.NodeConfig
	nfdClientset nfdclientset.Interface
	ownerRefs    []metav1.OwnerReference
}

// Output creates/updates the node-specific NodeFeature custom resource.
func (n *nodeFeatureObject) Output(labels Labels) error {
	nodename := n.nodeConfig.Name
	if nodename == "" {
		return fmt.Errorf("required flag %q not set", "node-name")
	}
	namespace := n.nodeConfig.Namespace
	nodeFeatureName := strings.Join([]string{nodeFeatureVendorPrefix, nodename}, "-")

	nfr, err := n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Get(context.TODO(), nodeFeatureName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get NodeFeature object: %w", err)
	}

	if errors.IsNotFound(err) {
		klog.Infof("creating NodeFeature object %s", nodeFeatureName)
		nfr = &nfdv1alpha1.NodeFeature{
			ObjectMeta: metav1.ObjectMeta{
				Name:            nodeFeatureName,
				Labels:          map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename},
				OwnerReferences: n.ownerRefs,
			},
			Spec: nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels},
		}
		nfrCreated, err := n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Create(context.TODO(), nfr, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create NodeFeature object %q: %w", nfr.Name, err)
		}
		klog.Infof("created NodeFeature object %v", nfrCreated)
		return nil
	}

	nfrUpdated := nfr.DeepCopy()
	nfrUpdated.Labels = map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodename}
	nfrUpdated.Spec = nfdv1alpha1.NodeFeatureSpec{Features: *nfdv1alpha1.NewFeatures(), Labels: labels}
	nfrUpdated.OwnerReferences = n.ownerRefs

	if apiequality.Semantic.DeepEqual(nfr, nfrUpdated) {
		klog.Infof("no changes in NodeFeature object %s", nodeFeatureName)
		return nil
	}

	klog.Infof("Updating NodeFeature object %s", nodeFeatureName)
	nfrUpdated, err = n.nfdClientset.NfdV1alpha1().NodeFeatures(namespace).Update(context.TODO(), nfrUpdated, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update NodeFeature object %q: %w", nfr.Name, err)
	}
	klog.Infof("NodeFeature object updated: %v", nfrUpdated)
	return nil
}

// getOwnerReferences returns owner references for the DaemonSet and Pod that owns this process.
// This ensures NodeFeature CRs are garbage collected when the DaemonSet is deleted.
func getOwnerReferences(ctx context.Context, client kubernetes.Interface, namespace, podName string) ([]metav1.OwnerReference, error) {
	if podName == "" {
		klog.Info("Pod name not provided, skipping owner reference resolution")
		return nil, nil
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	var dsOwnerRef *metav1.OwnerReference
	for i := range pod.OwnerReferences {
		if pod.OwnerReferences[i].Kind == "DaemonSet" {
			dsOwnerRef = &pod.OwnerReferences[i]
			break
		}
	}

	if dsOwnerRef == nil {
		klog.Info("Pod is not owned by a DaemonSet, skipping owner reference resolution")
		return nil, nil
	}

	controller := true
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: dsOwnerRef.APIVersion,
			Kind:       dsOwnerRef.Kind,
			Name:       dsOwnerRef.Name,
			UID:        dsOwnerRef.UID,
			Controller: &controller,
		},
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       pod.Name,
			UID:        pod.UID,
		},
	}

	return ownerRefs, nil
}
