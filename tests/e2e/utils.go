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
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

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
