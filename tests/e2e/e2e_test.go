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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	helm "github.com/mittwald/go-helm-client"
	nfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientset "k8s.io/client-go/kubernetes"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// DefaultNamespaceDeletionTimeout is timeout duration for waiting for a namespace deletion.
	DefaultNamespaceDeletionTimeout = 10 * time.Minute

	// PollInterval is how often to Poll pods, nodes and claims.
	PollInterval = 2 * time.Second
)

var (
	Kubeconfig            string
	Timeout               time.Duration
	HelmChart             string
	LogArtifactDir        string
	ImageRepo             string
	ImageTag              string
	ImagePullPolicy       string
	CollectLogsFrom       string
	cwd                   string
	NVIDIA_DRIVER_ENABLED bool

	// k8s clients
	clientConfig *rest.Config
	clientSet    clientset.Interface
	extClient    *extclient.Clientset
	nfdClient    *nfdclient.Clientset

	testNamespace *corev1.Namespace // Every test has at least one namespace unless creation is skipped

	// Helm
	helmClient      helm.Client
	helmLogFile     *os.File
	helmArtifactDir string
	helmLogger      *log.Logger
	helmReleaseName string

	ctx         context.Context
	packagePath string
	projectRoot string
)

func TestMain(t *testing.T) {
	suiteName := "E2E K8s Device Plugin"

	RegisterFailHandler(Fail)

	// get the package path
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)
	projectRoot = filepath.Join(packagePath, "..", "..")

	ctx = context.Background()
	getTestEnv()

	// Log random seed for reproducibility
	GinkgoWriter.Printf("Random seed: %d\n", GinkgoRandomSeed())

	RunSpecs(t,
		suiteName,
		Label("e2e"),
	)
}

// BeforeSuite runs before the test suite
var _ = BeforeSuite(func(ctx SpecContext) {
	var err error

	cwd, err = os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	// Get k8s clients
	getK8sClients()

	// Create clients for apiextensions and our CRD api
	extClient = extclient.NewForConfigOrDie(clientConfig)

	// Create a namespace for the test
	testNamespace, err = CreateTestingNS("k8s-device-plugin-e2e-test", clientSet, nil)
	Expect(err).NotTo(HaveOccurred())

	// Get Helm client
	helmReleaseName = "k8s-device-plugin-e2e-test" + rand.String(5)
	getHelmClient()
})

var _ = AfterSuite(func(ctx SpecContext) {
	By("Cleaning up namespace resources")
	cleanupNamespaceResources(testNamespace.Name)

	By("Deleting the test namespace")
	deleteTestNamespace()
})

// Add ReportAfterSuite for logging test summary and random seed
var _ = ReportAfterSuite("", func(report Report) {
	// Log test summary
	failedCount := 0
	for _, specReport := range report.SpecReports {
		if specReport.Failed() {
			failedCount++
		}
	}

	GinkgoWriter.Printf("\nTest Summary:\n")
	GinkgoWriter.Printf("  Total Specs: %d\n", len(report.SpecReports))
	GinkgoWriter.Printf("  Random Seed: %d\n", report.SuiteConfig.RandomSeed)
	GinkgoWriter.Printf("  Failed: %d\n", failedCount)
	GinkgoWriter.Printf("  Duration: %.2fs\n", report.RunTime.Seconds())
})

// getK8sClients creates the k8s clients
func getK8sClients() {
	var err error

	// get config from kubeconfig
	c, err := clientcmd.LoadFromFile(Kubeconfig)
	Expect(err).NotTo(HaveOccurred())

	// get client config
	clientConfig, err = clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{}).ClientConfig()
	Expect(err).NotTo(HaveOccurred())

	clientSet, err = clientset.NewForConfig(clientConfig)
	Expect(err).NotTo(HaveOccurred())

	// Create clients for apiextensions and our CRD api
	nfdClient = nfdclient.NewForConfigOrDie(clientConfig)
}

