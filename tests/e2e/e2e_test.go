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
	"fmt"
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
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
)

func TestMain(t *testing.T) {
	suiteName := "E2E K8s Device Plugin"

	RegisterFailHandler(Fail)

	// get the package path
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)

	ctx = context.Background()
	getTestEnv()

	RunSpecs(t,
		suiteName,
	)
}

// BeforeSuite runs before the test suite
var _ = BeforeSuite(func() {
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

var _ = AfterSuite(func() {
	By("Cleaning up namespace resources")
	// Remove finalizers and force delete resourceclaims, resourceclaimtemplates, daemonsets, and pods.
	cleanupNamespaceResources(testNamespace.Name)

	By("Deleting the test namespace")
	// Delete the test namespace to remove any remaining objects.
	deleteTestNamespace()
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

	Kubeconfig = os.Getenv("KUBECONFIG")
	Expect(Kubeconfig).NotTo(BeEmpty(), "KUBECONFIG must be set")

	Timeout = time.Duration(getIntEnvVar("E2E_TIMEOUT_SECONDS", 1800)) * time.Second

	HelmChart = os.Getenv("HELM_CHART")
	Expect(HelmChart).NotTo(BeEmpty(), "HELM_CHART must be set")

	LogArtifactDir = os.Getenv("LOG_ARTIFACTS_DIR")

	ImageRepo = os.Getenv("E2E_IMAGE_REPO")
	Expect(ImageRepo).NotTo(BeEmpty(), "IMAGE_REPO must be set")

	ImageTag = os.Getenv("E2E_IMAGE_TAG")
	Expect(ImageTag).NotTo(BeEmpty(), "IMAGE_TAG must be set")

	ImagePullPolicy = os.Getenv("E2E_IMAGE_PULL_POLICY")
	Expect(ImagePullPolicy).NotTo(BeEmpty(), "E2E_IMAGE_PULL_POLICY must be set")

	CollectLogsFrom = os.Getenv("COLLECT_LOGS_FROM")

	NVIDIA_DRIVER_ENABLED = getBoolEnvVar("NVIDIA_DRIVER_ENABLED", false)

	// Get current working directory
	cwd, err = os.Getwd()
	Expect(err).NotTo(HaveOccurred())
}

// getBoolEnvVar returns the boolean value of the environment variable or the default value if not set.
func getBoolEnvVar(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

// getIntEnvVar returns the integer value of the environment variable or the default value if not set.
func getIntEnvVar(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// CreateTestingNS should be used by every test, note that we append a common prefix to the provided test name.
// Please see NewFramework instead of using this directly.
func CreateTestingNS(baseName string, c clientset.Interface, labels map[string]string) (*corev1.Namespace, error) {
	uid := randomSuffix()
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
			if apierrors.IsAlreadyExists(err) {
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
//
//nolint:unused
func eventuallyNonControlPlaneNodes(ctx context.Context, cli clientset.Interface) AsyncAssertion {
	return Eventually(func(g Gomega, ctx context.Context) ([]corev1.Node, error) {
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

// TODO remove nolint when golangci-lint is able to cope with generics
//
//nolint:unused
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

// jobIsCompleted checks if a job is completed
//
//nolint:unused
func jobIsCompleted(ctx context.Context, cli clientset.Interface, namespace, podName string) bool {
	pod, err := cli.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	// Check if the pod's phase is Succeeded.
	if pod.Status.Phase == "Succeeded" {
		return true
	}
	return false
}

// randomSuffix provides a random sequence to append to pods,services,rcs.
//
//nolint:unused
func randomSuffix() string {
	return strconv.Itoa(rand.Intn(10000))
}

// getNonControlPlaneNodes gets the nodes that are not tainted for exclusive control-plane usage
//
//nolint:unused
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
//
//nolint:unused
func taintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

// getNode returns the node object from the list of nodes
//
//nolint:unused
func getNode(nodes []corev1.Node, nodeName string) corev1.Node {
	for _, node := range nodes {
		if node.Name == nodeName {
			return node
		}
	}
	return corev1.Node{}
}

// CreateOrUpdateJobsFromFile creates or updates jobs from a file
func CreateOrUpdateJobsFromFile(ctx context.Context, cli clientset.Interface, filename, namespace string) ([]string, error) {
	jobs, err := newJobFromfile(filepath.Join(packagePath, "data", filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create Job from file: %w", err)
	}

	names := make([]string, len(jobs))
	for i, job := range jobs {
		job.Namespace = namespace

		names[i] = job.Name

		// create or update the job
		_, err := cli.BatchV1().Jobs(namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = cli.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
				if err != nil {
					return nil, fmt.Errorf("failed to create Job %s: %w", job.Name, err)
				}
			} else {
				return nil, fmt.Errorf("failed to get Job %s: %w", job.Name, err)
			}
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

	// TODO: find out a nicer way to decode multiple api objects from a single
	// file (K8s must have that somewhere)
	split := bytes.Split(data, []byte("---"))
	objs := []apiruntime.Object{}

	for _, slice := range split {
		if len(slice) == 0 {
			continue
		}
		obj, _, err := decoder.Decode(slice, nil, nil)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, err
}
