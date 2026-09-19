/**
# Copyright 2024 NVIDIA CORPORATION
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

package helm_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/logger"

	"github.com/gruntwork-io/terratest/modules/k8s"
)

func TestDevicePluginDaemonsetTemplateRenderedDeployment(t *testing.T) {
	// Path to the helm chart we will test
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	releaseName := "nvidia-device-plugin"
	require.NoError(t, err)

	// Since we aren't deploying any resources, there is no need to setup kubectl authentication or helm home.

	testCases := []struct {
		description string
		options     map[string]string
		// TODO: We should find a better way to define the expected
		expectedContainer v1.Container
	}{
		{
			description: "no options",
			expectedContainer: v1.Container{
				SecurityContext: &v1.SecurityContext{
					AllowPrivilegeEscalation: ptr(false),
					Capabilities: &v1.Capabilities{
						Drop: []v1.Capability{"ALL"},
					},
				},
			},
		},
		{
			description: "set compatWithCPUManager",
			options: map[string]string{
				"compatWithCPUManager": "true",
			},
			expectedContainer: v1.Container{
				SecurityContext: &v1.SecurityContext{
					Privileged: ptr(true),
				},
			},
		},
		{
			description: "set mig-strategy to single",
			options: map[string]string{
				"migStrategy": "single",
			},
			expectedContainer: v1.Container{
				SecurityContext: &v1.SecurityContext{
					Capabilities: &v1.Capabilities{
						Add: []v1.Capability{"SYS_ADMIN"},
					},
				},
			},
		},
		{
			description: "set device-list-strategy to volume-mounts",
			options: map[string]string{
				"deviceListStrategy": "volume-mounts",
			},
			expectedContainer: v1.Container{
				SecurityContext: &v1.SecurityContext{
					Capabilities: &v1.Capabilities{
						Add: []v1.Capability{"SYS_ADMIN"},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Set up the namespace; confirm that the template renders the expected value for the namespace.
			namespaceName := fmt.Sprintf("k8s-device-plugin-test-%d", i)

			options := &helm.Options{
				SetValues:      tc.options,
				KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
				Logger:         logger.Discard,
			}

			// Run RenderTemplate to render the template and capture the output. Note that we use the version without `E`, since
			// we want to assert that the template renders without any errors.
			// Additionally, although we know there is only one yaml file in the template, we deliberately path a templateFiles
			// arg to demonstrate how to select individual templates to render.
			output := helm.RenderTemplate(t, options, helmChartPath, releaseName, []string{"templates/daemonset-device-plugin.yml"})

			// Now we use kubernetes/client-go library to render the template output into the Deployment struct. This will
			// ensure the Deployment resource is rendered correctly.
			var deployment appsv1.Deployment
			helm.UnmarshalK8SYaml(t, output, &deployment)

			require.Equal(t, namespaceName, deployment.Namespace)
			require.Len(t, deployment.Spec.Template.Spec.Containers, 1)

			devicePluginContainer := deployment.Spec.Template.Spec.Containers[0]
			require.EqualValues(t, tc.expectedContainer.SecurityContext, devicePluginContainer.SecurityContext)
		})
	}
}

func TestDevicePluginNamespaceValidation(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description       string
		releaseNamespace  string
		namespaceOverride string
		expectedNamespace string
	}{
		{
			description:       "default release without override",
			releaseNamespace:  "default",
			expectedNamespace: "default",
		},
		{
			description:       "non-default release without override",
			releaseNamespace:  "gpu-system",
			expectedNamespace: "gpu-system",
		},
		{
			description:       "override default release with non-default namespace",
			releaseNamespace:  "default",
			namespaceOverride: "gpu-system",
			expectedNamespace: "gpu-system",
		},
		{
			description:       "override non-default release with default namespace",
			releaseNamespace:  "gpu-system",
			namespaceOverride: "default",
			expectedNamespace: "default",
		},
		{
			description:       "override default release with default namespace",
			releaseNamespace:  "default",
			namespaceOverride: "default",
			expectedNamespace: "default",
		},
		{
			description:       "override non-default release with another namespace",
			releaseNamespace:  "gpu-system",
			namespaceOverride: "device-plugin",
			expectedNamespace: "device-plugin",
		},
	}

	for _, tc := range testCases {
		for _, allowDefaultNamespace := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/allowDefaultNamespace=%t", tc.description, allowDefaultNamespace), func(t *testing.T) {
				options := &helm.Options{
					SetValues: map[string]string{
						"namespaceOverride":     tc.namespaceOverride,
						"allowDefaultNamespace": fmt.Sprint(allowDefaultNamespace),
					},
					KubectlOptions: k8s.NewKubectlOptions("", "", tc.releaseNamespace),
					Logger:         logger.Discard,
				}

				output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
				if tc.expectedNamespace == "default" && !allowDefaultNamespace {
					require.ErrorContains(t, err, "Running in the 'default' namespace is not recommended.")
					require.ErrorContains(t, err, "Set 'allowDefaultNamespace=true' to bypass this error.")
					return
				}
				require.NoError(t, err)

				var daemonSet appsv1.DaemonSet
				helm.UnmarshalK8SYaml(t, output, &daemonSet)
				require.Equal(t, tc.expectedNamespace, daemonSet.Namespace)
			})
		}
	}
}

// prt returns a reference to whatever type is passed into it
func ptr[T any](x T) *T {
	return &x
}
