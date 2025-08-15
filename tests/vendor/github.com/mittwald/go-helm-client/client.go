package helmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var storage = repo.File{}

const (
	defaultCachePath            = "/tmp/.helmcache"
	defaultRepositoryConfigPath = "/tmp/.helmrepo"
)

// New returns a new Helm client with the provided options
func New(options *Options) (Client, error) {
	settings := cli.New()

	err := setEnvSettings(&options, settings)
	if err != nil {
		return nil, err
	}

	return newClient(options, settings.RESTClientGetter(), settings)
}

// NewClientFromKubeConf returns a new Helm client constructed with the provided kubeconfig & RESTClient (optional) options.
func NewClientFromKubeConf(options *KubeConfClientOptions, restClientOpts ...RESTClientOption) (Client, error) {
	settings := cli.New()
	if options.KubeConfig == nil {
		return nil, fmt.Errorf("kubeconfig missing")
	}

	clientGetter := NewRESTClientGetter(options.Namespace, options.KubeConfig, nil, restClientOpts...)

	if options.KubeContext != "" {
		settings.KubeContext = options.KubeContext
	}

	return newClient(options.Options, clientGetter, settings)
}

// NewClientFromRestConf returns a new Helm client constructed with the provided REST config options.
func NewClientFromRestConf(options *RestConfClientOptions) (Client, error) {
	settings := cli.New()

	clientGetter := NewRESTClientGetter(options.Namespace, nil, options.RestConfig)

	return newClient(options.Options, clientGetter, settings)
}

// newClient is used by both NewClientFromKubeConf and NewClientFromRestConf
// and returns a new Helm client via the provided options and REST config.
func newClient(options *Options, clientGetter genericclioptions.RESTClientGetter, settings *cli.EnvSettings) (Client, error) {
	err := setEnvSettings(&options, settings)
	if err != nil {
		return nil, err
	}

	debugLog := options.DebugLog
	if debugLog == nil {
		debugLog = func(format string, v ...interface{}) {
			log.Printf(format, v...)
		}
	}

	if options.Output == nil {
		options.Output = os.Stdout
	}

	actionConfig := new(action.Configuration)
	err = actionConfig.Init(
		clientGetter,
		settings.Namespace(),
		os.Getenv("HELM_DRIVER"),
		debugLog,
	)
	if err != nil {
		return nil, err
	}

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}
	actionConfig.RegistryClient = registryClient

	return &HelmClient{
		Settings:     settings,
		Providers:    getter.All(settings),
		storage:      &storage,
		ActionConfig: actionConfig,
		linting:      options.Linting,
		DebugLog:     debugLog,
		output:       options.Output,
	}, nil
}

// setEnvSettings sets the client's environment settings based on the provided client configuration.
func setEnvSettings(ppOptions **Options, settings *cli.EnvSettings) error {
	if *ppOptions == nil {
		*ppOptions = &Options{
			RepositoryConfig: defaultRepositoryConfigPath,
			RepositoryCache:  defaultCachePath,
			Linting:          true,
		}
	}

	options := *ppOptions

	// set the namespace with this ugly workaround because cli.EnvSettings.namespace is private
	// thank you helm!
	if options.Namespace != "" {
		pflags := pflag.NewFlagSet("", pflag.ContinueOnError)
		settings.AddFlags(pflags)
		err := pflags.Parse([]string{"-n", options.Namespace})
		if err != nil {
			return err
		}
	}

	if options.RepositoryConfig == "" {
		options.RepositoryConfig = defaultRepositoryConfigPath
	}

	if options.RepositoryCache == "" {
		options.RepositoryCache = defaultCachePath
	}

	settings.RepositoryCache = options.RepositoryCache
	settings.RepositoryConfig = options.RepositoryConfig
	settings.Debug = options.Debug

	if options.RegistryConfig != "" {
		settings.RegistryConfig = options.RegistryConfig
	}

	return nil
}

