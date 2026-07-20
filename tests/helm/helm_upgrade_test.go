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

package helm_test

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	"helm.sh/helm/v4/pkg/strvals"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const mpsSharingConfig = `version: v1
sharing:
  mps:
    resources:
    - name: nvidia.com/gpu
      replicas: 2
`

func TestReuseValuesUpgradeWithNullValue(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	testCases := []struct {
		nullValuePath string
		// On main as well, null only renders for these with GFD and an embedded config left off.
		installWithChartDefaults bool
	}{
		{nullValuePath: "affinity"},
		{nullValuePath: "allowDefaultNamespace"},
		{nullValuePath: "componentSelectorLabels"},
		{nullValuePath: "componentSelectorLabels.enabled"},
		{nullValuePath: "config.default"},
		{nullValuePath: "config.fallbackStrategies", installWithChartDefaults: true},
		{nullValuePath: "config.map"},
		{nullValuePath: "config.name"},
		{nullValuePath: "configManager"},
		{nullValuePath: "configManager.resources"},
		{nullValuePath: "devicePlugin.enableHostNetwork"},
		{nullValuePath: "devicePlugin.enabled"},
		{nullValuePath: "devicePlugin.nvidiaVisibleDevices"},
		{nullValuePath: "devicePlugin.resources"},
		{nullValuePath: "fullnameOverride"},
		{nullValuePath: "gfd.enableHostNetwork"},
		{nullValuePath: "gfd.enabled"},
		{nullValuePath: "gfd.nameOverride"},
		{nullValuePath: "gfd.namespaceOverride"},
		{nullValuePath: "gfd.resources"},
		{nullValuePath: "gfd.securityContext", installWithChartDefaults: true},
		{nullValuePath: "gfd.securityContext.privileged"},
		{nullValuePath: "image.pullPolicy"},
		{nullValuePath: "image.tag"},
		{nullValuePath: "imagePullSecrets"},
		{nullValuePath: "mps.enableHostNetwork"},
		{nullValuePath: "mps.enableHostPID"},
		{nullValuePath: "mps.nvidiaVisibleDevices"},
		{nullValuePath: "mps.resources"},
		{nullValuePath: "nameOverride"},
		{nullValuePath: "namespaceOverride"},
		{nullValuePath: "nodeSelector"},
		{nullValuePath: "podSecurityContext"},
		{nullValuePath: "priorityClassName"},
		{nullValuePath: "resources"},
		{nullValuePath: "selectorLabelsOverride"},
		{nullValuePath: "tolerations"},
		{nullValuePath: "updateStrategy"},
		{nullValuePath: "updateStrategy.type"},
	}

	for _, tc := range testCases {
		t.Run(tc.nullValuePath, func(t *testing.T) {
			installValues := map[string]any{}
			if !tc.installWithChartDefaults {
				require.NoError(t, strvals.ParseInto("gfd.enabled=true", installValues))
				require.NoError(t, strvals.ParseIntoString("config.map.default="+mpsSharingConfig, installValues))
			}
			require.NoError(t, strvals.ParseInto(tc.nullValuePath+"=null", installValues))

			configuration := action.NewConfiguration()
			configuration.Releases = storage.Init(driver.NewMemory())
			configuration.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
			configuration.Capabilities = common.DefaultCapabilities

			installedChart, err := loader.Load(helmChartPath)
			require.NoError(t, err)
			install := action.NewInstall(configuration)
			install.ReleaseName = "nvidia-device-plugin"
			install.Namespace = "k8s-device-plugin-test"
			// The fake kube client cannot install the NFD subchart CRDs.
			install.SkipCRDs = true
			_, err = install.Run(installedChart, installValues)
			require.NoError(t, err)

			upgradedChart, err := loader.Load(helmChartPath)
			require.NoError(t, err)
			upgrade := action.NewUpgrade(configuration)
			upgrade.Namespace = "k8s-device-plugin-test"
			// Unlike install, --reuse-values passes a stored null back into schema validation.
			upgrade.ReuseValues = true
			_, err = upgrade.Run("nvidia-device-plugin", upgradedChart, map[string]any{})
			require.NoError(t, err)
		})
	}
}

