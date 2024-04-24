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

// Package framework contains provider-independent helper code for
// building and running E2E tests with Ginkgo. The actual Ginkgo test
// suites gets assembled by combining this framework, the optional
// provider support code and specific tests via a separate .go file
// like Kubernetes' test/e2e.go.
package framework

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	helm "github.com/mittwald/go-helm-client"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// DefaultNamespaceDeletionTimeout is timeout duration for waiting for a namespace deletion.
	DefaultNamespaceDeletionTimeout = 5 * time.Minute
)

// Options is a struct for managing test framework options.
type Options struct {
	ClientQPS    float32
	ClientBurst  int
	GroupVersion *schema.GroupVersion
}

// Framework supports common operations used by e2e tests; it will keep a client & a namespace for you.
// Eventual goal is to merge this with integration test framework.
//
// You can configure the pod security level for your test by setting the `NamespacePodSecurityLevel`
// which will set all three of pod security admission enforce, warn and audit labels on the namespace.
// The default pod security profile is "restricted".
// Each of the labels can be overridden by using more specific NamespacePodSecurity* attributes of this
// struct.
type Framework struct {
	BaseName string

	// Set together with creating the ClientSet and the namespace.
	// Guaranteed to be unique in the cluster even when running the same
	// test multiple times in parallel.
	UniqueName string

	clientConfig *rest.Config
	ClientSet    clientset.Interface

	// Helm
	HelmClient  helm.Client
	HelmLogFile *os.File
	HelmLogger  *log.Logger

	// configuration for framework's client
	Options Options

	SkipNamespaceCreation    bool              // Whether to skip creating a namespace
	Namespace                *corev1.Namespace // Every test has at least one namespace unless creation is skipped
	NamespaceDeletionTimeout time.Duration

	namespacesToDelete []*corev1.Namespace // Some tests have more than one.
}

// NewFramework creates a test framework.
func NewFramework(baseName string) *Framework {
	f := &Framework{
		BaseName: baseName,
	}

	// The order is important here: if the extension calls ginkgo.BeforeEach
	// itself, then it can be sure that f.BeforeEach already ran when its
	// own callback gets invoked.
	ginkgo.BeforeEach(f.BeforeEach)

	return f
}

// ClientConfig an externally accessible method for reading the kube client config.
func (f *Framework) ClientConfig() *rest.Config {
	ret := rest.CopyConfig(f.clientConfig)
	// json is the least common denominator
	ret.ContentType = runtime.ContentTypeJSON
	ret.AcceptContentTypes = runtime.ContentTypeJSON
	return ret
}

