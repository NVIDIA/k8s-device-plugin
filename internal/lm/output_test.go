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
			description: "pod owned by DaemonSet returns two owner refs",
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
			expectedOwnerRef: 2,
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

			if tc.expectedOwnerRef == 2 {
				// Verify the DaemonSet owner ref is controller
				require.Equal(t, "DaemonSet", ownerRefs[0].Kind)
				require.NotNil(t, ownerRefs[0].Controller)
				require.True(t, *ownerRefs[0].Controller)
				require.Equal(t, tc.pod.OwnerReferences[0].Name, ownerRefs[0].Name)
				require.Equal(t, tc.pod.OwnerReferences[0].UID, ownerRefs[0].UID)

				// Verify the Pod owner ref
				require.Equal(t, "Pod", ownerRefs[1].Kind)
				require.Equal(t, tc.pod.Name, ownerRefs[1].Name)
				require.Equal(t, tc.pod.UID, ownerRefs[1].UID)
			}
		})
	}
}
