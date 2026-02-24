package bundle

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/blang/semver/v4"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/giantswarm/observability-operator/pkg/common/apps"
	"github.com/giantswarm/observability-operator/pkg/config"
)

const observabilityBundleAppName string = "observability-bundle"

var helmReleaseGVK = schema.GroupVersionKind{
	Group:   "helm.toolkit.fluxcd.io",
	Version: "v2",
	Kind:    "HelmRelease",
}

var ociRepositoryGVK = schema.GroupVersionKind{
	Group:   "source.toolkit.fluxcd.io",
	Version: "v1beta2",
	Kind:    "OCIRepository",
}

type BundleConfigurationService struct {
	client client.Client
	config config.Config
}

func NewBundleConfigurationService(client client.Client, config config.Config) *BundleConfigurationService {
	return &BundleConfigurationService{
		client: client,
		config: config,
	}
}

func getConfigMapObjectKey(cluster *clusterv1.Cluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-observability-platform-configuration", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

func getBundleObjectKey(cluster *clusterv1.Cluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", cluster.Name, observabilityBundleAppName),
		Namespace: cluster.Namespace,
	}
}

// Configure creates or updates the observability-bundle configuration based on
// cluster feature flags and links it to the bundle CR (HelmRelease or App) via
// valuesFrom or extraConfigs respectively.
func (s BundleConfigurationService) Configure(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	bundleConfig := s.buildBundleConfiguration(cluster)

	logger.Info("creating or updating observability-bundle configmap")
	err := s.createOrUpdateConfigMap(ctx, cluster, bundleConfig)
	if err != nil {
		return err
	}
	logger.Info("observability-bundle configmap created or updated successfully")

	logger.Info("configuring observability-bundle")
	err = s.configureBundle(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to configure observability-bundle: %w", err)
	}

	logger.Info("observability-bundle configured successfully")

	return nil
}

// buildBundleConfiguration creates the bundle configuration based on cluster feature flags.
func (s BundleConfigurationService) buildBundleConfiguration(cluster *clusterv1.Cluster) bundleConfiguration {
	return bundleConfiguration{
		Apps: map[string]app{
			apps.AlloyMetricsHelmValueKey: {
				AppName: apps.AlloyMetricsAppName,
				Enabled: s.config.Monitoring.IsMonitoringEnabled(cluster),
			},
			apps.AlloyLogsHelmValueKey: {
				AppName: apps.AlloyLogsAppName,
				Enabled: s.config.Logging.IsLoggingEnabled(cluster),
			},
			apps.AlloyEventsHelmValueKey: {
				AppName: apps.AlloyEventsAppName,
				Enabled: s.isEventsEnabled(cluster),
			},
		},
	}
}

// isEventsEnabled returns true if events logging should be enabled.
// Events are enabled when either logging or tracing is enabled.
func (s BundleConfigurationService) isEventsEnabled(cluster *clusterv1.Cluster) bool {
	return s.config.Logging.IsLoggingEnabled(cluster) || s.config.Tracing.IsTracingEnabled(cluster)
}

