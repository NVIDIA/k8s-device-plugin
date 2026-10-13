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
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"

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

func TestComponentResourcesTemplateRendered(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	templateFiles := []string{
		"templates/daemonset-device-plugin.yml",
		"templates/daemonset-gfd.yml",
		"templates/daemonset-mps-control-daemon.yml",
	}

	testCases := []struct {
		description string
		options     map[string]string
		// An empty memory limit means the container has no resources set.
		expectedMemoryLimitByContainer map[string]string
	}{
		{
			description: "top-level resources apply to main containers only",
			options: map[string]string{
				"resources.limits.memory": "1Gi",
			},
			expectedMemoryLimitByContainer: map[string]string{
				"nvidia-device-plugin-init":     "",
				"nvidia-device-plugin-sidecar":  "",
				"nvidia-device-plugin-ctr":      "1Gi",
				"gpu-feature-discovery-init":    "",
				"gpu-feature-discovery-sidecar": "",
				"gpu-feature-discovery-ctr":     "1Gi",
				"mps-control-daemon-mounts":     "1Gi",
				"mps-control-daemon-init":       "",
				"mps-control-daemon-sidecar":    "",
				"mps-control-daemon-ctr":        "1Gi",
			},
		},
		{
			description: "values reused from a release without configManager",
			options: map[string]string{
				"configManager":           "null",
				"resources.limits.memory": "1Gi",
			},
			expectedMemoryLimitByContainer: map[string]string{
				"nvidia-device-plugin-init":     "",
				"nvidia-device-plugin-sidecar":  "",
				"nvidia-device-plugin-ctr":      "1Gi",
				"gpu-feature-discovery-init":    "",
				"gpu-feature-discovery-sidecar": "",
				"gpu-feature-discovery-ctr":     "1Gi",
				"mps-control-daemon-mounts":     "1Gi",
				"mps-control-daemon-init":       "",
				"mps-control-daemon-sidecar":    "",
				"mps-control-daemon-ctr":        "1Gi",
			},
		},
		{
			description: "component resources override top-level resources",
			options: map[string]string{
				"resources.limits.memory":               "1Gi",
				"devicePlugin.resources.limits.memory":  "3Gi",
				"gfd.resources.limits.memory":           "2Gi",
				"mps.resources.limits.memory":           "4Gi",
				"configManager.resources.limits.memory": "64Mi",
			},
			expectedMemoryLimitByContainer: map[string]string{
				"nvidia-device-plugin-init":     "64Mi",
				"nvidia-device-plugin-sidecar":  "64Mi",
				"nvidia-device-plugin-ctr":      "3Gi",
				"gpu-feature-discovery-init":    "64Mi",
				"gpu-feature-discovery-sidecar": "64Mi",
				"gpu-feature-discovery-ctr":     "2Gi",
				"mps-control-daemon-mounts":     "4Gi",
				"mps-control-daemon-init":       "64Mi",
				"mps-control-daemon-sidecar":    "64Mi",
				"mps-control-daemon-ctr":        "4Gi",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			setValues := map[string]string{
				"config.name": "external-config",
				"gfd.enabled": "true",
			}
			maps.Copy(setValues, tc.options)
			options := &helm.Options{
				SetValues:      setValues,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			memoryLimitByContainer := make(map[string]string)
			for _, templateFile := range templateFiles {
				output := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{templateFile})

				var daemonset appsv1.DaemonSet
				helm.UnmarshalK8SYaml(t, output, &daemonset)

				podSpec := daemonset.Spec.Template.Spec
				for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
					memoryLimit := ""
					if limit, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
						memoryLimit = limit.String()
					}
					memoryLimitByContainer[container.Name] = memoryLimit
				}
			}

			require.Equal(t, tc.expectedMemoryLimitByContainer, memoryLimitByContainer)
		})
	}
}

