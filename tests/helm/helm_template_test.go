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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

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

func TestRBACTemplatesNonOpenShift(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-non-openshift"
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// Without GFD or OpenShift, no roles or bindings should render
	_, err = helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"})
	require.Error(t, err, "role.yml should not render without GFD or OpenShift")

	_, err = helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"})
	require.Error(t, err, "role-binding.yml should not render without GFD or OpenShift")
}

func TestRBACTemplatesNonOpenShiftWithGFD(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-non-openshift-gfd"
	options := &helm.Options{
		SetValues: map[string]string{
			"gfd.enabled": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// With GFD but no OpenShift: only ClusterRole (no SCC rule)
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"})
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 1, "expected only ClusterRole")

	var clusterRole rbacv1.ClusterRole
	helm.UnmarshalK8SYaml(t, roleDocs[0], &clusterRole)
	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			require.NotEqual(t, "security.openshift.io", group, "SCC rule should not be present on non-OpenShift")
		}
	}

	// With GFD: ClusterRole should include NFD and pod rules
	var hasNFD, hasPods bool
	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			if group == "nfd.k8s-sigs.io" {
				hasNFD = true
			}
		}
		for _, res := range rule.Resources {
			if res == "pods" {
				hasPods = true
			}
		}
	}
	require.True(t, hasNFD, "ClusterRole should include NFD rules with GFD enabled")
	require.True(t, hasPods, "ClusterRole should include pod rules with GFD enabled")

	// Only ClusterRoleBinding, no RoleBinding
	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"})
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 1, "expected only ClusterRoleBinding")

	var crb rbacv1.ClusterRoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[0], &crb)
	require.Equal(t, "ClusterRoleBinding", crb.Kind)
}

func TestRBACTemplatesNonOpenShiftWithConfigMap(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-non-openshift-configmap"
	options := &helm.Options{
		SetValues: map[string]string{
			"config.default": "default",
			"config.map.default": "version: v1\nflags:\n  migStrategy: none",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// With ConfigMap but no GFD: ClusterRole should exist with only node rules
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"})
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 1, "expected ClusterRole for config-manager")

	var clusterRole rbacv1.ClusterRole
	helm.UnmarshalK8SYaml(t, roleDocs[0], &clusterRole)

	require.Len(t, clusterRole.Rules, 1, "expected only node rules without GFD")
	require.Contains(t, clusterRole.Rules[0].Resources, "nodes")

	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			require.NotEqual(t, "nfd.k8s-sigs.io", group, "NFD rules should not be present without GFD")
		}
	}

	// ClusterRoleBinding should exist
	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"})
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 1, "expected only ClusterRoleBinding")

	var crb rbacv1.ClusterRoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[0], &crb)
	require.Equal(t, "ClusterRoleBinding", crb.Kind)
}

func TestRBACTemplatesOpenShift(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-openshift"
	apiVersions := "--api-versions=security.openshift.io/v1/SecurityContextConstraints"
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// Without GFD: only namespaced Role with SCC rule (no ClusterRole)
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"}, apiVersions)
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 1, "expected only namespaced Role")

	var role rbacv1.Role
	helm.UnmarshalK8SYaml(t, roleDocs[0], &role)
	require.Equal(t, "Role", role.Kind)
	require.Equal(t, namespaceName, role.Namespace)
	requireHasSCCRule(t, role.Rules)

	// Only namespaced RoleBinding (no ClusterRoleBinding)
	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"}, apiVersions)
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 1, "expected only namespaced RoleBinding")

	var rb rbacv1.RoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[0], &rb)
	require.Equal(t, "RoleBinding", rb.Kind)
	require.Equal(t, namespaceName, rb.Namespace)
	require.Equal(t, "Role", rb.RoleRef.Kind)
}

func TestRBACTemplatesOpenShiftWithGFD(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-openshift-gfd"
	apiVersions := "--api-versions=security.openshift.io/v1/SecurityContextConstraints"
	options := &helm.Options{
		SetValues: map[string]string{
			"gfd.enabled": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// With GFD + OpenShift: ClusterRole (node/NFD rules) + namespaced Role (SCC rule)
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"}, apiVersions)
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 2, "expected ClusterRole + namespaced Role")

	var clusterRole rbacv1.ClusterRole
	helm.UnmarshalK8SYaml(t, roleDocs[0], &clusterRole)
	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			require.NotEqual(t, "security.openshift.io", group, "SCC rule should not be in ClusterRole")
		}
	}

	var nfdRule *rbacv1.PolicyRule
	for i, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			if group == "nfd.k8s-sigs.io" {
				nfdRule = &clusterRole.Rules[i]
			}
		}
	}
	require.NotNil(t, nfdRule, "ClusterRole should include nfd.k8s-sigs.io rules when gfd is enabled")
	require.Contains(t, nfdRule.Resources, "nodefeatures")
	for _, verb := range []string{"get", "list", "watch", "create", "update"} {
		require.Contains(t, nfdRule.Verbs, verb)
	}

	var role rbacv1.Role
	helm.UnmarshalK8SYaml(t, roleDocs[1], &role)
	require.Equal(t, "Role", role.Kind)
	requireHasSCCRule(t, role.Rules)

	// ClusterRoleBinding + namespaced RoleBinding
	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"}, apiVersions)
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 2, "expected ClusterRoleBinding + namespaced RoleBinding")

	var crb rbacv1.ClusterRoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[0], &crb)
	require.Equal(t, "ClusterRoleBinding", crb.Kind)

	var rb rbacv1.RoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[1], &rb)
	require.Equal(t, "RoleBinding", rb.Kind)
	require.Equal(t, namespaceName, rb.Namespace)
	require.Equal(t, "Role", rb.RoleRef.Kind)
	require.Len(t, rb.Subjects, 2, "RoleBinding should include device plugin and NFD worker service accounts")
	var saNames []string
	for _, s := range rb.Subjects {
		saNames = append(saNames, s.Name)
	}
	require.Contains(t, saNames, "nvidia-device-plugin-node-feature-discovery-worker")
}

func splitYAMLDocuments(output string) []string {
	parts := strings.Split(output, "---")
	var docs []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			docs = append(docs, trimmed)
		}
	}
	return docs
}

func requireHasSCCRule(t *testing.T, rules []rbacv1.PolicyRule) {
	t.Helper()
	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			if group == "security.openshift.io" {
				require.Contains(t, rule.Resources, "securitycontextconstraints")
				require.Contains(t, rule.ResourceNames, "privileged")
				require.Contains(t, rule.Verbs, "use")
				return
			}
		}
	}
	t.Fatal("expected SCC rule with apiGroup security.openshift.io not found")
}

// prt returns a reference to whatever type is passed into it
func ptr[T any](x T) *T {
	return &x
}