// AddOrUpdateChartRepo adds or updates the provided helm chart repository.
func (c *HelmClient) AddOrUpdateChartRepo(entry repo.Entry) error {
	chartRepo, err := repo.NewChartRepository(&entry, c.Providers)
	if err != nil {
		return err
	}

	chartRepo.CachePath = c.Settings.RepositoryCache

	if c.storage.Has(entry.Name) {
		c.DebugLog("WARNING: repository name %q already exists", entry.Name)
		return nil
	}

	if !registry.IsOCI(entry.URL) {
		_, err = chartRepo.DownloadIndexFile()
		if err != nil {
			return err
		}
	}

	c.storage.Update(&entry)
	err = c.storage.WriteFile(c.Settings.RepositoryConfig, 0o644)
	if err != nil {
		return err
	}

	return nil
}

// UpdateChartRepos updates the list of chart repositories stored in the client's cache.
func (c *HelmClient) UpdateChartRepos() error {
	for _, entry := range c.storage.Repositories {
		chartRepo, err := repo.NewChartRepository(entry, c.Providers)
		if err != nil {
			return err
		}

		chartRepo.CachePath = c.Settings.RepositoryCache
		if !registry.IsOCI(entry.URL) {
			_, err = chartRepo.DownloadIndexFile()
			if err != nil {
				return err
			}
		}

		c.storage.Update(entry)
	}

	return c.storage.WriteFile(c.Settings.RepositoryConfig, 0o644)
}

// InstallOrUpgradeChart installs or upgrades the provided chart and returns the corresponding release.
// Namespace and other context is provided via the helmclient.Options struct when instantiating a client.
func (c *HelmClient) InstallOrUpgradeChart(ctx context.Context, spec *ChartSpec, opts *GenericHelmOptions) (*release.Release, error) {
	exists, err := c.chartExists(spec)
	if err != nil {
		return nil, err
	}

	if exists {
		return c.upgrade(ctx, spec, opts)
	}

	return c.install(ctx, spec, opts)
}

// InstallChart installs the provided chart and returns the corresponding release.
// Namespace and other context is provided via the helmclient.Options struct when instantiating a client.
func (c *HelmClient) InstallChart(ctx context.Context, spec *ChartSpec, opts *GenericHelmOptions) (*release.Release, error) {
	return c.install(ctx, spec, opts)
}

// UpgradeChart upgrades the provided chart and returns the corresponding release.
// Namespace and other context is provided via the helmclient.Options struct when instantiating a client.
func (c *HelmClient) UpgradeChart(ctx context.Context, spec *ChartSpec, opts *GenericHelmOptions) (*release.Release, error) {
	return c.upgrade(ctx, spec, opts)
}

// ListDeployedReleases lists all deployed releases.
// Namespace and other context is provided via the helmclient.Options struct when instantiating a client.
func (c *HelmClient) ListDeployedReleases() ([]*release.Release, error) {
	return c.listReleases(action.ListDeployed)
}

// ListReleasesByStateMask lists all releases filtered by stateMask.
// Namespace and other context is provided via the helmclient.Options struct when instantiating a client.
func (c *HelmClient) ListReleasesByStateMask(states action.ListStates) ([]*release.Release, error) {
	return c.listReleases(states)
}

// GetReleaseValues returns the (optionally, all computed) values for the specified release.
func (c *HelmClient) GetReleaseValues(name string, allValues bool) (map[string]interface{}, error) {
	return c.getReleaseValues(name, allValues)
}

// GetRelease returns a release specified by name.
func (c *HelmClient) GetRelease(name string) (*release.Release, error) {
	return c.getRelease(name)
}

// RollbackRelease implicitly rolls back a release to the last revision.
func (c *HelmClient) RollbackRelease(spec *ChartSpec) error {
	return c.rollbackRelease(spec)
}

// UninstallRelease uninstalls the provided release
func (c *HelmClient) UninstallRelease(spec *ChartSpec) error {
	return c.uninstallRelease(spec)
}

