/*
 * SPDX-FileCopyrightText: Copyright (c) 2023 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	helm "github.com/mittwald/go-helm-client"
	helmValues "github.com/mittwald/go-helm-client/values"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/common/diagnostics"
)

const (
	devicePluginEventuallyTimeout = 10 * time.Minute
)

// Actual test suite
var _ = Describe("GPU Device Plugin", Ordered, Label("gpu", "e2e", "device-plugin"), func() {
	// Init global suite vars
	var (
		helmReleaseName string
		chartSpec       helm.ChartSpec

		collectLogsFrom      []string
		diagnosticsCollector *diagnostics.Diagnostic
	)

	collectLogsFrom = []string{
		"pods",
		"nodes",
		"namespaces",
		"deployments",
		"daemonsets",
		"jobs",
	}
	if CollectLogsFrom != "" && CollectLogsFrom != "default" {
		collectLogsFrom = strings.Split(CollectLogsFrom, ",")
	}

	values := helmValues.Options{
		Values: []string{
			fmt.Sprintf("image.repository=%s", ImageRepo),
			fmt.Sprintf("image.tag=%s", ImageTag),
			fmt.Sprintf("image.pullPolicy=%s", ImagePullPolicy),
			"devicePlugin.enabled=true",
			// We need to make affinity null, if not deploying NFD/GFD
			// test will fail if not run on a GPU node
			"affinity=",
		},
	}

	When("deploying k8s-device-plugin", Ordered, func() {
		BeforeAll(func(ctx SpecContext) {
			// Create clients for apiextensions and our CRD api
			helmReleaseName = "nvdp-e2e-test-" + rand.String(5)

			chartSpec = helm.ChartSpec{
				ReleaseName:   helmReleaseName,
				ChartName:     HelmChart,
				Namespace:     testNamespace.Name,
				Wait:          true,
				Timeout:       1 * time.Minute,
				ValuesOptions: values,
				CleanupOnFail: true,
			}

			By("Installing k8s-device-plugin Helm chart")
			_, err := helmClient.InstallChart(ctx, &chartSpec, nil)
			Expect(err).NotTo(HaveOccurred())

			// Wait for all DaemonSets to be ready
			// Note: DaemonSet names are dynamically generated with the Helm release prefix,
			// so we wait for all DaemonSets in the namespace rather than specific names
			By("Waiting for all DaemonSets to be ready")
			err = waitForDaemonSetsReady(ctx, clientSet, testNamespace.Name, "app.kubernetes.io/name=nvidia-device-plugin")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func(ctx SpecContext) {
			By("Uninstalling k8s-device-plugin Helm chart")
			err := helmClient.UninstallReleaseByName(helmReleaseName)
			if err != nil {
				GinkgoWriter.Printf("Failed to uninstall helm release %s: %v\n", helmReleaseName, err)
			}
		})

		AfterEach(func(ctx SpecContext) {
			// Run diagnostic collector if test failed
			if CurrentSpecReport().Failed() {
				var err error
				diagnosticsCollector, err = diagnostics.New(
					diagnostics.WithNamespace(testNamespace.Name),
					diagnostics.WithArtifactDir(LogArtifactDir),
					diagnostics.WithKubernetesClient(clientSet),
					diagnostics.WithObjects(collectLogsFrom...),
				)
				Expect(err).NotTo(HaveOccurred())

				err = diagnosticsCollector.Collect(ctx)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should create nvidia.com/gpu resource", func(ctx SpecContext) {
			nodeList, err := clientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(nodeList.Items)).ToNot(BeZero())

			// We pick one node
			nodes, err := getNonControlPlaneNodes(ctx, clientSet)
			Expect(err).NotTo(HaveOccurred())

			targetNodeName := nodes[0].Name
			Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

			By("Checking the node capacity")
			capacityChecker := map[string]k8sLabels{
				targetNodeName: {
					"nvidia.com/gpu": "^[1-9]$",
				}}
			eventuallyNonControlPlaneNodes(ctx, clientSet).Should(MatchCapacity(capacityChecker, nodes), "Node capacity does not match")
		})
		It("should run GPU jobs", func(ctx SpecContext) {
			By("Creating a GPU job")
			jobNames, err := CreateOrUpdateJobsFromFile(ctx, clientSet, testNamespace.Name, filepath.Join(projectRoot, "testdata", "job-1.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(jobNames).To(HaveLen(1))

			// Defer cleanup for the job
			DeferCleanup(func(ctx SpecContext) {
				By("Deleting the GPU job")
				err := clientSet.BatchV1().Jobs(testNamespace.Name).Delete(ctx, jobNames[0], metav1.DeleteOptions{})
				if err != nil {
					GinkgoWriter.Printf("Failed to delete job %s: %v\n", jobNames[0], err)
				}
			})

			By("Waiting for job to complete")
			Eventually(func(g Gomega) error {
				job, err := clientSet.BatchV1().Jobs(testNamespace.Name).Get(ctx, jobNames[0], metav1.GetOptions{})
				if err != nil {
					return err
				}
				if job.Status.Failed > 0 {
					return fmt.Errorf("job %s/%s has failed pods: %d", job.Namespace, job.Name, job.Status.Failed)
				}
				if job.Status.Succeeded != 1 {
					return fmt.Errorf("job %s/%s not completed yet: %d succeeded", job.Namespace, job.Name, job.Status.Succeeded)
				}
				return nil
			}).WithContext(ctx).WithPolling(5 * time.Second).WithTimeout(devicePluginEventuallyTimeout).Should(Succeed())
		})
	})
})
