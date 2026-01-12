package agent

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestK8sConfigurationRepository_Save(t *testing.T) {
	tests := []struct {
		name         string
		config       *AgentConfiguration
		existingObjs []client.Object
		wantErr      bool
		validateFunc func(t *testing.T, c client.Client, config *AgentConfiguration)
	}{
		{
			name: "creates new ConfigMap and Secret",
			config: &AgentConfiguration{
				ClusterName:      "test-cluster",
				ClusterNamespace: "default",
				ConfigMapName:    "test-configmap",
				SecretName:       "test-secret",
				ConfigMapData: map[string]string{
					"config.yaml": "test: config",
				},
				SecretData: map[string]string{
					"MIMIR_URL":      "https://mimir.example.com",
					"MIMIR_PASSWORD": "secret123",
				},
				Labels: map[string]string{
					"app": "alloy",
				},
			},
			existingObjs: []client.Object{},
			wantErr:      false,
			validateFunc: func(t *testing.T, c client.Client, config *AgentConfiguration) {
				// Validate ConfigMap
				cm := &v1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Name:      config.ConfigMapName,
					Namespace: config.ClusterNamespace,
				}, cm)
				if err != nil {
					t.Errorf("Failed to get ConfigMap: %v", err)
				}
				if cm.Data["config.yaml"] != "test: config" {
					t.Errorf("ConfigMap data mismatch: got %v", cm.Data)
				}
				if cm.Labels["app"] != "alloy" {
					t.Errorf("ConfigMap labels mismatch: got %v", cm.Labels)
				}

				// Validate Secret
				secret := &v1.Secret{}
				err = c.Get(context.Background(), client.ObjectKey{
					Name:      config.SecretName,
					Namespace: config.ClusterNamespace,
				}, secret)
				if err != nil {
					t.Errorf("Failed to get Secret: %v", err)
				}
				// Validate secret has the "values" key with generated template data
				if _, ok := secret.Data["values"]; !ok {
					t.Errorf("Secret missing 'values' key")
				}
			},
		},
		{
			name: "updates existing ConfigMap and Secret",
			config: &AgentConfiguration{
				ClusterName:      "test-cluster",
				ClusterNamespace: "default",
				ConfigMapName:    "test-configmap",
				SecretName:       "test-secret",
				ConfigMapData: map[string]string{
					"config.yaml": "updated: config",
				},
				SecretData: map[string]string{
					"MIMIR_URL":      "https://mimir-updated.example.com",
					"MIMIR_PASSWORD": "newsecret456",
				},
				Labels: map[string]string{
					"app": "alloy-updated",
				},
			},
			existingObjs: []client.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "default",
					},
					Data: map[string]string{
						"config.yaml": "old: config",
					},
				},
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"values": []byte("old data"),
					},
				},
			},
			wantErr: false,
			validateFunc: func(t *testing.T, c client.Client, config *AgentConfiguration) {
				// Validate ConfigMap was updated
				cm := &v1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Name:      config.ConfigMapName,
					Namespace: config.ClusterNamespace,
				}, cm)
				if err != nil {
					t.Errorf("Failed to get ConfigMap: %v", err)
				}
				if cm.Data["config.yaml"] != "updated: config" {
					t.Errorf("ConfigMap data not updated: got %v", cm.Data)
				}
				if cm.Labels["app"] != "alloy-updated" {
					t.Errorf("ConfigMap labels not updated: got %v", cm.Labels)
				}

				// Validate Secret was updated
				secret := &v1.Secret{}
				err = c.Get(context.Background(), client.ObjectKey{
					Name:      config.SecretName,
					Namespace: config.ClusterNamespace,
				}, secret)
				if err != nil {
					t.Errorf("Failed to get Secret: %v", err)
				}
				// Validate secret has the "values" key with new generated data
				if _, ok := secret.Data["values"]; !ok {
					t.Errorf("Secret missing 'values' key")
				}
				if string(secret.Data["values"]) == "old data" {
					t.Errorf("Secret data not updated")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = v1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.existingObjs...).
				Build()

			repo := NewConfigurationRepository(fakeClient)

			err := repo.Save(context.Background(), tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, fakeClient, tt.config)
			}
		})
	}
}

func TestK8sConfigurationRepository_Delete(t *testing.T) {
	tests := []struct {
		name             string
		clusterName      string
		clusterNamespace string
		configMapName    string
		secretName       string
		existingObjs     []client.Object
		wantErr          bool
		validateFunc     func(t *testing.T, c client.Client)
	}{
		{
			name:             "deletes existing ConfigMap and Secret",
			clusterName:      "test-cluster",
			clusterNamespace: "default",
			configMapName:    "test-configmap",
			secretName:       "test-secret",
			existingObjs: []client.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "default",
					},
				},
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "default",
					},
				},
			},
			wantErr: false,
			validateFunc: func(t *testing.T, c client.Client) {
				// Validate ConfigMap was deleted
				cm := &v1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Name:      "test-configmap",
					Namespace: "default",
				}, cm)
				if !apierrors.IsNotFound(err) {
					t.Errorf("ConfigMap should be deleted, got error: %v", err)
				}

				// Validate Secret was deleted
				secret := &v1.Secret{}
				err = c.Get(context.Background(), client.ObjectKey{
					Name:      "test-secret",
					Namespace: "default",
				}, secret)
				if !apierrors.IsNotFound(err) {
					t.Errorf("Secret should be deleted, got error: %v", err)
				}
			},
		},
		{
			name:             "handles non-existent resources gracefully",
			clusterName:      "test-cluster",
			clusterNamespace: "default",
			configMapName:    "non-existent-configmap",
			secretName:       "non-existent-secret",
			existingObjs:     []client.Object{},
			wantErr:          false,
			validateFunc: func(t *testing.T, c client.Client) {
				// Should not error when resources don't exist
			},
		},
		{
			name:             "deletes only specified resources",
			clusterName:      "test-cluster",
			clusterNamespace: "default",
			configMapName:    "test-configmap",
			secretName:       "test-secret",
			existingObjs: []client.Object{
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-configmap",
						Namespace: "default",
					},
				},
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-configmap",
						Namespace: "default",
					},
				},
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "default",
					},
				},
			},
			wantErr: false,
			validateFunc: func(t *testing.T, c client.Client) {
				// Validate only specified ConfigMap was deleted
				cm := &v1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Name:      "test-configmap",
					Namespace: "default",
				}, cm)
				if !apierrors.IsNotFound(err) {
					t.Errorf("test-configmap should be deleted")
				}

				// Validate other ConfigMap still exists
				otherCm := &v1.ConfigMap{}
				err = c.Get(context.Background(), client.ObjectKey{
					Name:      "other-configmap",
					Namespace: "default",
				}, otherCm)
				if err != nil {
					t.Errorf("other-configmap should still exist, got error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = v1.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.existingObjs...).
				Build()

			repo := NewConfigurationRepository(fakeClient)

			err := repo.Delete(
				context.Background(),
				tt.clusterName,
				tt.clusterNamespace,
				tt.configMapName,
				tt.secretName,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, fakeClient)
			}
		})
	}
}