// UninstallReleaseByName uninstalls a release identified by the provided 'name'.
func (c *HelmClient) UninstallReleaseByName(name string) error {
	return c.uninstallReleaseByName(name)
}

// install installs the provided chart.
// Optionally lints the chart if the linting flag is set.
func (c *HelmClient) install(ctx context.Context, spec *ChartSpec, opts *GenericHelmOptions) (*release.Release, error) {
	client := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, client)

	// NameAndChart returns either the TemplateName if set,
	// the ReleaseName if set or the generatedName as the first return value.
	releaseName, _, err := client.NameAndChart([]string{spec.ChartName})
	if err != nil {
		return nil, err
	}
	client.ReleaseName = releaseName

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	if opts != nil {
		if opts.PostRenderer != nil {
			client.PostRenderer = opts.PostRenderer
		}
	}

	helmChart, chartPath, err := c.GetChart(spec.ChartName, &client.ChartPathOptions)
	if err != nil {
		return nil, err
	}

	if helmChart.Metadata.Type != "" && helmChart.Metadata.Type != "application" {
		return nil, fmt.Errorf(
			"chart %q has an unsupported type and is not installable: %q",
			helmChart.Metadata.Name,
			helmChart.Metadata.Type,
		)
	}

	helmChart, err = updateDependencies(helmChart, &client.ChartPathOptions, chartPath, c, client.DependencyUpdate, spec)
	if err != nil {
		return nil, err
	}

	p := getter.All(c.Settings)
	values, err := spec.GetValuesMap(p)
	if err != nil {
		return nil, err
	}

	if c.linting {
		err = c.lint(chartPath, values)
		if err != nil {
			return nil, err
		}
	}

	rel, err := client.RunWithContext(ctx, helmChart, values)
	if err != nil {
		return rel, err
	}

	c.DebugLog("release installed successfully: %s/%s-%s", rel.Name, rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)

	return rel, nil
}

// upgrade upgrades a chart and CRDs.
// Optionally lints the chart if the linting flag is set.
func (c *HelmClient) upgrade(ctx context.Context, spec *ChartSpec, opts *GenericHelmOptions) (*release.Release, error) {
	client := action.NewUpgrade(c.ActionConfig)
	mergeUpgradeOptions(spec, client)
	client.Install = true

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	if opts != nil {
		if opts.PostRenderer != nil {
			client.PostRenderer = opts.PostRenderer
		}
	}

	helmChart, chartPath, err := c.GetChart(spec.ChartName, &client.ChartPathOptions)
	if err != nil {
		return nil, err
	}

	helmChart, err = updateDependencies(helmChart, &client.ChartPathOptions, chartPath, c, client.DependencyUpdate, spec)
	if err != nil {
		return nil, err
	}

	p := getter.All(c.Settings)
	values, err := spec.GetValuesMap(p)
	if err != nil {
		return nil, err
	}

	if c.linting {
		err = c.lint(chartPath, values)
		if err != nil {
			return nil, err
		}
	}

	if !spec.SkipCRDs && spec.UpgradeCRDs {
		c.DebugLog("upgrading crds")
		err = c.upgradeCRDs(ctx, helmChart)
		if err != nil {
			return nil, err
		}
	}

	upgradedRelease, upgradeErr := client.RunWithContext(ctx, spec.ReleaseName, helmChart, values)
	if upgradeErr != nil {
		resultErr := upgradeErr
		if upgradedRelease == nil && opts != nil && opts.RollBack != nil {
			rollbackErr := opts.RollBack.RollbackRelease(spec)
			if rollbackErr != nil {
				resultErr = fmt.Errorf("release failed, rollback failed: release error: %w, rollback error: %v", upgradeErr, rollbackErr)
			} else {
				resultErr = fmt.Errorf("release failed, rollback succeeded: release error: %w", upgradeErr)
			}
		}
		c.DebugLog("release upgrade failed: %s", resultErr)
		return nil, resultErr
	}

	c.DebugLog("release upgraded successfully: %s/%s-%s", upgradedRelease.Name, upgradedRelease.Chart.Metadata.Name, upgradedRelease.Chart.Metadata.Version)

	return upgradedRelease, nil
}