func (s BundleConfigurationService) createOrUpdateConfigMap(ctx context.Context, cluster *clusterv1.Cluster, configuration bundleConfiguration) error {
	configMapObjectKey := getConfigMapObjectKey(cluster)
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapObjectKey.Name,
			Namespace: configMapObjectKey.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, s.client, configMap, func() error {
		configMap.Labels = map[string]string{
			"app.kubernetes.io/name":       observabilityBundleAppName,
			"app.kubernetes.io/managed-by": "observability-operator",
			"app.kubernetes.io/part-of":    "observability-platform",
		}
		values, err := yaml.Marshal(configuration)
		if err != nil {
			return fmt.Errorf("failed to marshal configuration to yaml: %w", err)
		}

		configMap.Data = map[string]string{
			"values": string(values),
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update configmap: %w", err)
	}

	return nil
}

// configureBundle detects whether the cluster uses a Flux HelmRelease or a
// Giant Swarm App CR for the observability-bundle, and configures the
// appropriate resource. It tries HelmRelease first, falling back to App CR.
func (s BundleConfigurationService) configureBundle(ctx context.Context, cluster *clusterv1.Cluster) error {
	bundleObjectKey := getBundleObjectKey(cluster)

	// Try HelmRelease first
	hr, err := s.getHelmRelease(ctx, bundleObjectKey)
	if err == nil {
		return s.configureHelmRelease(ctx, cluster, hr)
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get observability-bundle helmrelease: %w", err)
	}

	// Fall back to App CR
	return s.configureApp(ctx, cluster)
}

// getHelmRelease fetches a HelmRelease as an unstructured object.
func (s BundleConfigurationService) getHelmRelease(ctx context.Context, key types.NamespacedName) (*unstructured.Unstructured, error) {
	hr := &unstructured.Unstructured{}
	hr.SetGroupVersionKind(helmReleaseGVK)
	err := s.client.Get(ctx, key, hr)
	if err != nil {
		return nil, err
	}
	return hr, nil
}

// configureHelmRelease updates the HelmRelease's spec.valuesFrom to reference
// the observability-platform-configuration ConfigMap.
func (s BundleConfigurationService) configureHelmRelease(ctx context.Context, cluster *clusterv1.Cluster, hr *unstructured.Unstructured) error {
	configMapObjectKey := getConfigMapObjectKey(cluster)

	desiredEntry := map[string]interface{}{
		"kind":      "ConfigMap",
		"name":      configMapObjectKey.Name,
		"valuesKey": "values",
	}

	spec, ok := hr.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("helmrelease %s/%s has no spec", hr.GetNamespace(), hr.GetName())
	}

	// Get existing valuesFrom or initialize empty slice
	var valuesFrom []interface{}
	if existing, ok := spec["valuesFrom"].([]interface{}); ok {
		valuesFrom = existing
	}

	// Find existing entry by kind+name match
	foundIndex := slices.IndexFunc(valuesFrom, func(item interface{}) bool {
		entry, ok := item.(map[string]interface{})
		if !ok {
			return false
		}
		return entry["kind"] == desiredEntry["kind"] && entry["name"] == desiredEntry["name"]
	})

	if foundIndex == -1 {
		valuesFrom = append(valuesFrom, desiredEntry)
	} else {
		if reflect.DeepEqual(valuesFrom[foundIndex], desiredEntry) {
			return nil // Already up to date
		}
		valuesFrom[foundIndex] = desiredEntry
	}

	spec["valuesFrom"] = valuesFrom

	err := s.client.Update(ctx, hr)
	if err != nil {
		return fmt.Errorf("failed to update observability-bundle helmrelease: %w", err)
	}

	return nil
}

func (s BundleConfigurationService) configureApp(ctx context.Context, cluster *clusterv1.Cluster) error {
	configMapObjectKey := getConfigMapObjectKey(cluster)
	appObjectKey := getBundleObjectKey(cluster)

	var current appv1.App
	err := s.client.Get(ctx, appObjectKey, &current)
	if err != nil {
		return fmt.Errorf("failed to get observability-bundle app: %w", err)
	}

	desired := current.DeepCopy()

	desiredExtraConfig := appv1.AppExtraConfig{
		Kind:      "configMap",
		Name:      configMapObjectKey.Name,
		Namespace: configMapObjectKey.Namespace,
		Priority:  25,
	}

	foundIndex := slices.IndexFunc(current.Spec.ExtraConfigs, func(extraConfig appv1.AppExtraConfig) bool {
		// We skip priority in case we want to change it
		return extraConfig.Kind == desiredExtraConfig.Kind &&
			extraConfig.Name == desiredExtraConfig.Name &&
			extraConfig.Namespace == desiredExtraConfig.Namespace
	})

	if foundIndex == -1 {
		desired.Spec.ExtraConfigs = append(desired.Spec.ExtraConfigs, desiredExtraConfig)
	} else {
		desired.Spec.ExtraConfigs[foundIndex] = desiredExtraConfig
	}

	if !reflect.DeepEqual(current, *desired) {
		err := s.client.Update(ctx, desired)
		if err != nil {
			return fmt.Errorf("failed to update observability-bundle app: %w", err)
		}
	}

	return nil
}