// getHelmClient creates a new Helm client
func getHelmClient() {
	var err error

	// Set Helm log file
	helmArtifactDir = filepath.Join(LogArtifactDir, "helm")

	// Create a Helm client
	err = os.MkdirAll(helmArtifactDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	helmLogFile, err = os.OpenFile(filepath.Join(LogArtifactDir, "helm_logs"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	Expect(err).NotTo(HaveOccurred())

	helmLogger = log.New(helmLogFile, fmt.Sprintf("%s\t", testNamespace.Name), log.Ldate|log.Ltime)

	helmRestConf := &helm.RestConfClientOptions{
		Options: &helm.Options{
			Namespace:        testNamespace.Name,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			DebugLog:         helmLogger.Printf,
		},
		RestConfig: clientConfig,
	}

	helmClient, err = helm.NewClientFromRestConf(helmRestConf)
	Expect(err).NotTo(HaveOccurred())
}

// getTestEnv gets the test environment variables
func getTestEnv() {
	defer GinkgoRecover()
	var err error

	Kubeconfig = getRequiredEnvvar[string]("KUBECONFIG")

	Timeout = time.Duration(getEnvVarOrDefault("E2E_TIMEOUT_SECONDS", 1800)) * time.Second

	HelmChart = getRequiredEnvvar[string]("HELM_CHART")

	LogArtifactDir = getEnvVarOrDefault("LOG_ARTIFACTS_DIR", "e2e_logs")

	ImageRepo = getRequiredEnvvar[string]("E2E_IMAGE_REPO")

	ImageTag = getRequiredEnvvar[string]("E2E_IMAGE_TAG")

	ImagePullPolicy = getRequiredEnvvar[string]("E2E_IMAGE_PULL_POLICY")

	CollectLogsFrom = getEnvVarOrDefault("COLLECT_LOGS_FROM", "")

	NVIDIA_DRIVER_ENABLED = getEnvVarOrDefault("NVIDIA_DRIVER_ENABLED", false)

	// Get current working directory
	cwd, err = os.Getwd()
	Expect(err).NotTo(HaveOccurred())
}

// CreateTestingNS should be used by every test, note that we append a common prefix to the provided test name.
// Please see NewFramework instead of using this directly.
func CreateTestingNS(baseName string, c clientset.Interface, labels map[string]string) (*corev1.Namespace, error) {
	uid := rand.String(5)
	if labels == nil {
		labels = map[string]string{}
	}
	labels["e2e-run"] = uid

	// We don't use ObjectMeta.GenerateName feature, as in case of API call
	// failure we don't know whether the namespace was created and what is its
	// name.
	name := fmt.Sprintf("%v-%v", baseName, uid)

	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "",
			Labels:    labels,
		},
		Status: corev1.NamespaceStatus{},
	}
	// Be robust about making the namespace creation call.
	var got *corev1.Namespace
	if err := wait.PollUntilContextTimeout(ctx, PollInterval, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		var err error
		got, err = c.CoreV1().Namespaces().Create(ctx, namespaceObj, metav1.CreateOptions{})
		if err != nil {
			if k8serrors.IsAlreadyExists(err) {
				// regenerate on conflict
				namespaceObj.Name = fmt.Sprintf("%v-%v", baseName, uid)
			}
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}

	return got, nil
}

type k8sLabels map[string]string

// eventuallyNonControlPlaneNodes is a helper for asserting node properties
func eventuallyNonControlPlaneNodes(ctx context.Context, cli clientset.Interface) AsyncAssertion {
	return Eventually(func(g Gomega) ([]corev1.Node, error) {
		return getNonControlPlaneNodes(ctx, cli)
	}).WithPolling(1 * time.Second).WithTimeout(1 * time.Minute).WithContext(ctx)
}

// MatchLabels returns a specialized Gomega matcher for checking if a list of
// nodes are labeled as expected.
func MatchLabels(expectedNew map[string]k8sLabels, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	return &nodeListPropertyRegexpMatcher[k8sLabels]{
		propertyName: "labels",
		expected:     expectedNew,
		oldNodes:     oldNodes,
	}
}

// MatchCapacity returns a specialized Gomega matcher for checking if a list of
// nodes are configured as expected.
func MatchCapacity(expectedNew map[string]k8sLabels, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	return &nodeListPropertyRegexpMatcher[k8sLabels]{
		propertyName: "capacity",
		expected:     expectedNew,
		oldNodes:     oldNodes,
	}
}

// nodeListPropertyRegexpMatcher is a generic Gomega matcher for asserting one property a group of nodes.
type nodeListPropertyRegexpMatcher[T any] struct {
	expected map[string]k8sLabels
	oldNodes []corev1.Node

	propertyName string
	node         *corev1.Node //nolint:unused
	missing      []string     //nolint:unused
	invalidValue []string     //nolint:unused
}

// Match method of the GomegaMatcher interface.
func (m *nodeListPropertyRegexpMatcher[T]) Match(actual interface{}) (bool, error) {
	nodes, ok := actual.([]corev1.Node)
	if !ok {
		return false, fmt.Errorf("expected []corev1.Node, got: %T", actual)
	}

	switch m.propertyName {
	case "labels":
		return m.matchLabels(nodes), nil
	case "capacity":
		return m.matchCapacity(nodes), nil
	default:
		return true, nil
	}

}

func (m *nodeListPropertyRegexpMatcher[T]) matchLabels(nodes []corev1.Node) bool {
	targetNode := corev1.Node{}
	for _, node := range nodes {
		_, ok := m.expected[node.Name]
		if !ok {
			continue
		}
		targetNode = node
		break
	}

	m.node = &targetNode

	for labelKey, labelValue := range m.expected[targetNode.Name] {
		// missing key
		if _, ok := targetNode.Labels[labelKey]; !ok {
			m.missing = append(m.missing, labelKey)
			continue
		}
		// invalid value
		regexMatcher := regexp.MustCompile(labelValue)
		if !regexMatcher.MatchString(targetNode.Labels[labelKey]) {
			m.invalidValue = append(m.invalidValue, fmt.Sprintf("%s: %s", labelKey, targetNode.Labels[labelKey]))
			return false
		}
	}

	return true
}

func (m *nodeListPropertyRegexpMatcher[T]) matchCapacity(nodes []corev1.Node) bool {
	targetNode := corev1.Node{}
	for _, node := range nodes {
		_, ok := m.expected[node.Name]
		if !ok {
			continue
		}
		targetNode = node
		break
	}

	m.node = &targetNode

	for labelKey, labelValue := range m.expected[targetNode.Name] {
		// missing key
		rn := corev1.ResourceName(labelKey)
		if _, ok := targetNode.Status.Capacity[rn]; !ok {
			m.missing = append(m.missing, labelKey)
			continue
		}
		// invalid value
		capacity := targetNode.Status.Capacity[rn]
		regexMatcher := regexp.MustCompile(labelValue)
		if !regexMatcher.MatchString(capacity.String()) {
			m.invalidValue = append(m.invalidValue, fmt.Sprintf("%s: %s", labelKey, capacity.String()))
			return false
		}
	}

	return true
}

// FailureMessage method of the GomegaMatcher interface.
func (m *nodeListPropertyRegexpMatcher[T]) FailureMessage(actual interface{}) string {
	return m.message()
}

// NegatedFailureMessage method of the GomegaMatcher interface.
func (m *nodeListPropertyRegexpMatcher[T]) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Node %q matched unexpectedly", m.node.Name)
}

// message returns the failure message for the node list property regex matcher
func (m *nodeListPropertyRegexpMatcher[T]) message() string {
	msg := fmt.Sprintf("Node %q %s did not match:", m.node.Name, m.propertyName)
	if len(m.missing) > 0 {
		msg += fmt.Sprintf("\n  missing:\n    %s", strings.Join(m.missing, "\n    "))
	}
	if len(m.invalidValue) > 0 {
		msg += fmt.Sprintf("\n  invalid value:\n    %s", strings.Join(m.invalidValue, "\n    "))
	}
	return msg
}

// getNonControlPlaneNodes gets the nodes that are not tainted for exclusive control-plane usage
func getNonControlPlaneNodes(ctx context.Context, cli clientset.Interface) ([]corev1.Node, error) {
	nodeList, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("no nodes found in the cluster")
	}

	controlPlaneTaint := corev1.Taint{
		Effect: corev1.TaintEffectNoSchedule,
		Key:    "node-role.kubernetes.io/control-plane",
	}
	out := []corev1.Node{}
	for _, node := range nodeList.Items {
		if !taintExists(node.Spec.Taints, &controlPlaneTaint) {
			out = append(out, node)
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no non-control-plane nodes found in the cluster")
	}
	return out, nil
}

// taintExists checks if the given taint exists in list of taints. Returns true if exists false otherwise.
func taintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

// CreateOrUpdateJobsFromFile creates or updates jobs from a file
func CreateOrUpdateJobsFromFile(ctx context.Context, cli clientset.Interface, namespace string, filename string) ([]string, error) {
	jobs, err := newJobFromfile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create Job from file: %w", err)
	}

	names := make([]string, len(jobs))
	for i, job := range jobs {
		job.Namespace = namespace

		names[i] = job.Name

		// create or update the job
		_, err = cli.BatchV1().Jobs(namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if !k8serrors.IsNotFound(err) {
			// update the job
			_, err = cli.BatchV1().Jobs(namespace).Update(ctx, job, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update job: %w", err)
			}
			continue
		}
		// create the job
		_, err = cli.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create job: %w", err)
		}

	}

	return names, nil
}