// uninstallRelease uninstalls the provided release.
func (c *HelmClient) uninstallRelease(spec *ChartSpec) error {
	client := action.NewUninstall(c.ActionConfig)

	mergeUninstallReleaseOptions(spec, client)

	resp, err := client.Run(spec.ReleaseName)
	if err != nil {
		return err
	}

	c.DebugLog("release uninstalled, response: %v", resp)

	return nil
}

// uninstallReleaseByName uninstalls a release identified by the provided 'name'.
func (c *HelmClient) uninstallReleaseByName(name string) error {
	client := action.NewUninstall(c.ActionConfig)

	resp, err := client.Run(name)
	if err != nil {
		return err
	}

	c.DebugLog("release uninstalled, response: %v", resp)

	return nil
}

// lint lints a chart's values.
func (c *HelmClient) lint(chartPath string, values map[string]interface{}) error {
	client := action.NewLint()

	result := client.Run([]string{chartPath}, values)

	for _, err := range result.Errors {
		c.DebugLog("Error %s", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("linting for chartpath %q failed", chartPath)
	}

	return nil
}

// TemplateChart returns a rendered version of the provided ChartSpec 'spec' by performing a "dry-run" install.
func (c *HelmClient) TemplateChart(spec *ChartSpec, options *HelmTemplateOptions) ([]byte, error) {
	client := action.NewInstall(c.ActionConfig)
	mergeInstallOptions(spec, client)

	client.DryRun = true
	client.ReleaseName = spec.ReleaseName
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.IncludeCRDs = true

	if options != nil {
		client.KubeVersion = options.KubeVersion
		client.APIVersions = options.APIVersions
	}

	// NameAndChart returns either the TemplateName if set,
	// the ReleaseName if set or the generatedName as the first return value.
	releaseName, _, err := client.NameAndChart([]string{spec.ChartName})
	if err != nil {
		return nil, err
	}
	client.ReleaseName = releaseName

	if client.Version == "" {
		client.Version = ">0.0.0-0"
	}

	helmChart, chartPath, err := c.GetChart(spec.ChartName, &client.ChartPathOptions)
	if err != nil {
		return nil, err
	}

	if helmChart.Metadata.Type != "" && helmChart.Metadata.Type != "application" {
		return nil, fmt.Errorf(
			"chart %q has an unsupported type and is not installable: %q",
			helmChart.Metadata.Name,
			helmChart.Metadata.Type,
		)
	}

	helmChart, err = updateDependencies(helmChart, &client.ChartPathOptions, chartPath, c, client.DependencyUpdate, spec)
	if err != nil {
		return nil, err
	}

	p := getter.All(c.Settings)
	values, err := spec.GetValuesMap(p)
	if err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	rel, err := client.Run(helmChart, values)

	// We ignore a potential error here because, when the --debug flag was specified,
	// we always want to print the YAML, even if it is not valid. The error is still returned afterwards.
	if rel != nil {
		var manifests bytes.Buffer
		fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
		if !client.DisableHooks {
			for _, m := range rel.Hooks {
				fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
			}
		}

		// if we have a list of files to render, then check that each of the
		// provided files exists in the chart.
		fmt.Fprintf(out, "%s", manifests.String())
	}

	return out.Bytes(), err
}

// LintChart fetches a chart using the provided ChartSpec 'spec' and lints it's values.
func (c *HelmClient) LintChart(spec *ChartSpec) error {
	_, chartPath, err := c.GetChart(spec.ChartName, &action.ChartPathOptions{
		Version: spec.Version,
	})
	if err != nil {
		return err
	}

	p := getter.All(c.Settings)
	values, err := spec.GetValuesMap(p)
	if err != nil {
		return err
	}

	return c.lint(chartPath, values)
}

// SetDebugLog set's a Helm client's DebugLog to the desired 'debugLog'.
func (c *HelmClient) SetDebugLog(debugLog action.DebugLog) {
	c.DebugLog = debugLog
}

// ListReleaseHistory lists the last 'max' number of entries
// in the history of the release identified by 'name'.
func (c *HelmClient) ListReleaseHistory(name string, max int) ([]*release.Release, error) {
	client := action.NewHistory(c.ActionConfig)

	client.Max = max

	return client.Run(name)
}

// upgradeCRDs upgrades the CRDs of the provided chart.
func (c *HelmClient) upgradeCRDs(ctx context.Context, chartInstance *chart.Chart) error {
	cfg, err := c.ActionConfig.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return err
	}

	k8sClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	for _, crd := range chartInstance.CRDObjects() {
		if err := c.upgradeCRD(ctx, k8sClient, crd); err != nil {
			return err
		}
		c.DebugLog("CRD %s upgraded successfully for chart: %s", crd.Name, chartInstance.Metadata.Name)
	}

	return nil
}

