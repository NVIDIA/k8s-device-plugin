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

package framework

import (
	"context"
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// CreateTestingNSFn is a func that is responsible for creating namespace used for executing e2e tests.
type CreateTestingNSFn func(ctx context.Context, baseName string, c clientset.Interface, labels map[string]string) (*corev1.Namespace, error)

// TestContextType contains test settings and global state
type TestContextType struct {
	KubeConfig               string
	KubeContext              string
	DeleteNamespace          bool
	DeleteNamespaceOnFailure bool

	HelmLogFile string

	// CreateTestingNS is responsible for creating namespace used for executing e2e tests.
	// It accepts namespace base name, which will be prepended with e2e prefix, kube client
	// and labels to be applied to a namespace.
	CreateTestingNS CreateTestingNSFn
}

// TestContext should be used by all tests to access common context data.
var TestContext = TestContextType{}

// RegisterClusterFlags registers flags specific to the cluster e2e test suite.
func RegisterClusterFlags(flags *flag.FlagSet) {
	flags.BoolVar(&TestContext.DeleteNamespace, "delete-namespace", true, "If true tests will delete namespace after completion. It is only designed to make debugging easier, DO NOT turn it off by default.")
	flags.BoolVar(&TestContext.DeleteNamespaceOnFailure, "delete-namespace-on-failure", true, "If true, framework will delete test namespace on failure. Used only during test debugging.")
	flags.StringVar(&TestContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, os.Getenv(clientcmd.RecommendedConfigPathEnvVar), "Path to kubeconfig containing embedded authinfo.")
	flags.StringVar(&TestContext.KubeContext, clientcmd.FlagContext, "", "kubeconfig context to use/override. If unset, will use value from 'current-context'")
	flags.StringVar(&TestContext.HelmLogFile, "helm-log-file", "e2e-helm", "Path to the file where helm logs will be written.")
}