func newJobFromfile(path string) ([]*batchv1.Job, error) {
	objs, err := apiObjsFromFile(path, k8sscheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	jobs := make([]*batchv1.Job, len(objs))

	for i, obj := range objs {
		var ok bool
		jobs[i], ok = obj.(*batchv1.Job)
		if !ok {
			return nil, fmt.Errorf("unexpected type %t when reading %q", obj, path)
		}
	}

	return jobs, nil
}
func apiObjsFromFile(path string, decoder apiruntime.Decoder) ([]apiruntime.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Use Kubernetes' YAML decoder that properly handles multiple documents
	// separated by "---", similar to how kubectl processes multi-document YAML files
	yamlDecoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	objs := []apiruntime.Object{}

	for {
		// Decode into raw extension first
		raw := apiruntime.RawExtension{}
		if err := yamlDecoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Skip empty documents
		raw.Raw = bytes.TrimSpace(raw.Raw)
		if len(raw.Raw) == 0 {
			continue
		}

		// Now decode the actual object using the provided decoder
		obj, _, err := decoder.Decode(raw.Raw, nil, nil)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}

	return objs, nil
}

// cleanupNamespaceResources removes all resources in the specified namespace.
func cleanupNamespaceResources(namespace string) {
	err := cleanupTestPods(namespace)
	Expect(err).NotTo(HaveOccurred())

	err = cleanupHelmDeployments(namespace)
	Expect(err).NotTo(HaveOccurred())

	cleanupNode(clientSet)
	cleanupNFDObjects(nfdClient, testNamespace.Name)
	cleanupCRDs()
}

// waitForDeletion polls the provided checkFunc until a NotFound error is returned,
// confirming that the resource is deleted.
func waitForDeletion(resourceName string, checkFunc func() error) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		err := checkFunc()
		if err != nil && k8serrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%s still exists", resourceName)
	}).WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).WithContext(ctx).Should(Succeed())
	return nil
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
		if err != nil && !k8serrors.IsNotFound(err) {
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
func cleanupNode(cs clientset.Interface) {
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
		nodeName := n.Name
		Eventually(func(g Gomega) error {
			return cleanup(nodeName)
		}).WithPolling(100 * time.Millisecond).WithTimeout(500 * time.Millisecond).Should(Succeed())
	}
}