// upgradeCRD upgrades the CRD 'crd' using the provided k8s client.
func (c *HelmClient) upgradeCRD(ctx context.Context, k8sClient *clientset.Clientset, crd chart.CRD) error {
	// use this ugly detour to parse the crdYaml to a CustomResourceDefinitions-Object because direct
	// yaml-unmarshalling does not find the correct keys
	jsonCRD, err := yaml.ToJSON(crd.File.Data)
	if err != nil {
		return err
	}

	var typeMeta metav1.TypeMeta
	err = json.Unmarshal(jsonCRD, &typeMeta)
	if err != nil {
		return err
	}

	switch typeMeta.APIVersion {
	default:
		return fmt.Errorf("WARNING: failed to upgrade CRD %q: unsupported api-version %q", crd.Name, typeMeta.APIVersion)
	case "apiextensions.k8s.io/v1beta1":
		return c.upgradeCRDV1Beta1(ctx, k8sClient, jsonCRD)
	case "apiextensions.k8s.io/v1":
		return c.upgradeCRDV1(ctx, k8sClient, jsonCRD)
	}
}

func (c *HelmClient) createCRDV1(ctx context.Context, cl *clientset.Clientset, crd *v1.CustomResourceDefinition) error {
	if _, err := cl.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{}); err != nil {
		return err
	}

	c.DebugLog("create ran successful for CRD: %s", crd.Name)
	return nil
}

func (c *HelmClient) createCRDV1Beta1(ctx context.Context, cl *clientset.Clientset, crd *v1beta1.CustomResourceDefinition) error {
	if _, err := cl.ApiextensionsV1beta1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{}); err != nil {
		return err
	}

	c.DebugLog("create ran successful for CRD: %s", crd.Name)
	return nil
}

// upgradeCRDV1Beta1 upgrades a CRD of the v1beta1 API version using the provided k8s client and CRD yaml.
func (c *HelmClient) upgradeCRDV1Beta1(ctx context.Context, cl *clientset.Clientset, rawCRD []byte) error {
	var crdObj v1beta1.CustomResourceDefinition
	if err := json.Unmarshal(rawCRD, &crdObj); err != nil {
		return err
	}

	existingCRDObj, err := cl.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.createCRDV1Beta1(ctx, cl, &crdObj)
		}

		return err
	}

	// Check that the storage version does not change through the update.
	oldStorageVersion := v1beta1.CustomResourceDefinitionVersion{}

	for _, oldVersion := range existingCRDObj.Spec.Versions {
		if oldVersion.Storage {
			oldStorageVersion = oldVersion
		}
	}

	i := 0

	for _, newVersion := range crdObj.Spec.Versions {
		if newVersion.Storage {
			i++
			if newVersion.Name != oldStorageVersion.Name {
				return fmt.Errorf("ERROR: storage version of CRD %q changed, aborting upgrade", crdObj.Name)
			}
		}
		if i > 1 {
			return fmt.Errorf("ERROR: more than one storage version set on CRD %q, aborting upgrade", crdObj.Name)
		}
	}

	if reflect.DeepEqual(existingCRDObj.Spec.Versions, crdObj.Spec.Versions) {
		c.DebugLog("INFO: new version of CRD %q contains no changes, skipping upgrade", crdObj.Name)
		return nil
	}

	crdObj.ResourceVersion = existingCRDObj.ResourceVersion
	if _, err := cl.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{DryRun: []string{"All"}}); err != nil {
		return err
	}
	c.DebugLog("upgrade ran successful for CRD (dry run): %s", crdObj.Name)

	if _, err = cl.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	c.DebugLog("upgrade ran successful for CRD: %s", crdObj.Name)

	return nil
}