func (s BundleConfigurationService) RemoveConfiguration(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting observability-bundle configmap")

	configMapObjectKey := getConfigMapObjectKey(cluster)
	var current = v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapObjectKey.Name,
			Namespace: configMapObjectKey.Namespace,
		},
	}
	if err := s.client.Delete(ctx, &current); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete observability-bundle configmap: %w", err)
	}

	logger.Info("observability-bundle configmap has been deleted successfully")

	return nil
}

// GetObservabilityBundleAppVersion retrieves the version of the observability-bundle
// installed in the cluster. It supports both Flux HelmRelease and Giant Swarm App CRs,
// trying HelmRelease first and falling back to App CR.
func (s BundleConfigurationService) GetObservabilityBundleAppVersion(ctx context.Context, cluster *clusterv1.Cluster) (version semver.Version, err error) {
	bundleObjectKey := getBundleObjectKey(cluster)

	// Try HelmRelease first
	hr, err := s.getHelmRelease(ctx, bundleObjectKey)
	if err == nil {
		return s.getHelmReleaseVersion(ctx, hr)
	}
	if !apierrors.IsNotFound(err) {
		return version, fmt.Errorf("failed to get observability-bundle helmrelease: %w", err)
	}

	// Fall back to App CR
	var currentApp appv1.App
	err = s.client.Get(ctx, bundleObjectKey, &currentApp)
	if err != nil {
		return version, fmt.Errorf("failed to get observability-bundle app: %w", err)
	}
	return semver.Parse(currentApp.Spec.Version)
}

// getHelmReleaseVersion extracts the chart version from a HelmRelease by following
// its spec.chartRef to the referenced OCIRepository and reading spec.ref.tag.
func (s BundleConfigurationService) getHelmReleaseVersion(ctx context.Context, hr *unstructured.Unstructured) (semver.Version, error) {
	spec, ok := hr.Object["spec"].(map[string]interface{})
	if !ok {
		return semver.Version{}, fmt.Errorf("helmrelease %s/%s has no spec", hr.GetNamespace(), hr.GetName())
	}

	chartRef, ok := spec["chartRef"].(map[string]interface{})
	if !ok {
		return semver.Version{}, fmt.Errorf("helmrelease %s/%s has no spec.chartRef", hr.GetNamespace(), hr.GetName())
	}

	name, _ := chartRef["name"].(string)
	namespace, _ := chartRef["namespace"].(string)
	if namespace == "" {
		namespace = hr.GetNamespace()
	}

	// Fetch the referenced OCIRepository
	ociRepo := &unstructured.Unstructured{}
	ociRepo.SetGroupVersionKind(ociRepositoryGVK)
	err := s.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ociRepo)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to get OCIRepository %s/%s: %w", namespace, name, err)
	}

	// Read spec.ref.tag
	ociSpec, ok := ociRepo.Object["spec"].(map[string]interface{})
	if !ok {
		return semver.Version{}, fmt.Errorf("ocirepository %s/%s has no spec", namespace, name)
	}

	ref, ok := ociSpec["ref"].(map[string]interface{})
	if !ok {
		return semver.Version{}, fmt.Errorf("ocirepository %s/%s has no spec.ref", namespace, name)
	}

	tag, ok := ref["tag"].(string)
	if !ok {
		return semver.Version{}, fmt.Errorf("ocirepository %s/%s has no spec.ref.tag", namespace, name)
	}

	return semver.Parse(tag)
}