func cleanupNFDObjects(cli *nfdclient.Clientset, namespace string) {
	cleanupNodeFeatureRules(cli)
	cleanupNodeFeatures(cli, namespace)
}

// cleanupNodeFeatures deletes all NodeFeature objects in the given namespace
func cleanupNodeFeatures(cli *nfdclient.Clientset, namespace string) {
	nfs, err := cli.NfdV1alpha1().NodeFeatures(namespace).List(ctx, metav1.ListOptions{})
	if k8serrors.IsNotFound(err) {
		// Omitted error, nothing to do.
		return
	}
	Expect(err).NotTo(HaveOccurred())

	if len(nfs.Items) != 0 {
		By("[Cleanup]\tDeleting NodeFeature objects from namespace " + namespace)
		for _, nf := range nfs.Items {
			err = cli.NfdV1alpha1().NodeFeatures(namespace).Delete(ctx, nf.Name, metav1.DeleteOptions{})
			if k8serrors.IsNotFound(err) {
				// Omitted error
				continue
			}
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

// cleanupNodeFeatureRules deletes all NodeFeatureRule objects
func cleanupNodeFeatureRules(cli *nfdclient.Clientset) {
	nfrs, err := cli.NfdV1alpha1().NodeFeatureRules().List(ctx, metav1.ListOptions{})
	if k8serrors.IsNotFound(err) {
		// Omitted error, nothing to do.
		return
	}
	Expect(err).NotTo(HaveOccurred())

	if len(nfrs.Items) != 0 {
		By("[Cleanup]\tDeleting NodeFeatureRule objects from the cluster")
		for _, nfr := range nfrs.Items {
			err = cli.NfdV1alpha1().NodeFeatureRules().Delete(ctx, nfr.Name, metav1.DeleteOptions{})
			if k8serrors.IsNotFound(err) {
				// Omitted error
				continue
			}
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

// getRequiredEnvvar returns the specified envvar if set or raises an error.
func getRequiredEnvvar[T any](key string) T {
	v, err := getEnvVarAs[T](key)
	Expect(err).To(BeNil(), "required environement variable not set", key)
	return v
}

func getEnvVarAs[T any](key string) (T, error) {
	var zero T
	value := os.Getenv(key)
	if value == "" {
		return zero, errors.New("env var not set")
	}

	switch any(zero).(type) {
	case bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return zero, err
		}
		return any(v).(T), nil
	case int:
		v, err := strconv.Atoi(value)
		if err != nil {
			return zero, err
		}
		return any(v).(T), nil
	case string:
		return any(value).(T), nil
	default:
		return zero, errors.New("unsupported type")
	}
}

func getEnvVarOrDefault[T any](key string, defaultValue T) T {
	val, err := getEnvVarAs[T](key)
	if err != nil {
		return defaultValue
	}
	return val
}

// waitForDaemonSetsReady waits for DaemonSets in a namespace to be ready, optionally filtered by label selector
func waitForDaemonSetsReady(ctx context.Context, client clientset.Interface, namespace, labelSelector string) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		dsList, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}

		if len(dsList.Items) == 0 {
			return fmt.Errorf("no daemonsets found in namespace %s with selector '%s'", namespace, labelSelector)
		}

		for _, ds := range dsList.Items {
			// Skip if no pods are desired
			if ds.Status.DesiredNumberScheduled == 0 {
				continue
			}

			if ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
				return fmt.Errorf("daemonset %s/%s rollout incomplete: %d/%d pods ready",
					namespace, ds.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
			}

			if ds.Status.UpdatedNumberScheduled != ds.Status.DesiredNumberScheduled {
				return fmt.Errorf("daemonset %s/%s update incomplete: %d/%d pods updated",
					namespace, ds.Name, ds.Status.UpdatedNumberScheduled, ds.Status.DesiredNumberScheduled)
			}

			// Check generation to ensure we're looking at the latest spec
			if ds.Generation != ds.Status.ObservedGeneration {
				return fmt.Errorf("daemonset %s/%s generation mismatch: %d != %d",
					namespace, ds.Name, ds.Generation, ds.Status.ObservedGeneration)
			}
		}

		return nil
	}).WithContext(ctx).WithPolling(2 * time.Second).WithTimeout(5 * time.Minute).Should(Succeed())
	return nil
}