// upgradeCRDV1Beta1 upgrades a CRD of the v1 API version using the provided k8s client and CRD yaml.
func (c *HelmClient) upgradeCRDV1(ctx context.Context, cl *clientset.Clientset, rawCRD []byte) error {
	var crdObj v1.CustomResourceDefinition
	if err := json.Unmarshal(rawCRD, &crdObj); err != nil {
		return err
	}

	existingCRDObj, err := cl.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdObj.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.createCRDV1(ctx, cl, &crdObj)
		}

		return err
	}

	// Check to ensure that no previously existing API version is deleted through the upgrade.
	if len(existingCRDObj.Spec.Versions) > len(crdObj.Spec.Versions) {
		c.DebugLog("WARNING: new version of CRD %q would remove an existing API version, skipping upgrade", crdObj.Name)
		return nil
	}

	// Check that the storage version does not change through the update.
	oldStorageVersion := v1.CustomResourceDefinitionVersion{}

	for _, oldVersion := range existingCRDObj.Spec.Versions {
		if oldVersion.Storage {
			oldStorageVersion = oldVersion
		}
	}

	i := 0

	for _, newVersion := range crdObj.Spec.Versions {
		if newVersion.Storage {
			i++
			if newVersion.Name != oldStorageVersion.Name {
				return fmt.Errorf("ERROR: storage version of CRD %q changed, aborting upgrade", crdObj.Name)
			}
		}
		if i > 1 {
			return fmt.Errorf("ERROR: more than one storage version set on CRD %q, aborting upgrade", crdObj.Name)
		}
	}

	if reflect.DeepEqual(existingCRDObj.Spec.Versions, crdObj.Spec.Versions) {
		c.DebugLog("INFO: new version of CRD %q contains no changes, skipping upgrade", crdObj.Name)
		return nil
	}

	crdObj.ResourceVersion = existingCRDObj.ResourceVersion
	if _, err := cl.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{DryRun: []string{"All"}}); err != nil {
		return err
	}
	c.DebugLog("upgrade ran successful for CRD (dry run): %s", crdObj.Name)

	if _, err := cl.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, &crdObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	c.DebugLog("upgrade ran successful for CRD: %s", crdObj.Name)

	return nil
}

// GetChart returns a chart matching the provided chart name and options.
func (c *HelmClient) GetChart(chartName string, chartPathOptions *action.ChartPathOptions) (*chart.Chart, string, error) {
	chartPath, err := chartPathOptions.LocateChart(chartName, c.Settings)
	if err != nil {
		return nil, "", err
	}

	helmChart, err := loader.Load(chartPath)
	if err != nil {
		return nil, "", err
	}

	if helmChart.Metadata.Deprecated {
		c.DebugLog("WARNING: This chart (%q) is deprecated", helmChart.Metadata.Name)
	}

	return helmChart, chartPath, err
}

// RunTests runs the tests that were deployed with the release provided. It returns true
// if all the tests ran successfully and false in all other cases.
// NOTE: error = nil implies that all tests ran to either success or failure.
func (c *HelmClient) RunChartTests(releaseName string) (bool, error) {

	client := action.NewReleaseTesting(c.ActionConfig)

	if c.Settings.Namespace() == "" {
		return false, fmt.Errorf("namespace not set")
	}

	client.Namespace = c.Settings.Namespace()

	rel, err := client.Run(releaseName)
	if err != nil && rel == nil {
		return false, fmt.Errorf("unable to find release '%s': %v", releaseName, err)
	}

	// Check that there are no test failures
	return !checkReleaseForTestFailure(rel), nil
}

