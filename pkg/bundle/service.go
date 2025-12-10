package bundle

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/blang/semver/v4"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Configure creates or updates the observability-bundle configuration based on
// cluster feature flags and links it to the bundle app via extraConfigs.
func (s BundleConfigurationService) Configure(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("configuring bundle")

	bundleConfig := s.buildBundleConfiguration(cluster)

	logger.Info("creating or updating configmap")
	err := s.createOrUpdateConfigMap(ctx, cluster, bundleConfig)
	if err != nil {
		return fmt.Errorf("failed to create/update bundle configmap: %w", err)
	}
	logger.Info("configmap created or updated successfully")

	logger.Info("configuring bundle")
	err = s.configureApp(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to configure bundle: %w", err)
	}

	logger.Info("bundle configured successfully")

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

func (s BundleConfigurationService) configureApp(ctx context.Context, cluster *clusterv1.Cluster) error {
	configMapObjectKey := getConfigMapObjectKey(cluster)

	// Get observability bundle app metadata.
	appObjectKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", cluster.Name, observabilityBundleAppName),
		Namespace: cluster.Namespace,
	}

	var current appv1.App
	err := s.client.Get(ctx, appObjectKey, &current)
	if err != nil {
		return fmt.Errorf("failed to get app: %w", err)
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
			return fmt.Errorf("failed to update app: %w", err)
		}
	}

	return nil
}

func (s BundleConfigurationService) RemoveConfiguration(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	logger.Info("deleting observability-bundle configuration")

	configMapObjectKey := getConfigMapObjectKey(cluster)
	var current = v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapObjectKey.Name,
			Namespace: configMapObjectKey.Namespace,
		},
	}
	if err := s.client.Delete(ctx, &current); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete observability bundle configmap: %w", err)
	}

	logger.Info("observability-bundle configuration has been deleted successfully")

	return nil
}

// GetObservabilityBundleAppVersion retrieves the version of the observability bundle app
// installed in the cluster. It returns an error if the app is not found or if
// the version cannot be parsed.
func (s BundleConfigurationService) GetObservabilityBundleAppVersion(ctx context.Context, cluster *clusterv1.Cluster) (version semver.Version, err error) {
	// Get observability bundle app metadata.
	appMeta := types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", cluster.GetName(), observabilityBundleAppName),
		Namespace: cluster.GetNamespace(),
	}
	// Retrieve the app.
	var currentApp appv1.App
	err = s.client.Get(ctx, appMeta, &currentApp)
	if err != nil {
		return version, fmt.Errorf("failed to get observability bundle app: %w", err)
	}
	return semver.Parse(currentApp.Spec.Version)
}