func TestDevicePluginDaemonsetNvidiaDriverCapabilities(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description                   string
		nvidiaDriverCapabilitiesJSON  string
		expectedDriverCapabilitiesEnv *v1.EnvVar
		expectSchemaRejection         bool
	}{
		{
			description:                   "default",
			expectedDriverCapabilitiesEnv: &v1.EnvVar{Name: "NVIDIA_DRIVER_CAPABILITIES", Value: "compute,utility"},
		},
		{
			description:                   "string",
			nvidiaDriverCapabilitiesJSON:  `"all"`,
			expectedDriverCapabilitiesEnv: &v1.EnvVar{Name: "NVIDIA_DRIVER_CAPABILITIES", Value: "all"},
		},
		{
			description:                  "null omits the variable",
			nvidiaDriverCapabilitiesJSON: "null",
		},
		{
			description:                  "empty string omits the variable",
			nvidiaDriverCapabilitiesJSON: `""`,
		},
		{
			description:                  "boolean is rejected",
			nvidiaDriverCapabilitiesJSON: "true",
			expectSchemaRejection:        true,
		},
		{
			description:                  "number is rejected",
			nvidiaDriverCapabilitiesJSON: "1",
			expectSchemaRejection:        true,
		},
		{
			description:                  "list is rejected",
			nvidiaDriverCapabilitiesJSON: `["compute","utility"]`,
			expectSchemaRejection:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}
			if tc.nvidiaDriverCapabilitiesJSON != "" {
				options.SetJsonValues = map[string]string{"nvidiaDriverCapabilities": tc.nvidiaDriverCapabilitiesJSON}
			}

			// values.schema.json is enforced even when only the daemonset is selected for output.
			output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
			if tc.expectSchemaRejection {
				requireSchemaRejection(t, err, "nvidiaDriverCapabilities")
				return
			}
			require.NoError(t, err)

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			require.Len(t, daemonset.Spec.Template.Spec.Containers, 1)

			var driverCapabilitiesEnv *v1.EnvVar
			for _, env := range daemonset.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "NVIDIA_DRIVER_CAPABILITIES" {
					driverCapabilitiesEnv = &env
				}
			}
			require.Equal(t, tc.expectedDriverCapabilitiesEnv, driverCapabilitiesEnv)
		})
	}
}

func TestDevicePluginDaemonsetNvidiaDevRoot(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description           string
		options               map[string]string
		expectedDevRootEnv    *v1.EnvVar
		expectSchemaRejection bool
	}{
		{
			description: "default",
		},
		{
			description:        "string",
			options:            map[string]string{"nvidiaDevRoot": "/dev-root"},
			expectedDevRootEnv: &v1.EnvVar{Name: "NVIDIA_DEV_ROOT", Value: "/dev-root"},
		},
		{
			description:           "number is rejected",
			options:               map[string]string{"nvidiaDevRoot": "1"},
			expectSchemaRejection: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				SetValues:      tc.options,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
			if tc.expectSchemaRejection {
				requireSchemaRejection(t, err, "nvidiaDevRoot")
				return
			}
			require.NoError(t, err)

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			require.Len(t, daemonset.Spec.Template.Spec.Containers, 1)

			var devRootEnv *v1.EnvVar
			for _, env := range daemonset.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "NVIDIA_DEV_ROOT" {
					devRootEnv = &env
				}
			}
			require.Equal(t, tc.expectedDevRootEnv, devRootEnv)
		})
	}
}

