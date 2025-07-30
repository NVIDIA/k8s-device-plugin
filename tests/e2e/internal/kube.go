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

package internal

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultPollInterval for Eventually checks
	DefaultPollInterval = 2 * time.Second
	// DefaultTimeout for Eventually checks
	DefaultTimeout = 5 * time.Minute
)

// WaitForDaemonSetRollout waits for a DaemonSet to complete its rollout
func WaitForDaemonSetRollout(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Check if rollout is complete
		if ds.Status.DesiredNumberScheduled == 0 {
			return fmt.Errorf("daemonset %s/%s has 0 desired pods", namespace, name)
		}

		if ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("daemonset %s/%s rollout incomplete: %d/%d pods ready",
				namespace, name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
		}

		if ds.Status.UpdatedNumberScheduled != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("daemonset %s/%s update incomplete: %d/%d pods updated",
				namespace, name, ds.Status.UpdatedNumberScheduled, ds.Status.DesiredNumberScheduled)
		}

		// Check generation to ensure we're looking at the latest spec
		if ds.Generation != ds.Status.ObservedGeneration {
			return fmt.Errorf("daemonset %s/%s generation mismatch: %d != %d",
				namespace, name, ds.Generation, ds.Status.ObservedGeneration)
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForAllDaemonSetsReady waits for all DaemonSets in a namespace to be ready
func WaitForAllDaemonSetsReady(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForDaemonSetsReady(ctx, client, namespace, "")
}

// WaitForDaemonSetsReady waits for DaemonSets in a namespace to be ready, optionally filtered by label selector
func WaitForDaemonSetsReady(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) error {
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
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForDaemonSetPodsReady waits for all pods of a DaemonSet to be ready
func WaitForDaemonSetPodsReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return fmt.Errorf("invalid selector: %v", err)
		}

		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return err
		}

		if len(pods.Items) == 0 {
			return fmt.Errorf("no pods found for daemonset %s/%s", namespace, name)
		}

		for _, pod := range pods.Items {
			if !isPodReady(&pod) {
				return fmt.Errorf("pod %s/%s is not ready", pod.Namespace, pod.Name)
			}
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForNodeLabels waits for specific labels to appear on nodes
func WaitForNodeLabels(ctx context.Context, client kubernetes.Interface, labelSelector string, expectedLabels map[string]string) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}

		if len(nodes.Items) == 0 {
			return fmt.Errorf("no nodes found with selector: %s", labelSelector)
		}

		// Check each node has the expected labels
		for _, node := range nodes.Items {
			for key, expectedValue := range expectedLabels {
				actualValue, exists := node.Labels[key]
				if !exists {
					return fmt.Errorf("node %s missing label: %s", node.Name, key)
				}
				if expectedValue != "" && actualValue != expectedValue {
					return fmt.Errorf("node %s label %s=%s, expected %s",
						node.Name, key, actualValue, expectedValue)
				}
			}
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForGFDLabels waits for GPU Feature Discovery labels on nodes
func WaitForGFDLabels(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	gfdLabels := []string{
		"nvidia.com/gfd.timestamp",
		"nvidia.com/cuda.driver.major",
		"nvidia.com/cuda.driver.minor",
		"nvidia.com/gpu.family",
		"nvidia.com/gpu.machine",
		"nvidia.com/gpu.memory",
		"nvidia.com/gpu.product",
	}

	EventuallyWithOffset(1, func(g Gomega) error {
		node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for _, label := range gfdLabels {
			if _, exists := node.Labels[label]; !exists {
				return fmt.Errorf("node %s missing GFD label: %s", nodeName, label)
			}
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForPodsRunning waits for pods matching a selector to be running
func WaitForPodsRunning(ctx context.Context, client kubernetes.Interface, namespace string, selector labels.Selector) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return err
		}

		if len(pods.Items) == 0 {
			return fmt.Errorf("no pods found matching selector: %s", selector.String())
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return fmt.Errorf("pod %s/%s is %s, not Running", pod.Namespace, pod.Name, pod.Status.Phase)
			}
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// WaitForDeploymentRollout waits for a deployment to complete its rollout
func WaitForDeploymentRollout(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	EventuallyWithOffset(1, func(g Gomega) error {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Check if the deployment is complete
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing {
				if condition.Status != corev1.ConditionTrue {
					return fmt.Errorf("deployment %s/%s is not progressing: %s", namespace, name, condition.Message)
				}
			}
			if condition.Type == appsv1.DeploymentAvailable {
				if condition.Status != corev1.ConditionTrue {
					return fmt.Errorf("deployment %s/%s is not available: %s", namespace, name, condition.Message)
				}
			}
		}

		if deployment.Status.UpdatedReplicas != *deployment.Spec.Replicas {
			return fmt.Errorf("deployment %s/%s update incomplete: %d/%d replicas updated",
				namespace, name, deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas)
		}

		if deployment.Status.ReadyReplicas != *deployment.Spec.Replicas {
			return fmt.Errorf("deployment %s/%s not ready: %d/%d replicas ready",
				namespace, name, deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
		}

		return nil
	}).WithContext(ctx).WithPolling(DefaultPollInterval).WithTimeout(DefaultTimeout).Should(Succeed())
	return nil
}

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
