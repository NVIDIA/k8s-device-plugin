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

	helm "github.com/mittwald/go-helm-client"
	helmValues "github.com/mittwald/go-helm-client/values"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/NVIDIA/k8s-test-infra/pkg/diagnostics"
)

// Actual test suite
var _ = Describe("GPU Feature Discovery", Ordered, func() {
	expectedLabelPatterns := k8sLabels{
		"nvidia.com/gfd.timestamp":        "[0-9]{10}",
		"nvidia.com/cuda.driver.major":    "[0-9]+",
		"nvidia.com/cuda.driver.minor":    "[0-9]+",
		"nvidia.com/cuda.driver.rev":      "[0-9]*",
		"nvidia.com/cuda.runtime.major":   "[0-9]+",
		"nvidia.com/cuda.runtime.minor":   "[0-9]+",
		"nvidia.com/gpu.machine":          ".*",
		"nvidia.com/gpu.count":            "[0-9]+",
		"nvidia.com/gpu.replicas":         "[0-9]+",
		"nvidia.com/gpu.sharing-strategy": "[none|mps|time-slicing]",
		"nvidia.com/gpu.product":          "[A-Za-z_-]+",
		"nvidia.com/gpu.memory":           "[0-9]+",
		"nvidia.com/gpu.family":           "[a-z]+",
		"nvidia.com/mig.capable":          "[true|false]",
		"nvidia.com/gpu.compute.major":    "[0-9]+",
		"nvidia.com/gpu.compute.minor":    "[0-9]+",
		"nvidia.com/mps.capable":          "[true|false]",
	}

	defaultCollectorObjects := []string{
		"pods",
		"nodes",
		"namespaces",
		"deployments",
		"daemonsets",
		"nodeFeature",
	}

	// Init global suite vars vars
	var (
		helmReleaseName string
		chartSpec       helm.ChartSpec

		collectLogsFrom      []string
		diagnosticsCollector diagnostics.Collector
	)

	values := helmValues.Options{
		Values: []string{
			fmt.Sprintf("image.repository=%s", ImageRepo),
			fmt.Sprintf("image.tag=%s", ImageTag),
			fmt.Sprintf("image.pullPolicy=%s", ImagePullPolicy),
			"gfd.enabled=true",
			"devicePlugin.enabled=false",
		},
	}

	// checkNodeFeatureObject is a helper function to check if NodeFeature object was created
	checkNodeFeatureObject := func(ctx context.Context, name string) bool {
		gfdNodeFeature := fmt.Sprintf("nvidia-features-for-%s", name)
		_, err := nfdClient.NfdV1alpha1().NodeFeatures(testNamespace.Name).Get(ctx, gfdNodeFeature, metav1.GetOptions{})
		return err == nil
	}

	// check Collector objects
	collectLogsFrom = defaultCollectorObjects
	if CollectLogsFrom != "" && CollectLogsFrom != "default" {
		collectLogsFrom = strings.Split(CollectLogsFrom, ",")
	}

	BeforeAll(func(ctx context.Context) {
		helmReleaseName = "gfd-e2e-test" + rand.String(5)

		// reset Helm Client
		chartSpec = helm.ChartSpec{
			ReleaseName:   helmReleaseName,
			ChartName:     HelmChart,
			Namespace:     testNamespace.Name,
			Wait:          true,
			Timeout:       1 * time.Minute,
			ValuesOptions: values,
			CleanupOnFail: true,
		}

		By("Installing GFD Helm chart")
		_, err := helmClient.InstallChart(ctx, &chartSpec, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	// Cleanup before next test run
	AfterEach(func(ctx context.Context) {
		// Run diagnostic collector if test failed
		if CurrentSpecReport().Failed() {
			var err error
			diagnosticsCollector, err = diagnostics.New(
				diagnostics.WithNamespace(testNamespace.Name),
				diagnostics.WithArtifactDir(LogArtifactDir),
				diagnostics.WithKubernetesClient(clientSet),
				diagnostics.WithNFDClient(nfdClient),
				diagnostics.WithObjects(collectLogsFrom...),
			)
			Expect(err).NotTo(HaveOccurred())

			err = diagnosticsCollector.Collect(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	When("When deploying GFD", Ordered, func() {
		Context("NV Driver is not installed", func() {
			It("it should create nvidia.com timestamp label", func(ctx context.Context) {
				nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				// We pick one node targeted for our NodeFeature objects
				nodes, err := getNonControlPlaneNodes(ctx, clientSet)
				Expect(err).NotTo(HaveOccurred())

				targetNodeName := nodes[0].Name
				Expect(targetNodeName).ToNot(BeEmpty())

				By("Checking the node labels")
				labelChecker := map[string]k8sLabels{
					targetNodeName: {
						"nvidia.com/gfd.timestamp": "[0-9]{10}",
					}}
				eventuallyNonControlPlaneNodes(ctx, clientSet).Should(MatchLabels(labelChecker, nodes))
			})
			Context("and the NodeFeature API is enabled", func() {
				It("gfd should create node feature object", func(ctx context.Context) {
					By("Updating GFD Helm chart values")
					newValues := values
					newValues.Values = append(newValues.Values, "nfd.enableNodeFeatureApi=true")
					chartSpec.ValuesOptions = newValues
					chartSpec.Replace = true
					_, err := helmClient.UpgradeChart(ctx, &chartSpec, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Checking if NodeFeature CR object is created")
					nodes, err := getNonControlPlaneNodes(ctx, clientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty())
					Eventually(func() bool {
						return checkNodeFeatureObject(ctx, targetNodeName)
					}, 2*time.Minute, 5*time.Second).Should(BeTrue())

					By("Checking that node labels are created from NodeFeature object")
					labelChecker := map[string]k8sLabels{
						targetNodeName: {
							"nvidia.com/gfd.timestamp": "[0-9]{10}",
						}}
					eventuallyNonControlPlaneNodes(ctx, clientSet).Should(MatchLabels(labelChecker, nodes))
				})
			})
		})

		When("NV Driver is installed", func() {
			It("it should create nvidia.com labels", func(ctx context.Context) {
				if !NVIDIA_DRIVER_ENABLED {
					Skip("NVIDIA_DRIVER_ENABLED is not set")
				}

				By("Checking the node labels")
				nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				// We pick one node targeted for our NodeFeature objects
				nodes, err := getNonControlPlaneNodes(ctx, clientSet)
				Expect(err).NotTo(HaveOccurred())

				targetNodeName := nodes[0].Name
				Expect(targetNodeName).ToNot(BeEmpty())

				By("Checking the node labels")
				labelChecker := map[string]k8sLabels{
					targetNodeName: expectedLabelPatterns}
				eventuallyNonControlPlaneNodes(ctx, clientSet).Should(MatchLabels(labelChecker, nodes))
			})
			Context("and the NodeFeature API is enabled", func() {
				It("gfd should create node feature object", func(ctx context.Context) {
					if !NVIDIA_DRIVER_ENABLED {
						Skip("NVIDIA_DRIVER_ENABLED is not set")
					}
					By("Updating GFD Helm chart values")
					newValues := values
					newValues.Values = append(newValues.Values, "nfd.enableNodeFeatureApi=true")
					chartSpec.ValuesOptions = newValues
					chartSpec.Replace = true
					_, err := helmClient.UpgradeChart(ctx, &chartSpec, nil)
					Expect(err).NotTo(HaveOccurred())

					By("Checking if NodeFeature CR object is created")
					nodes, err := getNonControlPlaneNodes(ctx, clientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty())
					Eventually(func() bool {
						return checkNodeFeatureObject(ctx, targetNodeName)
					}, 2*time.Minute, 5*time.Second).Should(BeTrue())

					By("Checking that node labels are created from NodeFeature CR object")
					checkForLabels := map[string]k8sLabels{
						targetNodeName: expectedLabelPatterns}
					eventuallyNonControlPlaneNodes(ctx, clientSet).Should(MatchLabels(checkForLabels, nodes))
				})
			})
		})
	})
})