// BeforeEach gets a client and makes a namespace.
func (f *Framework) BeforeEach(ctx context.Context) {
	// DeferCleanup, in contrast to AfterEach, triggers execution in
	// first-in-last-out order. This ensures that the framework instance
	// remains valid as long as possible.
	//
	// In addition, AfterEach will not be called if a test never gets here.
	ginkgo.DeferCleanup(f.AfterEach)

	ginkgo.By("Creating a kubernetes client")
	config, err := LoadConfig()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	config.QPS = f.Options.ClientQPS
	config.Burst = f.Options.ClientBurst
	if f.Options.GroupVersion != nil {
		config.GroupVersion = f.Options.GroupVersion
	}
	f.clientConfig = rest.CopyConfig(config)
	f.ClientSet, err = clientset.NewForConfig(config)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	if !f.SkipNamespaceCreation {
		ginkgo.By(fmt.Sprintf("Building a namespace with basename %s", f.BaseName))
		namespace, err := f.CreateNamespace(ctx, f.BaseName, map[string]string{
			"e2e-framework": f.BaseName,
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		f.Namespace = namespace

		f.UniqueName = f.Namespace.GetName()
	} else {
		// not guaranteed to be unique, but very likely
		f.UniqueName = fmt.Sprintf("%s-%08x", f.BaseName, rand.Int31())
	}

	// Create a Helm client
	ginkgo.By("Creating a Helm client")

	err = os.MkdirAll(filepath.Dir(TestContext.HelmLogFile), 0755)
	gomega.Expect(err).To(gomega.BeNil())

	f.HelmLogFile, err = os.OpenFile(TestContext.HelmLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	gomega.Expect(err).To(gomega.BeNil())

	f.HelmLogger = log.New(f.HelmLogFile, fmt.Sprintf("%s\t", f.UniqueName), log.Ldate|log.Ltime)
	helmRestConf := &helm.RestConfClientOptions{
		Options: &helm.Options{
			Namespace:        f.Namespace.Name,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			DebugLog:         f.HelmLogger.Printf,
		},
		RestConfig: config,
	}

	f.HelmClient, err = helm.NewClientFromRestConf(helmRestConf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

// AfterEach deletes the namespace, after reading its events.
func (f *Framework) AfterEach(ctx context.Context) {
	// This should not happen. Given ClientSet is a public field a test must have updated it!
	// Error out early before any API calls during cleanup.
	gomega.Expect(f.ClientSet).NotTo(gomega.BeNil())

	// DeleteNamespace at the very end in defer, to avoid any
	// expectation failures preventing deleting the namespace.
	defer func() {
		var nsDeletionErrors error
		// Whether to delete namespace is determined by 3 factors: delete-namespace flag, delete-namespace-on-failure flag and the test result
		// if delete-namespace set to false, namespace will always be preserved.
		// if delete-namespace is true and delete-namespace-on-failure is false, namespace will be preserved if test failed.
		if TestContext.DeleteNamespace && (TestContext.DeleteNamespaceOnFailure || !ginkgo.CurrentSpecReport().Failed()) {
			for _, ns := range f.namespacesToDelete {
				ginkgo.By(fmt.Sprintf("[Cleanup]\tDeleting testing namespace %q.", ns.Name))
				if err := f.ClientSet.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
					if !apierrors.IsNotFound(err) {
						nsDeletionErrors = errors.Join(nsDeletionErrors, fmt.Errorf("error deleting %v: %w", ns.Name, err))
					}
				}
				// remove the namespace from the list of namespaces to delete
				// so that it is not deleted again in the defer block
				f.namespacesToDelete = f.namespacesToDelete[1:]
			}
		}

		// Unsetting this is relevant for a following test that uses
		// the same instance because it might not reach f.BeforeEach
		// when some other BeforeEach skips the test first.
		f.Namespace = nil
		f.clientConfig = nil
		f.ClientSet = nil

		// if we had errors deleting, report them now.
		gomega.Expect(nsDeletionErrors).NotTo(gomega.HaveOccurred())
	}()

	// Close helm log file
	err := f.HelmLogFile.Close()
	gomega.Expect(err).To(gomega.BeNil())
}

// CreateNamespace creates a namespace for e2e testing.
func (f *Framework) CreateNamespace(ctx context.Context, baseName string, labels map[string]string) (*corev1.Namespace, error) {
	createTestingNS := TestContext.CreateTestingNS
	if createTestingNS == nil {
		createTestingNS = CreateTestingNS
	}

	if labels == nil {
		labels = make(map[string]string)
	} else {
		labelsCopy := make(map[string]string)
		for k, v := range labels {
			labelsCopy[k] = v
		}
		labels = labelsCopy
	}

	ns, err := createTestingNS(ctx, baseName, f.ClientSet, labels)

	// check ns instead of err to see if it's nil as we may
	// fail to create serviceAccount in it.
	f.AddNamespacesToDelete(ns)

	return ns, err
}

// DeleteNamespace can be used to delete a namespace
func (f *Framework) DeleteNamespace(ctx context.Context, name string) {
	defer func() {
		err := f.ClientSet.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}
		err = WaitForNamespacesDeleted(ctx, f.ClientSet, []string{name}, DefaultNamespaceDeletionTimeout)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()
}

// AddNamespacesToDelete adds one or more namespaces to be deleted when the test
// completes.
func (f *Framework) AddNamespacesToDelete(namespaces ...*corev1.Namespace) {
	for _, ns := range namespaces {
		if ns == nil {
			continue
		}
		f.namespacesToDelete = append(f.namespacesToDelete, ns)

	}
}