func TestDevicePluginDaemonsetImageTag(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	chartYAML, err := os.ReadFile(filepath.Join(helmChartPath, "Chart.yaml"))
	require.NoError(t, err)
	var chartMetadata struct {
		AppVersion string `json:"appVersion"`
	}
	require.NoError(t, yaml.Unmarshal(chartYAML, &chartMetadata))
	defaultImage := "nvcr.io/nvidia/k8s-device-plugin:v" + chartMetadata.AppVersion

	testCases := []struct {
		description   string
		options       map[string]string
		jsonOptions   map[string]string
		expectedImage string
	}{
		{
			description:   "default",
			expectedImage: defaultImage,
		},
		{
			description:   "empty tag",
			options:       map[string]string{"image.tag": ""},
			expectedImage: defaultImage,
		},
		{
			description:   "string tag",
			options:       map[string]string{"image.tag": "v0.17.0"},
			expectedImage: "nvcr.io/nvidia/k8s-device-plugin:v0.17.0",
		},
		{
			// --set parses an all-digit tag as a number.
			description:   "numeric tag",
			options:       map[string]string{"image.tag": "123"},
			expectedImage: "nvcr.io/nvidia/k8s-device-plugin:123",
		},
		{
			description:   "zero tag",
			options:       map[string]string{"image.tag": "0"},
			expectedImage: "nvcr.io/nvidia/k8s-device-plugin:0",
		},
		{
			// --set-json and values files load numbers as float64.
			description:   "large numeric tag from JSON",
			jsonOptions:   map[string]string{"image.tag": "20260101"},
			expectedImage: "nvcr.io/nvidia/k8s-device-plugin:20260101",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				SetValues:      tc.options,
				SetJsonValues:  tc.jsonOptions,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			output := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			require.Len(t, daemonset.Spec.Template.Spec.Containers, 1)
			require.Equal(t, tc.expectedImage, daemonset.Spec.Template.Spec.Containers[0].Image)
		})
	}
}

func TestComponentHostNetworkTemplateRendered(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	templateFileByComponent := map[string]string{
		"devicePlugin": "templates/daemonset-device-plugin.yml",
		"gfd":          "templates/daemonset-gfd.yml",
		"mps":          "templates/daemonset-mps-control-daemon.yml",
	}

	testCases := []struct {
		description           string
		setValue              string
		setStringValue        string
		expectedHostNetwork   bool
		expectSchemaRejection bool
	}{
		{
			description: "default",
		},
		{
			description:         "true",
			setValue:            "true",
			expectedHostNetwork: true,
		},
		{
			description:           "string is rejected",
			setStringValue:        "false",
			expectSchemaRejection: true,
		},
	}

	for component, templateFile := range templateFileByComponent {
		for _, tc := range testCases {
			t.Run(component+"/"+tc.description, func(t *testing.T) {
				hostNetworkKey := component + ".enableHostNetwork"
				options := &helm.Options{
					SetValues: map[string]string{
						"config.name": "external-config",
						"gfd.enabled": "true",
					},
					SetStrValues:   map[string]string{},
					KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
					Logger:         logger.Discard,
				}
				if tc.setValue != "" {
					options.SetValues[hostNetworkKey] = tc.setValue
				}
				if tc.setStringValue != "" {
					options.SetStrValues[hostNetworkKey] = tc.setStringValue
				}

				output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{templateFile})
				if tc.expectSchemaRejection {
					requireSchemaRejection(t, err, hostNetworkKey)
					return
				}
				require.NoError(t, err)

				var daemonset appsv1.DaemonSet
				helm.UnmarshalK8SYaml(t, output, &daemonset)
				require.Equal(t, tc.expectedHostNetwork, daemonset.Spec.Template.Spec.HostNetwork)
			})
		}
	}
}

