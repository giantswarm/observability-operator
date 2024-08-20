package bundle

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	commonmonitoring "github.com/giantswarm/observability-operator/pkg/common/monitoring"
	"github.com/giantswarm/observability-operator/pkg/monitoring"
)

type BundleConfigurationService struct {
	client client.Client
	config monitoring.Config
}

func NewBundleConfigurationService(client client.Client, config monitoring.Config) *BundleConfigurationService {
	return &BundleConfigurationService{
		client: client,
		config: config,
	}
}

func (s *BundleConfigurationService) SetMonitoringAgent(monitoringAgent string) {
	s.config.MonitoringAgent = monitoringAgent
}

func getConfigMapObjectKey(cluster *clusterv1.Cluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-observability-platform-configuration", cluster.Name),
		Namespace: cluster.Namespace,
	}
}

// Configure configures the observability-bundle application.
// the observabilitybundle application to enable logging agents.
func (s BundleConfigurationService) Configure(ctx context.Context, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)
	logger.Info("configuring observability-bundle")

	bundleConfiguration := bundleConfiguration{
		Apps: map[string]app{},
	}

	switch s.config.MonitoringAgent {
	case commonmonitoring.MonitoringAgentPrometheus:
		bundleConfiguration.Apps[commonmonitoring.MonitoringPrometheusAgentAppName] = app{
			Enabled: s.config.IsMonitored(cluster),
		}
		bundleConfiguration.Apps[commonmonitoring.MonitoringAlloyAppName] = app{
			Enabled: false,
		}
	case commonmonitoring.MonitoringAgentAlloy:
		bundleConfiguration.Apps[commonmonitoring.MonitoringPrometheusAgentAppName] = app{
			Enabled: false,
		}
		bundleConfiguration.Apps[commonmonitoring.MonitoringAlloyAppName] = app{
			AppName: commonmonitoring.AlloyMonitoringAgentAppName,
			Enabled: s.config.IsMonitored(cluster),
		}
	default:
		return errors.Errorf("unsupported monitoring agent %q", s.config.MonitoringAgent)
	}

	logger.Info("creating or updating observability-bundle configmap")
	err := s.createOrUpdateObservabilityBundleConfigMap(ctx, cluster, bundleConfiguration)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("configure observability-bundle app")
	err = s.configureObservabilityBundleApp(ctx, cluster)
	if err != nil {
		return errors.WithStack(err)
	}

	logger.Info("observability-bundle is configured successfully")

	return nil
}

func (s BundleConfigurationService) createOrUpdateObservabilityBundleConfigMap(
	ctx context.Context, cluster *clusterv1.Cluster, configuration bundleConfiguration) error {

	logger := log.FromContext(ctx)

	values, err := yaml.Marshal(configuration)
	if err != nil {
		return errors.WithStack(err)
	}

	configMapObjectKey := getConfigMapObjectKey(cluster)
	desired := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapObjectKey.Name,
			Namespace: configMapObjectKey.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "observability-bundle",
				"app.kubernetes.io/managed-by": "observability-operator",
				"app.kubernetes.io/part-of":    "observability-platform",
			},
		},
		Data: map[string]string{"values": string(values)},
	}

	var current v1.ConfigMap
	err = s.client.Get(ctx, configMapObjectKey, &current)
	if err != nil {
		if apimachineryerrors.IsNotFound(err) {
			err = s.client.Create(ctx, &desired)
			if err != nil {
				return errors.WithStack(err)
			}
			logger.Info("observability-bundle configuration created")
		} else {
			return errors.WithStack(err)
		}
	}

	if !reflect.DeepEqual(current.Data, desired.Data) ||
		!reflect.DeepEqual(current.ObjectMeta.Labels, desired.ObjectMeta.Labels) {
		err := s.client.Update(ctx, &desired)
		if err != nil {
			return errors.WithStack(err)
		}
		logger.Info("observability-bundle configuration updated")
	}

	logger.Info("observability-bundle configuration up to date")
	return nil
}

func (s BundleConfigurationService) configureObservabilityBundleApp(
	ctx context.Context, cluster *clusterv1.Cluster) error {

	configMapObjectKey := getConfigMapObjectKey(cluster)

	// Get observability bundle app metadata.
	appObjectKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s-observability-bundle", cluster.Name),
		Namespace: cluster.Namespace,
	}

	var current appv1.App
	err := s.client.Get(ctx, appObjectKey, &current)
	if err != nil {
		return errors.WithStack(err)
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
			return errors.WithStack(err)
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
		return errors.WithStack(err)
	}

	logger.Info("observability-bundle configuration has been deleted successfully")

	return nil
}
