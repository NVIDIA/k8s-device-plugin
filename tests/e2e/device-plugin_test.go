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

	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/common"
	"github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework"
	e2elog "github.com/NVIDIA/k8s-device-plugin/tests/e2e/framework/logs"
)

// Actual test suite
var _ = NVDescribe("GPU Device Plugin", func() {
	f := framework.NewFramework("k8s-device-plugin")

	Context("When deploying k8s-device-plugin", Ordered, func() {
		// helm-chart is required
		if *HelmChart == "" {
			Fail("No helm-chart for k8s-device-plugin specified")
		}

		// Init global suite vars vars
		var (
			crds      []*apiextensionsv1.CustomResourceDefinition
			extClient *extclient.Clientset

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
				"devicePlugin.enabled=true",
				// We need to make affinity is none if not deploying NFD/GFD
				// test will fail if not run on a GPU node
				"affinity=",
			},
		}

		BeforeAll(func(ctx context.Context) {
			var err error
			// Create clients for apiextensions and our CRD api
			extClient = extclient.NewForConfigOrDie(f.ClientConfig())
			helmReleaseName = "nvdp-e2e-test" + rand.String(5)
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
		})

		AfterAll(func(ctx context.Context) {
			for _, crd := range crds {
				err := extClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			}
		})

		Context("and NV Driver is installed", func() {
			It("it should create nvidia.com/gpu resource", func(ctx context.Context) {
				By("Getting node objects")
				nodeList, err := f.ClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(nodeList.Items)).ToNot(BeZero())

				// We pick one node
				nodes, err := common.GetNonControlPlaneNodes(ctx, f.ClientSet)
				Expect(err).NotTo(HaveOccurred())

				targetNodeName := nodes[0].Name
				Expect(targetNodeName).ToNot(BeEmpty(), "No suitable worker node found")

				By("Check node capacity")
				capacityChecker := map[string]k8sLabels{
					targetNodeName: {
						"nvidia.com/gpu": "^[1-9]$",
					}}
				e2elog.Logf("verifying capacity of node %q...", targetNodeName)
				eventuallyNonControlPlaneNodes(ctx, f.ClientSet).Should(MatchCapacity(capacityChecker, nodes))
			})
			It("it should run GPU jobs", func(ctx context.Context) {
				By("Creating GPU job")
				job := common.GPUJob.DeepCopy()
				job.Namespace = f.Namespace.Name
				_, err := f.ClientSet.BatchV1().Jobs(f.Namespace.Name).Create(ctx, job, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for job to complete")

				Eventually(func() error {
					job, err := f.ClientSet.BatchV1().Jobs(f.Namespace.Name).Get(ctx, job.Name, metav1.GetOptions{})
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
				}, 5*time.Minute, 5*time.Second).Should(BeNil())
			})
		})
	})
})