func TestStringMapValuesTemplateRendered(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description       string
		options           map[string]string
		stringOptions     map[string]string
		jsonOptions       map[string]string
		rejectedValuePath string
		verifyDaemonSet   func(t *testing.T, daemonset appsv1.DaemonSet)
	}{
		{
			description: "config.map entry as a YAML string",
			jsonOptions: map[string]string{"config.map": `{"default": "version: v1"}`},
			verifyDaemonSet: func(t *testing.T, daemonset appsv1.DaemonSet) {
				require.Contains(t, daemonset.Spec.Template.Annotations, "checksum/config")
			},
		},
		{
			description:       "config.map entry that is not a YAML string is rejected",
			jsonOptions:       map[string]string{"config.map": `{"default": {"version": "v1"}}`},
			rejectedValuePath: "config.map.default",
		},
		{
			description:   "quoted pod annotation",
			stringOptions: map[string]string{"podAnnotations.example": "1"},
			verifyDaemonSet: func(t *testing.T, daemonset appsv1.DaemonSet) {
				require.Equal(t, map[string]string{"example": "1"}, daemonset.Spec.Template.Annotations)
			},
		},
		{
			description:       "numeric pod annotation is rejected",
			options:           map[string]string{"podAnnotations.example": "1"},
			rejectedValuePath: "podAnnotations.example",
		},
		{
			description:   "quoted node selector",
			stringOptions: map[string]string{"nodeSelector.example": "true"},
			verifyDaemonSet: func(t *testing.T, daemonset appsv1.DaemonSet) {
				require.Equal(t, map[string]string{"example": "true"}, daemonset.Spec.Template.Spec.NodeSelector)
			},
		},
		{
			description:       "boolean node selector is rejected",
			options:           map[string]string{"nodeSelector.example": "true"},
			rejectedValuePath: "nodeSelector.example",
		},
		{
			description:   "quoted selector label override",
			stringOptions: map[string]string{"selectorLabelsOverride.example": "1"},
			verifyDaemonSet: func(t *testing.T, daemonset appsv1.DaemonSet) {
				require.Equal(t, map[string]string{"example": "1"}, daemonset.Spec.Selector.MatchLabels)
			},
		},
		{
			description:       "numeric selector label override is rejected",
			options:           map[string]string{"selectorLabelsOverride.example": "1"},
			rejectedValuePath: "selectorLabelsOverride.example",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				SetValues:      tc.options,
				SetStrValues:   tc.stringOptions,
				SetJsonValues:  tc.jsonOptions,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
			if tc.rejectedValuePath != "" {
				requireSchemaRejection(t, err, tc.rejectedValuePath)
				return
			}
			require.NoError(t, err)

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			tc.verifyDaemonSet(t, daemonset)
		})
	}
}

func TestImagePullSecretsTemplateRendered(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description              string
		options                  map[string]string
		jsonOptions              map[string]string
		rejectedValuePath        string
		expectedImagePullSecrets []v1.LocalObjectReference
	}{
		{
			description:              "secret name",
			options:                  map[string]string{"imagePullSecrets[0].name": "registry-secret"},
			expectedImagePullSecrets: []v1.LocalObjectReference{{Name: "registry-secret"}},
		},
		{
			// Kubernetes accepts entries without a name and the kubelet skips them.
			description:              "empty entry",
			jsonOptions:              map[string]string{"imagePullSecrets": "[{}]"},
			expectedImagePullSecrets: []v1.LocalObjectReference{{}},
		},
		{
			description:       "bare string is rejected",
			options:           map[string]string{"imagePullSecrets[0]": "registry-secret"},
			rejectedValuePath: "imagePullSecrets.0",
		},
		{
			description:       "numeric name is rejected",
			options:           map[string]string{"imagePullSecrets[0].name": "123"},
			rejectedValuePath: "imagePullSecrets.0.name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				SetValues:      tc.options,
				SetJsonValues:  tc.jsonOptions,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
			if tc.rejectedValuePath != "" {
				requireSchemaRejection(t, err, tc.rejectedValuePath)
				return
			}
			require.NoError(t, err)

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			require.Equal(t, tc.expectedImagePullSecrets, daemonset.Spec.Template.Spec.ImagePullSecrets)
		})
	}
}

