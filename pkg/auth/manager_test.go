package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockPasswordGenerator for testing
type mockPasswordGenerator struct{}

func (m *mockPasswordGenerator) GeneratePassword(length int) (string, error) {
	return "generated-password-32-chars-long", nil
}

func (m *mockPasswordGenerator) GenerateHtpasswd(username, password string) (string, error) {
	return username + ":$2a$10$encrypted" + password, nil
}

func TestAuthManager(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	config := NewConfig(
		AuthTypeMetrics,
		"secrets-namespace",
		"test-ingress-secret",
		"test-httproute-secret",
	)

	t.Run("NewAuthManager", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		manager := NewAuthManager(client, config)
		assert.NotNil(t, manager)
	})

	t.Run("EnsureClusterAuth", func(t *testing.T) {
		t.Run("should create new per-cluster auth secret", func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: &mockPasswordGenerator{},
				config:            config,
			}

			ctx := context.Background()
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-cluster-namespace",
				},
			}
			err := manager.EnsureClusterAuth(ctx, cluster)
			require.NoError(t, err)

			// Verify per-cluster auth secret was created
			clusterSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      "test-cluster-observability-metrics-auth",
				Namespace: "test-cluster-namespace",
			}, clusterSecret)
			require.NoError(t, err)

			// Verify password and htpasswd data
			assert.Equal(t, "generated-password-32-chars-long", string(clusterSecret.Data["password"]))
			assert.Equal(t, "test-cluster:$2a$10$encryptedgenerated-password-32-chars-long", string(clusterSecret.Data["htpasswd"]))

			// Verify labels
			assert.Equal(t, "metrics-auth", clusterSecret.Labels["app.kubernetes.io/component"])
			assert.Equal(t, "observability-operator", clusterSecret.Labels["app.kubernetes.io/part-of"])
			assert.Equal(t, "test-cluster", clusterSecret.Labels["observability.giantswarm.io/cluster"])
			assert.Equal(t, "metrics", clusterSecret.Labels["observability.giantswarm.io/auth-type"])
		})

		t.Run("should not overwrite existing cluster password", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-observability-metrics-auth",
					Namespace: "test-cluster-namespace",
					Labels: map[string]string{
						"app.kubernetes.io/component":           "metrics-auth",
						"observability.giantswarm.io/auth-type": "metrics",
					},
				},
				Data: map[string][]byte{
					"password": []byte("existing-password"),
					"htpasswd": []byte("test-cluster:$2a$10$encryptedexisting-password"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: &mockPasswordGenerator{},
				config:            config,
			}

			ctx := context.Background()
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-cluster-namespace",
				},
			}
			err := manager.EnsureClusterAuth(ctx, cluster)
			require.NoError(t, err)

			// Verify password wasn't changed
			clusterSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      "test-cluster-observability-metrics-auth",
				Namespace: "test-cluster-namespace",
			}, clusterSecret)
			require.NoError(t, err)
			assert.Equal(t, "existing-password", string(clusterSecret.Data["password"]))
		})
	})

	t.Run("DeleteClusterAuth", func(t *testing.T) {
		t.Run("should remove existing cluster auth secret", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1-observability-metrics-auth",
					Namespace: "cluster-1-namespace",
					Labels: map[string]string{
						"app.kubernetes.io/component":           "metrics-auth",
						"observability.giantswarm.io/auth-type": "metrics",
					},
				},
				Data: map[string][]byte{
					"password": []byte("password-1"),
					"htpasswd": []byte("cluster-1:$2a$10$encryptedpassword-1"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: &mockPasswordGenerator{},
				config:            config,
			}

			ctx := context.Background()
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-1",
					Namespace: "cluster-1-namespace",
				},
			}
			err := manager.DeleteClusterAuth(ctx, cluster)
			require.NoError(t, err)

			// Verify cluster secret was deleted
			clusterSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      "cluster-1-observability-metrics-auth",
				Namespace: "cluster-1-namespace",
			}, clusterSecret)
			require.Error(t, err)
			assert.True(t, errors.IsNotFound(err))
		})
	})

	t.Run("GetClusterPassword", func(t *testing.T) {
		t.Run("should return password from cluster secret", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-observability-metrics-auth",
					Namespace: "test-cluster-namespace",
				},
				Data: map[string][]byte{
					"password": []byte("test-password"),
					"htpasswd": []byte("test-cluster:$2a$10$encryptedtest-password"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: &mockPasswordGenerator{},
				config:            config,
			}

			ctx := context.Background()
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-cluster-namespace",
				},
			}
			password, err := manager.GetClusterPassword(ctx, cluster)
			require.NoError(t, err)
			assert.Equal(t, "test-password", password)
		})
	})
}
