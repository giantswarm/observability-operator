package bundle

import (
	"context"
	"testing"

	"github.com/blang/semver/v4"
	appv1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/pkg/config"
)

func newTestCluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newTestHelmRelease(name, namespace, version string) *unstructured.Unstructured {
	hr := &unstructured.Unstructured{}
	hr.SetGroupVersionKind(helmReleaseGVK)
	hr.SetName(name)
	hr.SetNamespace(namespace)
	hr.Object["spec"] = map[string]interface{}{
		"chart": map[string]interface{}{
			"spec": map[string]interface{}{
				"chart":   "observability-bundle",
				"version": version,
			},
		},
	}
	return hr
}

func newTestApp(name, namespace, version string) *appv1.App {
	return &appv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appv1.AppSpec{
			Name:    "observability-bundle",
			Version: version,
		},
	}
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clusterv1.AddToScheme(s)
	_ = appv1.AddToScheme(s)
	return s
}

func TestConfigureBundle(t *testing.T) {
	const (
		clusterName      = "test-cluster"
		clusterNamespace = "test-ns"
		bundleName       = "test-cluster-observability-bundle"
	)

	t.Run("configures HelmRelease when present", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "1.2.3")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		err := svc.configureBundle(context.Background(), cluster)
		require.NoError(t, err)

		// Verify HelmRelease was updated with valuesFrom
		updated := &unstructured.Unstructured{}
		updated.SetGroupVersionKind(helmReleaseGVK)
		err = client.Get(context.Background(), types.NamespacedName{Name: bundleName, Namespace: clusterNamespace}, updated)
		require.NoError(t, err)

		spec := updated.Object["spec"].(map[string]interface{})
		valuesFrom := spec["valuesFrom"].([]interface{})
		require.Len(t, valuesFrom, 1)

		entry := valuesFrom[0].(map[string]interface{})
		assert.Equal(t, "ConfigMap", entry["kind"])
		assert.Equal(t, "test-cluster-observability-platform-configuration", entry["name"])
		assert.Equal(t, "values", entry["valuesKey"])
	})

	t.Run("falls back to App CR when HelmRelease not found", func(t *testing.T) {
		app := newTestApp(bundleName, clusterNamespace, "1.2.3")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(app).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		err := svc.configureBundle(context.Background(), cluster)
		require.NoError(t, err)

		// Verify App was updated with ExtraConfigs
		var updated appv1.App
		err = client.Get(context.Background(), types.NamespacedName{Name: bundleName, Namespace: clusterNamespace}, &updated)
		require.NoError(t, err)

		require.Len(t, updated.Spec.ExtraConfigs, 1)
		assert.Equal(t, "configMap", updated.Spec.ExtraConfigs[0].Kind)
		assert.Equal(t, "test-cluster-observability-platform-configuration", updated.Spec.ExtraConfigs[0].Name)
		assert.Equal(t, clusterNamespace, updated.Spec.ExtraConfigs[0].Namespace)
		assert.Equal(t, 25, updated.Spec.ExtraConfigs[0].Priority)
	})

	t.Run("returns error when neither CR exists", func(t *testing.T) {
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		err := svc.configureBundle(context.Background(), cluster)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get observability-bundle app")
	})

	t.Run("configureHelmRelease is idempotent", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "1.2.3")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		// Configure twice
		err := svc.configureBundle(context.Background(), cluster)
		require.NoError(t, err)
		err = svc.configureBundle(context.Background(), cluster)
		require.NoError(t, err)

		// Verify only one valuesFrom entry exists
		updated := &unstructured.Unstructured{}
		updated.SetGroupVersionKind(helmReleaseGVK)
		err = client.Get(context.Background(), types.NamespacedName{Name: bundleName, Namespace: clusterNamespace}, updated)
		require.NoError(t, err)

		spec := updated.Object["spec"].(map[string]interface{})
		valuesFrom := spec["valuesFrom"].([]interface{})
		assert.Len(t, valuesFrom, 1)
	})

	t.Run("preserves existing valuesFrom entries in HelmRelease", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "1.2.3")
		spec := hr.Object["spec"].(map[string]interface{})
		spec["valuesFrom"] = []interface{}{
			map[string]interface{}{
				"kind":      "Secret",
				"name":      "existing-secret",
				"valuesKey": "values",
			},
		}

		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		err := svc.configureBundle(context.Background(), cluster)
		require.NoError(t, err)

		// Verify both entries exist
		updated := &unstructured.Unstructured{}
		updated.SetGroupVersionKind(helmReleaseGVK)
		err = client.Get(context.Background(), types.NamespacedName{Name: bundleName, Namespace: clusterNamespace}, updated)
		require.NoError(t, err)

		updatedSpec := updated.Object["spec"].(map[string]interface{})
		valuesFrom := updatedSpec["valuesFrom"].([]interface{})
		assert.Len(t, valuesFrom, 2)

		// Original entry still present
		existing := valuesFrom[0].(map[string]interface{})
		assert.Equal(t, "Secret", existing["kind"])
		assert.Equal(t, "existing-secret", existing["name"])

		// New entry added
		newEntry := valuesFrom[1].(map[string]interface{})
		assert.Equal(t, "ConfigMap", newEntry["kind"])
		assert.Equal(t, "test-cluster-observability-platform-configuration", newEntry["name"])
	})
}

func TestGetObservabilityBundleAppVersion(t *testing.T) {
	const (
		clusterName      = "test-cluster"
		clusterNamespace = "test-ns"
		bundleName       = "test-cluster-observability-bundle"
	)

	t.Run("returns version from HelmRelease", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "1.2.3")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		version, err := svc.GetObservabilityBundleAppVersion(context.Background(), cluster)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("1.2.3"), version)
	})

	t.Run("falls back to App CR version when HelmRelease not found", func(t *testing.T) {
		app := newTestApp(bundleName, clusterNamespace, "2.0.0")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(app).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		version, err := svc.GetObservabilityBundleAppVersion(context.Background(), cluster)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("2.0.0"), version)
	})

	t.Run("returns error when neither CR exists", func(t *testing.T) {
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		_, err := svc.GetObservabilityBundleAppVersion(context.Background(), cluster)
		assert.Error(t, err)
	})

	t.Run("returns error for invalid version in HelmRelease", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "not-a-version")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		_, err := svc.GetObservabilityBundleAppVersion(context.Background(), cluster)
		assert.Error(t, err)
	})

	t.Run("prefers HelmRelease over App CR when both exist", func(t *testing.T) {
		hr := newTestHelmRelease(bundleName, clusterNamespace, "3.0.0")
		app := newTestApp(bundleName, clusterNamespace, "2.0.0")
		client := fake.NewClientBuilder().
			WithScheme(newScheme()).
			WithObjects(hr, app).
			Build()

		svc := NewBundleConfigurationService(client, config.Config{})
		cluster := newTestCluster(clusterName, clusterNamespace)

		version, err := svc.GetObservabilityBundleAppVersion(context.Background(), cluster)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("3.0.0"), version)
	})
}