func TestDevicePluginDaemonsetAffinityTemplateRendered(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		description           string
		options               map[string]string
		expectedAffinity      bool
		expectSchemaRejection bool
	}{
		{
			description:      "default",
			expectedAffinity: true,
		},
		{
			description:      "null clears the default",
			options:          map[string]string{"affinity": "null"},
			expectedAffinity: false,
		},
		{
			// Releases installed with --set affinity= store an empty string.
			description:      "empty string clears the default",
			options:          map[string]string{"affinity": ""},
			expectedAffinity: false,
		},
		{
			description:           "other string is rejected",
			options:               map[string]string{"affinity": "gpu-nodes"},
			expectSchemaRejection: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			options := &helm.Options{
				SetValues:      tc.options,
				KubectlOptions: k8s.NewKubectlOptions("", "", "k8s-device-plugin-test"),
				Logger:         logger.Discard,
			}

			output, err := helm.RenderTemplateE(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/daemonset-device-plugin.yml"})
			if tc.expectSchemaRejection {
				requireSchemaRejection(t, err, "affinity")
				return
			}
			require.NoError(t, err)

			var daemonset appsv1.DaemonSet
			helm.UnmarshalK8SYaml(t, output, &daemonset)
			require.Equal(t, tc.expectedAffinity, daemonset.Spec.Template.Spec.Affinity != nil)
		})
	}
}

// Helm 3.18 switched JSON schema validators, so the error names a value as
// "/devicePlugin/enableHostNetwork" from then on and as "devicePlugin.enableHostNetwork" before it.
func requireSchemaRejection(t *testing.T, err error, valuePath string) {
	t.Helper()
	require.ErrorContains(t, err, "values don't meet the specifications of the schema")
	jsonPointer := "/" + strings.ReplaceAll(valuePath, ".", "/")
	require.Truef(t, strings.Contains(err.Error(), jsonPointer) || strings.Contains(err.Error(), valuePath+":"),
		"schema error does not name %q: %v", valuePath, err)
}


func TestRBACTemplatesNonOpenShift(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-non-openshift"
	options := &helm.Options{
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// Base install always renders the ClusterRole (nodes only) and ClusterRoleBinding.
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"})
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 1, "expected only ClusterRole")

	var clusterRole rbacv1.ClusterRole
	helm.UnmarshalK8SYaml(t, roleDocs[0], &clusterRole)
	require.Equal(t, "ClusterRole", clusterRole.Kind)
	require.Len(t, clusterRole.Rules, 1, "expected only node rules without GFD")
	require.Contains(t, clusterRole.Rules[0].Resources, "nodes")
	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			require.NotEqual(t, "security.openshift.io", group, "SCC rule should not be present on non-OpenShift")
			require.NotEqual(t, "nfd.k8s-sigs.io", group, "NFD rules should not be present without GFD")
		}
	}

	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"})
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 1, "expected only ClusterRoleBinding")

	var crb rbacv1.ClusterRoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[0], &crb)
	require.Equal(t, "ClusterRoleBinding", crb.Kind)
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