// chartExists checks whether a chart is already installed
// in a namespace or not based on the provided chart spec.
// Note that this function only considers the contained chart name and namespace.
func (c *HelmClient) chartExists(spec *ChartSpec) (bool, error) {
	releases, err := c.listReleases(action.ListAll)
	if err != nil {
		return false, err
	}

	for _, r := range releases {
		if r.Name == spec.ReleaseName && r.Namespace == spec.Namespace {
			return true, nil
		}
	}

	return false, nil
}

// listReleases lists all releases that match the given state.
func (c *HelmClient) listReleases(state action.ListStates) ([]*release.Release, error) {
	listClient := action.NewList(c.ActionConfig)
	listClient.StateMask = state

	return listClient.Run()
}

// getReleaseValues returns the values for the provided release 'name'.
// If allValues = true is specified, all computed values are returned.
func (c *HelmClient) getReleaseValues(name string, allValues bool) (map[string]interface{}, error) {
	getReleaseValuesClient := action.NewGetValues(c.ActionConfig)

	getReleaseValuesClient.AllValues = allValues

	return getReleaseValuesClient.Run(name)
}

// getRelease returns a release matching the provided 'name'.
func (c *HelmClient) getRelease(name string) (*release.Release, error) {
	getReleaseClient := action.NewGet(c.ActionConfig)

	return getReleaseClient.Run(name)
}

// rollbackRelease implicitly rolls back a release to the last revision.
func (c *HelmClient) rollbackRelease(spec *ChartSpec) error {
	client := action.NewRollback(c.ActionConfig)

	mergeRollbackOptions(spec, client)

	return client.Run(spec.ReleaseName)
}

// updateDependencies checks dependencies for given helmChart and updates dependencies with metadata if dependencyUpdate is true. returns updated HelmChart
func updateDependencies(helmChart *chart.Chart, chartPathOptions *action.ChartPathOptions, chartPath string, c *HelmClient, dependencyUpdate bool, spec *ChartSpec) (*chart.Chart, error) {
	if req := helmChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(helmChart, req); err != nil {
			if dependencyUpdate {
				man := &downloader.Manager{
					ChartPath:        chartPath,
					Keyring:          chartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          c.Providers,
					RepositoryConfig: c.Settings.RepositoryConfig,
					RepositoryCache:  c.Settings.RepositoryCache,
					Out:              c.output,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}

				helmChart, _, err = c.GetChart(spec.ChartName, chartPathOptions)
				if err != nil {
					return nil, err
				}

			} else {
				return nil, err
			}
		}
	}
	return helmChart, nil
}

// checkReleaseForTestFailure parses the list of hooks in the release
// and checks the status of the test hooks, returning true if any test has Phase != Succeeded
// Returns false if all tests have passed (including if there are no tests)
func checkReleaseForTestFailure(rel *release.Release) bool {
	// Check if any test failed
	hooksToCheck := []*release.Hook{}
	for _, hook := range rel.Hooks {
		// Only check the Phase for events which are supposed to get triggered for "test" hook
		if slices.Contains(hook.Events, release.HookTest) {
			hooksToCheck = append(hooksToCheck, hook)
		}
	}
	return slices.ContainsFunc(hooksToCheck, func(h *release.Hook) bool {
		return h.LastRun.Phase != release.HookPhaseSucceeded
	})
}

