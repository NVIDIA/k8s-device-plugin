/*
 * SPDX-FileCopyrightText: Copyright (c) 2025 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
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
package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// cleanupNamespaceResources removes all resources in the specified namespace.
func cleanupNamespaceResources(namespace string) {
	err := cleanupTestPods(namespace)
	Expect(err).NotTo(HaveOccurred())

	err = cleanupHelmDeployments(namespace)
	Expect(err).NotTo(HaveOccurred())

	cleanupNode(ctx, clientSet)
	cleanupNFDObjects(ctx, nfdClient, testNamespace.Name)
	cleanupCRDs()
}

// waitForDeletion polls the provided checkFunc until a NotFound error is returned,
// confirming that the resource is deleted.
func waitForDeletion(resourceName string, checkFunc func() error) error {
	timeout := 2 * time.Minute
	interval := 5 * time.Second
	start := time.Now()
	for {
		err := checkFunc()
		if err != nil && errors.IsNotFound(err) {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timed out waiting for deletion of %s", resourceName)
		}
		time.Sleep(interval)
	}
}

// cleanupTestPods deletes all test Pods in the namespace that have the label "app.nvidia.com=k8s-dra-driver-gpu-test-app".
func cleanupTestPods(namespace string) error {
	labelSelector := "app.nvidia.com=k8s-device-plugin-test-app"
	podList, err := clientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}

	zero := int64(0)
	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &zero}
	for _, pod := range podList.Items {
		if err = clientSet.CoreV1().Pods(namespace).Delete(ctx, pod.Name, deleteOptions); err != nil {
			return err
		}
		if err = waitForDeletion(pod.Name, func() error {
			_, err := clientSet.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// cleanupHelmDeployments uninstalls all deployed Helm releases in the specified namespace.
func cleanupHelmDeployments(namespace string) error {
	releases, err := helmClient.ListDeployedReleases()
	if err != nil {
		return fmt.Errorf("failed to list deployed releases: %w", err)
	}

	for _, release := range releases {
		// Check if the release is deployed in the target namespace.
		// Depending on your helmClient configuration the release might carry the namespace information.
		if release.Namespace == namespace {
			if err := helmClient.UninstallReleaseByName(release.Name); err != nil {
				return fmt.Errorf("failed to uninstall release %q: %w", release.Name, err)
			}
		}
	}
	return nil
}

// deleteTestNamespace deletes the test namespace and waits for its deletion.
func deleteTestNamespace() {
	defer func() {
		err := clientSet.CoreV1().Namespaces().Delete(ctx, testNamespace.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
		err = waitForDeletion(testNamespace.Name, func() error {
			_, err := clientSet.CoreV1().Namespaces().Get(ctx, testNamespace.Name, metav1.GetOptions{})
			return err
		})
		Expect(err).NotTo(HaveOccurred())
	}()
}

// cleanupCRDs deletes specific CRDs used during testing.
func cleanupCRDs() {
	crds := []string{
		"nodefeatures.nfd.k8s-sigs.io",
		"nodefeaturegroups.nfd.k8s-sigs.io",
		"nodefeaturerules.nfd.k8s-sigs.io",
	}

	for _, crd := range crds {
		err := extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		_ = waitForDeletion(crd, func() error {
			_, err := extClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd, metav1.GetOptions{})
			return err
		})
	}
}

// cleanupNode deletes all NFD/GFD related metadata from the Node object, i.e.
// labels and annotations
func cleanupNode(ctx context.Context, cs clientset.Interface) {
	// Per-node cleanup function
	cleanup := func(nodeName string) error {
		node, err := cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		update := false
		updateStatus := false
		// Gather info about all NFD-managed node assets outside the default prefix
		nfdLabels := map[string]struct{}{}
		for _, name := range strings.Split(node.Annotations[nfdv1alpha1.FeatureLabelsAnnotation], ",") {
			if strings.Contains(name, "/") {
				nfdLabels[name] = struct{}{}
			}
		}
		nfdERs := map[string]struct{}{}
		for _, name := range strings.Split(node.Annotations[nfdv1alpha1.ExtendedResourceAnnotation], ",") {
			if strings.Contains(name, "/") {
				nfdERs[name] = struct{}{}
			}
		}

		// Remove labels
		for key := range node.Labels {
			_, ok := nfdLabels[key]
			if ok || strings.HasPrefix(key, nfdv1alpha1.FeatureLabelNs) {
				delete(node.Labels, key)
				update = true
			}
		}

		// Remove annotations
		for key := range node.Annotations {
			if strings.HasPrefix(key, nfdv1alpha1.AnnotationNs) {
				delete(node.Annotations, key)
				update = true
			}
		}

		// Remove nvidia.com/ labels
		for key := range node.Labels {
			if strings.HasPrefix(key, "nvidia.com/") {
				delete(node.Labels, key)
				update = true
			}
		}

		// Remove extended resources
		for key := range node.Status.Capacity {
			// We check for FeatureLabelNs as -resource-labels can create ERs there
			_, ok := nfdERs[string(key)]
			if ok || strings.HasPrefix(string(key), nfdv1alpha1.FeatureLabelNs) {
				delete(node.Status.Capacity, key)
				delete(node.Status.Allocatable, key)
				updateStatus = true
			}
		}

		if updateStatus {
			By("[Cleanup]\tDeleting NFD extended resources from node " + nodeName)
			if _, err := cs.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}

		if update {
			By("[Cleanup]\tDeleting NFD labels, annotations and taints from node " + node.Name)
			if _, err := cs.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}

		return nil
	}

	// Cleanup all nodes
	nodeList, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, n := range nodeList.Items {
		var err error
		for retry := 0; retry < 5; retry++ {
			if err = cleanup(n.Name); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		Expect(err).NotTo(HaveOccurred())
	}
}

func cleanupNFDObjects(ctx context.Context, cli *nfdclient.Clientset, namespace string) {
	cleanupNodeFeatureRules(ctx, cli)
	cleanupNodeFeatures(ctx, cli, namespace)
}

// cleanupNodeFeatures deletes all NodeFeature objects in the given namespace
func cleanupNodeFeatures(ctx context.Context, cli *nfdclient.Clientset, namespace string) {
	nfs, err := cli.NfdV1alpha1().NodeFeatures(namespace).List(ctx, metav1.ListOptions{})
	if errors.IsNotFound(err) {
		// Omitted error, nothing to do.
		return
	}
	Expect(err).NotTo(HaveOccurred())

	if len(nfs.Items) != 0 {
		By("[Cleanup]\tDeleting NodeFeature objects from namespace " + namespace)
		for _, nf := range nfs.Items {
			err = cli.NfdV1alpha1().NodeFeatures(namespace).Delete(ctx, nf.Name, metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				// Omitted error
				continue
			}
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

// cleanupNodeFeatureRules deletes all NodeFeatureRule objects
func cleanupNodeFeatureRules(ctx context.Context, cli *nfdclient.Clientset) {
	nfrs, err := cli.NfdV1alpha1().NodeFeatureRules().List(ctx, metav1.ListOptions{})
	if errors.IsNotFound(err) {
		// Omitted error, nothing to do.
		return
	}
	Expect(err).NotTo(HaveOccurred())

	if len(nfrs.Items) != 0 {
		By("[Cleanup]\tDeleting NodeFeatureRule objects from the cluster")
		for _, nfr := range nfrs.Items {
			err = cli.NfdV1alpha1().NodeFeatureRules().Delete(ctx, nfr.Name, metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				// Omitted error
				continue
			}
			Expect(err).NotTo(HaveOccurred())
		}
	}
}