func TestReuseValuesUpgradeNvidiaVisibleDevices(t *testing.T) {
	helmChartPath, err := filepath.Abs("../../deployments/helm/nvidia-device-plugin")
	require.NoError(t, err)

	components := []struct {
		valuesKey     string
		containerName string
	}{
		{valuesKey: "devicePlugin", containerName: "nvidia-device-plugin-ctr"},
		{valuesKey: "mps", containerName: "mps-control-daemon-ctr"},
	}

	testCases := []struct {
		description string
		// Simulates an install from a chart release that predates the value.
		installChartWithoutValue  bool
		installValue              *string
		upgradeValue              *string
		expectedVisibleDevicesEnv []v1.EnvVar
	}{
		{
			description:               "upgrade from a chart without the value",
			installChartWithoutValue:  true,
			expectedVisibleDevicesEnv: []v1.EnvVar{{Name: "NVIDIA_VISIBLE_DEVICES", Value: "all"}},
		},
		{
			description:               "override is kept",
			installValue:              ptr("void"),
			expectedVisibleDevicesEnv: []v1.EnvVar{{Name: "NVIDIA_VISIBLE_DEVICES", Value: "void"}},
		},
		{
			description:               "override is cleared with all",
			installValue:              ptr("void"),
			upgradeValue:              ptr("all"),
			expectedVisibleDevicesEnv: []v1.EnvVar{{Name: "NVIDIA_VISIBLE_DEVICES", Value: "all"}},
		},
		{
			description:               "override is cleared with an empty string",
			installValue:              ptr("void"),
			upgradeValue:              ptr(""),
			expectedVisibleDevicesEnv: []v1.EnvVar{{Name: "NVIDIA_VISIBLE_DEVICES", Value: "all"}},
		},
	}

	for _, component := range components {
		for _, tc := range testCases {
			t.Run(component.valuesKey+"/"+tc.description, func(t *testing.T) {
				valuePath := component.valuesKey + ".nvidiaVisibleDevices"
				installValues := map[string]any{}
				if tc.installValue != nil {
					require.NoError(t, strvals.ParseInto(valuePath+"="+*tc.installValue, installValues))
				}
				upgradeValues := map[string]any{}
				if tc.upgradeValue != nil {
					require.NoError(t, strvals.ParseInto(valuePath+"="+*tc.upgradeValue, upgradeValues))
				}

				configuration := action.NewConfiguration()
				configuration.Releases = storage.Init(driver.NewMemory())
				configuration.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
				configuration.Capabilities = common.DefaultCapabilities

				installedChart, err := loader.Load(helmChartPath)
				require.NoError(t, err)
				if tc.installChartWithoutValue {
					componentValues, ok := installedChart.Values[component.valuesKey].(map[string]any)
					require.True(t, ok)
					delete(componentValues, "nvidiaVisibleDevices")
				}
				install := action.NewInstall(configuration)
				install.ReleaseName = "nvidia-device-plugin"
				install.Namespace = "k8s-device-plugin-test"
				install.SkipCRDs = true
				_, err = install.Run(installedChart, installValues)
				require.NoError(t, err)

				upgradedChart, err := loader.Load(helmChartPath)
				require.NoError(t, err)
				upgrade := action.NewUpgrade(configuration)
				upgrade.Namespace = "k8s-device-plugin-test"
				upgrade.ReuseValues = true
				upgradedRelease, err := upgrade.Run("nvidia-device-plugin", upgradedChart, upgradeValues)
				require.NoError(t, err)
				accessor, err := release.NewAccessor(upgradedRelease)
				require.NoError(t, err)

				require.Equal(t, tc.expectedVisibleDevicesEnv, containerEnvVarsNamed(t, accessor.Manifest(), component.containerName, "NVIDIA_VISIBLE_DEVICES"))
			})
		}
	}
}

func containerEnvVarsNamed(t *testing.T, manifest string, containerName string, envName string) []v1.EnvVar {
	t.Helper()
	var matches []v1.EnvVar
	for _, document := range strings.Split(manifest, "\n---") {
		var typeMeta metav1.TypeMeta
		require.NoError(t, yaml.Unmarshal([]byte(document), &typeMeta))
		if typeMeta.Kind != "DaemonSet" {
			continue
		}
		var daemonset appsv1.DaemonSet
		require.NoError(t, yaml.Unmarshal([]byte(document), &daemonset))
		for _, container := range daemonset.Spec.Template.Spec.Containers {
			if container.Name == containerName {
				matches = append(matches, envVarsNamed(container.Env, envName)...)
			}
		}
	}
	return matches
}