func TestRBACTemplatesNonOpenShiftWithTimeSlicing(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-non-openshift-timeslicing"
	options := &helm.Options{
		SetValues: map[string]string{
			"config.default":         "timeslicing",
			"config.map.timeslicing": "version: v1\nsharing:\n  timeSlicing:\n    resources:\n      - name: nvidia.com/gpu\n        replicas: 10",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	// With time-slicing ConfigMap but no GFD: ClusterRole should exist with only node rules
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

	// OpenShift base install: ClusterRole (nodes) + namespaced Role (SCC use)
	roleOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role.yml"}, apiVersions)
	roleDocs := splitYAMLDocuments(roleOutput)
	require.Len(t, roleDocs, 2, "expected ClusterRole + namespaced Role")

	var clusterRole rbacv1.ClusterRole
	helm.UnmarshalK8SYaml(t, roleDocs[0], &clusterRole)
	require.Equal(t, "ClusterRole", clusterRole.Kind)
	for _, rule := range clusterRole.Rules {
		for _, group := range rule.APIGroups {
			require.NotEqual(t, "security.openshift.io", group, "SCC rule should not be in ClusterRole")
		}
	}

	var role rbacv1.Role
	helm.UnmarshalK8SYaml(t, roleDocs[1], &role)
	require.Equal(t, "Role", role.Kind)
	require.Equal(t, namespaceName, role.Namespace)
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
	require.Len(t, rb.Subjects, 1, "RoleBinding should only include device plugin SA without GFD")
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
	for _, verb := range []string{"get", "list", "watch", "create", "update", "delete"} {
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

func TestRBACTemplatesOpenShiftGFDWithoutNFD(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	namespaceName := "rbac-test-openshift-gfd-no-nfd"
	apiVersions := "--api-versions=security.openshift.io/v1/SecurityContextConstraints"
	options := &helm.Options{
		SetValues: map[string]string{
			"gfd.enabled": "true",
			"nfd.enabled": "false",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	bindingOutput := helm.RenderTemplate(t, options, helmChartPath, "nvidia-device-plugin", []string{"templates/role-binding.yml"}, apiVersions)
	bindingDocs := splitYAMLDocuments(bindingOutput)
	require.Len(t, bindingDocs, 2, "expected ClusterRoleBinding + namespaced RoleBinding")

	var rb rbacv1.RoleBinding
	helm.UnmarshalK8SYaml(t, bindingDocs[1], &rb)
	require.Len(t, rb.Subjects, 1, "RoleBinding should only include device plugin SA when nfd.enabled=false")
	require.Equal(t, "nvidia-device-plugin-service-account", rb.Subjects[0].Name)
}

// TestRBACNFDWorkerServiceAccountMatchesSubchart renders the whole chart, including the
// NFD subchart, and asserts the SCC RoleBinding names a ServiceAccount that is actually
// created. The release name deliberately differs from the chart name, because that is
// where this chart's fullname and the subchart's fullname diverge. RBAC accepts subjects
// that do not exist, so this mismatch would otherwise fail silently at runtime.
func TestRBACNFDWorkerServiceAccountMatchesSubchart(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	releaseName := "my-gpu"
	namespaceName := "rbac-test-nfd-sa-name"
	apiVersions := "--api-versions=security.openshift.io/v1/SecurityContextConstraints"
	options := &helm.Options{
		SetValues: map[string]string{
			"gfd.enabled": "true",
		},
		KubectlOptions: k8s.NewKubectlOptions("", "", namespaceName),
		Logger:         logger.Discard,
	}

	output := helm.RenderTemplate(t, options, helmChartPath, releaseName, nil, apiVersions)

	serviceAccounts := map[string]bool{}
	var sccBinding *rbacv1.RoleBinding
	for _, doc := range splitManifestDocuments(output) {
		var probe struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		helm.UnmarshalK8SYaml(t, doc, &probe)

		switch {
		case probe.Kind == "ServiceAccount":
			serviceAccounts[probe.Metadata.Name] = true
		case probe.Kind == "RoleBinding" && strings.HasSuffix(probe.Metadata.Name, "-scc-role-binding"):
			var rb rbacv1.RoleBinding
			helm.UnmarshalK8SYaml(t, doc, &rb)
			sccBinding = &rb
		}
	}

	require.NotNil(t, sccBinding, "SCC RoleBinding was not rendered")
	require.NotEmpty(t, serviceAccounts, "no ServiceAccounts rendered; subchart may not have loaded")

	var workerSubject string
	for _, s := range sccBinding.Subjects {
		if strings.HasSuffix(s.Name, "-worker") {
			workerSubject = s.Name
		}
	}
	require.NotEmpty(t, workerSubject, "SCC RoleBinding should bind the NFD worker ServiceAccount")
	require.True(t, serviceAccounts[workerSubject],
		"SCC RoleBinding references ServiceAccount %q, which no template creates; rendered ServiceAccounts: %v",
		workerSubject, keysOf(serviceAccounts))
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// splitManifestDocuments splits a multi-document manifest on document separators only,
// so that a "---" appearing inside a value does not truncate the surrounding document.
func splitManifestDocuments(output string) []string {
	var docs []string
	for _, p := range strings.Split("\n"+output, "\n---") {
		trimmed := strings.TrimSpace(p)
		if strings.Contains(trimmed, "apiVersion:") {
			docs = append(docs, trimmed)
		}
	}
	return docs
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

func ptr[T any](x T) *T {
	return &x
}
