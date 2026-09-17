/**
# Copyright 2026 NVIDIA CORPORATION
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package lm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetOwnerRefs(t *testing.T) {
	testCases := []struct {
		description      string
		podName          string
		namespace        string
		pod              *corev1.Pod
		expectedOwnerRef int
		expectError      bool
	}{
		{
			description:      "empty pod name returns nil",
			podName:          "",
			namespace:        "default",
			expectedOwnerRef: 0,
			expectError:      false,
		},
		{
			description: "pod owned by DaemonSet returns only DaemonSet owner ref",
			podName:     "gfd-pod",
			namespace:   "gpu-operator",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gfd-pod",
					Namespace: "gpu-operator",
					UID:       types.UID("pod-uid-123"),
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "DaemonSet",
							Name:       "nvidia-gfd",
							UID:        types.UID("ds-uid-456"),
						},
					},
				},
			},
			expectedOwnerRef: 1,
			expectError:      false,
		},
		{
			description: "pod not owned by anything returns nil",
			podName:     "standalone-pod",
			namespace:   "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "standalone-pod",
					Namespace: "default",
					UID:       types.UID("pod-uid-789"),
				},
			},
			expectedOwnerRef: 0,
			expectError:      false,
		},
		{
			description: "pod owned by ReplicaSet returns nil",
			podName:     "rs-pod",
			namespace:   "default",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rs-pod",
					Namespace: "default",
					UID:       types.UID("pod-uid-abc"),
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       "my-replicaset",
							UID:        types.UID("rs-uid-def"),
						},
					},
				},
			},
			expectedOwnerRef: 0,
			expectError:      false,
		},
		{
			description:      "pod not found returns error",
			podName:          "nonexistent-pod",
			namespace:        "default",
			pod:              nil,
			expectedOwnerRef: 0,
			expectError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var client kubernetes.Interface
			if tc.pod != nil {
				client = fake.NewClientset(tc.pod)
			} else {
				client = fake.NewClientset()
			}

			ownerRefs, err := getOwnerReferences(context.Background(), client, tc.namespace, tc.podName)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, ownerRefs, tc.expectedOwnerRef)

			if tc.expectedOwnerRef == 1 {
				require.Equal(t, "DaemonSet", ownerRefs[0].Kind)
				require.NotNil(t, ownerRefs[0].Controller)
				require.True(t, *ownerRefs[0].Controller)
				require.Equal(t, tc.pod.OwnerReferences[0].Name, ownerRefs[0].Name)
				require.Equal(t, tc.pod.OwnerReferences[0].UID, ownerRefs[0].UID)

				// Pod must not be an owner: re-adding it after a restart requires
				// delete permission on the NodeFeature and causes CrashLoopBackOff.
				for _, ref := range ownerRefs {
					require.NotEqual(t, "Pod", ref.Kind)
				}
			}
		})
	}
}

// TestGetOwnerRefsStableAcrossPodRestart verifies that two successive GFD pods
// owned by the same DaemonSet resolve to the same NodeFeature ownerRefs. That
// stability avoids OwnerReferencesPermissionEnforcement rejecting an update that
// would otherwise add a new Pod ownerRef after the previous pod was GC'd.
func TestGetOwnerRefsStableAcrossPodRestart(t *testing.T) {
	namespace := "gpu-operator"
	dsRef := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       "gpu-feature-discovery",
		UID:        types.UID("ds-uid-stable"),
	}

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gpu-feature-discovery-aaaa",
			Namespace:       namespace,
			UID:             types.UID("pod-uid-old"),
			OwnerReferences: []metav1.OwnerReference{dsRef},
		},
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gpu-feature-discovery-bbbb",
			Namespace:       namespace,
			UID:             types.UID("pod-uid-new"),
			OwnerReferences: []metav1.OwnerReference{dsRef},
		},
	}

	oldRefs, err := getOwnerReferences(context.Background(), fake.NewClientset(oldPod), namespace, oldPod.Name)
	require.NoError(t, err)
	newRefs, err := getOwnerReferences(context.Background(), fake.NewClientset(newPod), namespace, newPod.Name)
	require.NoError(t, err)

	require.Len(t, oldRefs, 1)
	require.Len(t, newRefs, 1)
	require.Equal(t, oldRefs[0].Kind, newRefs[0].Kind)
	require.Equal(t, oldRefs[0].Name, newRefs[0].Name)
	require.Equal(t, oldRefs[0].UID, newRefs[0].UID)
	require.Equal(t, oldRefs[0].APIVersion, newRefs[0].APIVersion)
	require.Equal(t, oldRefs[0].Controller, newRefs[0].Controller)
}
