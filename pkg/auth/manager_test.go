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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// mockPasswordGenerator for testing
type mockPasswordGenerator struct {
	passwords map[string]string
	htpasswds map[string]string
}

func newMockPasswordGenerator() *mockPasswordGenerator {
	return &mockPasswordGenerator{
		passwords: make(map[string]string),
		htpasswds: make(map[string]string),
	}
}

func (m *mockPasswordGenerator) GeneratePassword(length int) (string, error) {
	password := "generated-password-32-chars-long"
	if length == 16 {
		password = "generated-pwd-16"
	}
	return password, nil
}

func (m *mockPasswordGenerator) GenerateHtpasswd(username, password string) (string, error) {
	return username + ":$2a$10$encrypted" + password, nil
}

func TestAuthManager(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	config := NewConfig(
		"test-auth-secret",
		"auth-namespace",
		"secrets-namespace",
		"test-ingress-secret",
		"test-httproute-secret",
	)

	t.Run("NewAuthManager", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		manager := NewAuthManager(client, config)
		assert.NotNil(t, manager)
	})

	t.Run("AddClusterPassword", func(t *testing.T) {
		t.Run("should create new auth secret and add cluster password", func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			err := manager.AddClusterPassword(ctx, "test-cluster")
			require.NoError(t, err)

			// Verify auth secret was created
			authSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			require.NoError(t, err)

			// Verify cluster password was added
			assert.Contains(t, authSecret.Data, "test-cluster")
			assert.Equal(t, "generated-password-32-chars-long", string(authSecret.Data["test-cluster"]))
		})

		t.Run("should not overwrite existing cluster password", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"test-cluster": []byte("existing-password"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			err := manager.AddClusterPassword(ctx, "test-cluster")
			require.NoError(t, err)

			// Verify password wasn't changed
			authSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			require.NoError(t, err)

			assert.Equal(t, "existing-password", string(authSecret.Data["test-cluster"]))
		})

		t.Run("should add multiple cluster passwords", func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()

			// Add first cluster
			err := manager.AddClusterPassword(ctx, "cluster-1")
			require.NoError(t, err)

			// Add second cluster
			err = manager.AddClusterPassword(ctx, "cluster-2")
			require.NoError(t, err)

			// Verify both passwords exist
			authSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			require.NoError(t, err)

			assert.Contains(t, authSecret.Data, "cluster-1")
			assert.Contains(t, authSecret.Data, "cluster-2")
		})
	})

	t.Run("RemoveClusterPassword", func(t *testing.T) {
		t.Run("should remove existing cluster password", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1": []byte("password-1"),
					"cluster-2": []byte("password-2"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			err := manager.RemoveClusterPassword(ctx, "cluster-1")
			require.NoError(t, err)

			// Verify cluster-1 was removed but cluster-2 remains
			authSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			require.NoError(t, err)

			assert.NotContains(t, authSecret.Data, "cluster-1")
			assert.Contains(t, authSecret.Data, "cluster-2")
		})

		t.Run("should handle non-existent cluster password gracefully", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1": []byte("password-1"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			err := manager.RemoveClusterPassword(ctx, "non-existent-cluster")
			require.NoError(t, err)

			// Verify existing data is unchanged
			authSecret := &corev1.Secret{}
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			require.NoError(t, err)

			assert.Contains(t, authSecret.Data, "cluster-1")
		})
	})

	t.Run("DeleteAllSecrets", func(t *testing.T) {
		t.Run("should delete all managed secrets", func(t *testing.T) {
			authSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
			}
			ingressSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.IngressSecretName,
					Namespace: config.SecretsNamespace,
				},
			}
			httprouteSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.HTTPRouteSecretName,
					Namespace: config.SecretsNamespace,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				authSecret, ingressSecret, httprouteSecret,
			).Build()

			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			err := manager.DeleteAllSecrets(ctx)
			require.NoError(t, err)

			// Verify all secrets are deleted
			err = client.Get(ctx, types.NamespacedName{
				Name:      config.AuthSecretName,
				Namespace: config.AuthSecretNamespace,
			}, authSecret)
			assert.True(t, errors.IsNotFound(err))

			err = client.Get(ctx, types.NamespacedName{
				Name:      config.IngressSecretName,
				Namespace: config.SecretsNamespace,
			}, ingressSecret)
			assert.True(t, errors.IsNotFound(err))

			err = client.Get(ctx, types.NamespacedName{
				Name:      config.HTTPRouteSecretName,
				Namespace: config.SecretsNamespace,
			}, httprouteSecret)
			assert.True(t, errors.IsNotFound(err))
		})
	})

	t.Run("getAllClusterPasswords", func(t *testing.T) {
		t.Run("should return all cluster passwords", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1":   []byte("password-1"),
					"cluster-2":   []byte("password-2"),
					"credentials": []byte("legacy-password"), // should be filtered out
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			passwords, err := manager.getAllClusterPasswords(ctx)
			require.NoError(t, err)

			expected := map[string]string{
				"cluster-1": "password-1",
				"cluster-2": "password-2",
			}
			assert.Equal(t, expected, passwords)
		})

		t.Run("should handle empty secret", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			passwords, err := manager.getAllClusterPasswords(ctx)
			require.NoError(t, err)

			assert.Empty(t, passwords)
		})
	})

	t.Run("generateHtpasswdContent", func(t *testing.T) {
		t.Run("should generate htpasswd content for all clusters", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1": []byte("password-1"),
					"cluster-2": []byte("password-2"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:            config,
			}

			ctx := context.Background()
			content, err := manager.generateHtpasswdContent(ctx)
			require.NoError(t, err)

			// Should contain htpasswd entries for both clusters
			assert.Contains(t, content, "cluster-1:$2a$10$encryptedpassword-1")
			assert.Contains(t, content, "cluster-2:$2a$10$encryptedpassword-2")
		})
	})
	t.Run("GetClusterPassword", func(t *testing.T) {
		t.Run("should return existing cluster password", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1": []byte("password-1"),
					"cluster-2": []byte("password-2"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:           config,
			}

			ctx := context.Background()
			password, err := manager.GetClusterPassword(ctx, "cluster-1")
			require.NoError(t, err)
			assert.Equal(t, "password-1", password)
		})

		t.Run("should return error for non-existent cluster", func(t *testing.T) {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.AuthSecretName,
					Namespace: config.AuthSecretNamespace,
				},
				Data: map[string][]byte{
					"cluster-1": []byte("password-1"),
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:           config,
			}

			ctx := context.Background()
			_, err := manager.GetClusterPassword(ctx, "non-existent-cluster")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "password not found for cluster non-existent-cluster")
		})

		t.Run("should return error when secret does not exist", func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			manager := &authManager{
				client:            client,
				passwordGenerator: newMockPasswordGenerator(),
				config:           config,
			}

			ctx := context.Background()
			_, err := manager.GetClusterPassword(ctx, "cluster-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to get auth secret")
		})
	})
}