// mergeRollbackOptions merges values of the provided chart to helm rollback options used by the client.
func mergeRollbackOptions(chartSpec *ChartSpec, rollbackOptions *action.Rollback) {
	rollbackOptions.DisableHooks = chartSpec.DisableHooks
	rollbackOptions.DryRun = chartSpec.DryRun
	rollbackOptions.Timeout = chartSpec.Timeout
	rollbackOptions.CleanupOnFail = chartSpec.CleanupOnFail
	rollbackOptions.Force = chartSpec.Force
	rollbackOptions.MaxHistory = chartSpec.MaxHistory
	rollbackOptions.Recreate = chartSpec.Recreate
	rollbackOptions.Wait = chartSpec.Wait
	rollbackOptions.WaitForJobs = chartSpec.WaitForJobs
}

// mergeInstallOptions merges values of the provided chart to helm install options used by the client.
func mergeInstallOptions(chartSpec *ChartSpec, installOptions *action.Install) {
	installOptions.CreateNamespace = chartSpec.CreateNamespace
	installOptions.DisableHooks = chartSpec.DisableHooks
	installOptions.Replace = chartSpec.Replace
	installOptions.Wait = chartSpec.Wait
	installOptions.DependencyUpdate = chartSpec.DependencyUpdate
	installOptions.Timeout = chartSpec.Timeout
	installOptions.Namespace = chartSpec.Namespace
	installOptions.ReleaseName = chartSpec.ReleaseName
	installOptions.Version = chartSpec.Version
	installOptions.GenerateName = chartSpec.GenerateName
	installOptions.NameTemplate = chartSpec.NameTemplate
	installOptions.Atomic = chartSpec.Atomic
	installOptions.SkipCRDs = chartSpec.SkipCRDs
	installOptions.DryRun = chartSpec.DryRun
	installOptions.DryRunOption = chartSpec.DryRunOption
	installOptions.SubNotes = chartSpec.SubNotes
	installOptions.WaitForJobs = chartSpec.WaitForJobs
	installOptions.Labels = chartSpec.Labels
}

// mergeUpgradeOptions merges values of the provided chart to helm upgrade options used by the client.
func mergeUpgradeOptions(chartSpec *ChartSpec, upgradeOptions *action.Upgrade) {
	upgradeOptions.Version = chartSpec.Version
	upgradeOptions.Namespace = chartSpec.Namespace
	upgradeOptions.Timeout = chartSpec.Timeout
	upgradeOptions.Wait = chartSpec.Wait
	upgradeOptions.DependencyUpdate = chartSpec.DependencyUpdate
	upgradeOptions.DisableHooks = chartSpec.DisableHooks
	upgradeOptions.Force = chartSpec.Force
	upgradeOptions.ResetValues = chartSpec.ResetValues
	upgradeOptions.ReuseValues = chartSpec.ReuseValues
	upgradeOptions.ResetThenReuseValues = chartSpec.ResetThenReuseValues
	upgradeOptions.Recreate = chartSpec.Recreate
	upgradeOptions.MaxHistory = chartSpec.MaxHistory
	upgradeOptions.Atomic = chartSpec.Atomic
	upgradeOptions.CleanupOnFail = chartSpec.CleanupOnFail
	upgradeOptions.DryRun = chartSpec.DryRun
	upgradeOptions.DryRunOption = chartSpec.DryRunOption
	upgradeOptions.SubNotes = chartSpec.SubNotes
	upgradeOptions.WaitForJobs = chartSpec.WaitForJobs
	upgradeOptions.Labels = chartSpec.Labels
}

// mergeUninstallReleaseOptions merges values of the provided chart to helm uninstall options used by the client.
func mergeUninstallReleaseOptions(chartSpec *ChartSpec, uninstallReleaseOptions *action.Uninstall) {
	uninstallReleaseOptions.DisableHooks = chartSpec.DisableHooks
	uninstallReleaseOptions.Timeout = chartSpec.Timeout
	uninstallReleaseOptions.DryRun = chartSpec.DryRun
	uninstallReleaseOptions.Description = chartSpec.Description
	uninstallReleaseOptions.KeepHistory = chartSpec.KeepHistory
	uninstallReleaseOptions.Wait = chartSpec.Wait
}
