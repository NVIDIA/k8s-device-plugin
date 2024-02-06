/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	helm "github.com/mittwald/go-helm-client"
	helmValues "github.com/mittwald/go-helm-client/values"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	nfdclient "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"

	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/common"
	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework"
	e2elog "github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework/logs"
)

// Actual test suite
var _ = NVDescribe("GPU Feature Discovery", func() {
	f := framework.NewFramework("gpu-feature-discovery")

	expectedLabelPatterns := k8sLabels{
		"nvidia.com/gfd.timestamp":       "[0-9]{10}",
		"nvidia.com/cuda.driver.major":   "[0-9]+",
		"nvidia.com/cuda.driver.minor":   "[0-9]+",
		"nvidia.com/cuda.driver.rev":     "[0-9]*",
		"nvidia.com/cuda.runtime.major":  "[0-9]+",
		"nvidia.com/cuda.runtime.minor":  "[0-9]+",
		"nvidia.com/gpu.machine":         ".*",
		"nvidia.com/gpu.count":           "[0-9]+",
		"nvidia.com/gpu.replicas":        "[0-9]+",
		"nvidia.com/gpu.product":         "[A-Za-z_-]+",
		"nvidia.com/gpu.memory":          "[0-9]+",
		"nvidia.com/gpu.family":          "[a-z]+",
		"nvidia.com/mig.capable":         "[true|false]",
		"nvidia.com/gpu.compute.major":   "[0-9]+",
		"nvidia.com/gpu.compute.minor":   "[0-9]+",
		"nvidia.com/sharing.mps.enabled": "[true|false]",
	}

	Context("When deploying GFD", Ordered, func() {
		// helm-chart is required
		if *HelmChart == "" {
			Fail("No helm-chart for GPU-Feature-Discovery specified")
		}

		// Init global suite vars vars
		var (
			crds      []*apiextensionsv1.CustomResourceDefinition
			extClient *extclient.Clientset
			nfdClient *nfdclient.Clientset

			helmClient      helm.Client
			chartSpec       helm.ChartSpec
			helmReleaseName string
			kubeconfig      []byte
		)

		values := helmValues.Options{
			Values: []string{
				fmt.Sprintf("image.repository=%s", *ImageRepo),
				fmt.Sprintf("image.tag=%s", *ImageTag),
				fmt.Sprintf("image.pullPolicy=%s", *ImagePullPolicy),
				"gfd.enabled=true",
				"devicePlugin.enabled=false",
			},
		}
		// checkNodeFeatureObject is a helper function to check if NodeFeature object was created
		checkNodeFeatureObject := func(ctx context.Context, name string) bool {
			gfdNodeFeature := fmt.Sprintf("nvidia-features-for-%s", name)
			_, err := nfdClient.NfdV1alpha1().NodeFeatures(f.Namespace.Name).Get(ctx, gfdNodeFeature, metav1.GetOptions{})
			return err == nil
		}

		BeforeAll(func(ctx context.Context) {
			var err error
			// Create clients for apiextensions and our CRD api
			extClient = extclient.NewForConfigOrDie(f.ClientConfig())
			nfdClient = nfdclient.NewForConfigOrDie(f.ClientConfig())
			helmReleaseName = "gfd-e2e-test" + rand.String(5)
			kubeconfig, err = os.ReadFile(os.Getenv("KUBECONFIG"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func(ctx context.Context) {
			// reset Helm Client
			var err error
			opt := &helm.KubeConfClientOptions{
				Options: &helm.Options{
					Namespace:        f.Namespace.Name,
					RepositoryCache:  "/tmp/.helmcache",
					RepositoryConfig: "/tmp/.helmrepo",
				},
				KubeConfig: kubeconfig,
			}
			chartSpec = helm.ChartSpec{
				ReleaseName:   helmReleaseName,
				ChartName:     *HelmChart,
				Namespace:     f.Namespace.Name,
				Wait:          true,
				Timeout:       1 * time.Minute,
				ValuesOptions: values,
				CleanupOnFail: true,
			}
			helmClient, err = helm.NewClientFromKubeConf(opt)
			Expect(err).NotTo(HaveOccurred())

			_, err = helmClient.InstallChart(ctx, &chartSpec, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		// Cleanup before next test run
		AfterEach(func(ctx context.Context) {
			// Delete Helm release
			err := helmClient.UninstallReleaseByName(helmReleaseName)
			Expect(err).NotTo(HaveOccurred())
			// cleanup node
			common.CleanupNode(ctx, f.ClientSet)
			common.CleanupNodeFeatureRules(ctx, nfdClient, f.Namespace.Name)
		})

		AfterAll(func(ctx context.Context) {
			for _, crd := range crds {
				err := extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Context("and NV Driver is not installed", func() {
			It("it should create nvidia.com timestamp label", func(ctx context.Context) {
				By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				// We pick one node targeted for our NodeFeature objects
				nodes, err := common.GetNonControlPlaneNodes(ctx, f.ClientSet)
				Expect(err).NotTo(HaveOccurred())

				targetNodeName := nodes[0].Name
				Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

				By("Check node labels")
				labelChecker := map[string]k8sLabels{
					targetNodeName: {
						"nvidia.com/gfd.timestamp": "[0-9]{10}",
					}}
				e2elog.Logf("verifying labels of node %q...", targetNodeName)
				eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(labelChecker, nodes))
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

					By("Checking if node feature object is created")
					nodes, err := common.GetNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")
					Eventually(func() bool {
						return checkNodeFeatureObject(ctx, targetNodeName)
					}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "Node feature object is not created")

					By("Check node labels are created from NodeFeature object")
					labelChecker := map[string]k8sLabels{
						targetNodeName: {
							"nvidia.com/gfd.timestamp": "[0-9]{10}",
						}}
					e2elog.Logf("verifying labels of node %q...", targetNodeName)
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(labelChecker, nodes))
				})
			})
		})

		Context("and NV Driver is installed", func() {
			BeforeEach(func(ctx context.Context) {
				// Skip test if NVIDIA_DRIVER_ENABLED is not set
				if !*NVIDIA_DRIVER_ENABLED {
					Skip("NVIDIA_DRIVER_ENABLED is not set")
				}
			})
			It("it should create nvidia.com labels", func(ctx context.Context) {
				By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				// We pick one node targeted for our NodeFeature objects
				nodes, err := common.GetNonControlPlaneNodes(ctx, f.ClientSet)
				Expect(err).NotTo(HaveOccurred())

				targetNodeName := nodes[0].Name
				Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

				By("Check node labels")
				labelChecker := map[string]k8sLabels{
					targetNodeName: expectedLabelPatterns}
				e2elog.Logf("verifying labels of node %q...", targetNodeName)
				eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(labelChecker, nodes))
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

					By("Checking if node feature object is created")
					nodes, err := common.GetNonControlPlaneNodes(ctx, f.ClientSet)
					Expect(err).NotTo(HaveOccurred())

					targetNodeName := nodes[0].Name
					Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")
					Eventually(func() bool {
						return checkNodeFeatureObject(ctx, targetNodeName)
					}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "Node feature object is not created")

					By("Check node labels are created from NodeFeature object")
					checkForLabels := map[string]k8sLabels{
						targetNodeName: expectedLabelPatterns}
					e2elog.Logf("verifying labels of node %q...", targetNodeName)
					eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchLabels(checkForLabels, nodes))
				})
			})
		})
	})
})
