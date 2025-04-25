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

	"github.com/NVIDIA/k8s-test-infra/pkg/diagnostics"
)

const (
	devicePluginEventuallyTimeout = 10 * time.Minute
)

// Actual test suite
var _ = Describe("GPU Device Plugin", Ordered, func() {
	// Init global suite vars vars
	var (
		helmReleaseName string
		chartSpec       helm.ChartSpec

		collectLogsFrom      []string
		diagnosticsCollector *diagnostics.Diagnostic
	)

	defaultCollectorObjects := []string{
		"pods",
		"nodes",
		"namespaces",
		"deployments",
		"daemonsets",
		"jobs",
	}

	values := helmValues.Options{
		Values: []string{
			fmt.Sprintf("image.repository=%s", ImageRepo),
			fmt.Sprintf("image.tag=%s", ImageTag),
			fmt.Sprintf("image.pullPolicy=%s", ImagePullPolicy),
			"devicePlugin.enabled=true",
			// We need to make affinity is none if not deploying NFD/GFD
			// test will fail if not run on a GPU node
			"affinity=",
		},
	}

	// check Collector objects
	collectLogsFrom = defaultCollectorObjects
	if CollectLogsFrom != "" && CollectLogsFrom != "default" {
		collectLogsFrom = strings.Split(CollectLogsFrom, ",")
	}

	BeforeAll(func(ctx context.Context) {
		// Create clients for apiextensions and our CRD api
		helmReleaseName = "nvdp-e2e-test-" + randomSuffix()

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
	})

	AfterEach(func(ctx context.Context) {
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

	When("When deploying k8s-device-plugin", Ordered, func() {
		It("it should create nvidia.com/gpu resource", func(ctx context.Context) {
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
		It("it should run GPU jobs", func(ctx context.Context) {
			By("Creating a GPU job")
			job, err := CreateOrUpdateJobsFromFile(ctx, clientSet, "job-1.yaml", testNamespace.Name)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for job to complete")
			Eventually(func() error {
				job, err := clientSet.BatchV1().Jobs(testNamespace.Name).Get(ctx, job[0], metav1.GetOptions{})
				if err != nil {
					return err
				}
				if job.Status.Succeeded != 1 {
					return fmt.Errorf("job %s/%s failed", job.Namespace, job.Name)
				}
				if job.Status.Succeeded == 1 {
					return nil
				}
				return fmt.Errorf("job %s/%s not completed yet", job.Namespace, job.Name)
			}, devicePluginEventuallyTimeout, 5*time.Second).Should(BeNil())
		})
	})
})
