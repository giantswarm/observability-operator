package tenancy

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func TestKubernetesTenantRepository_List(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme) // Register GrafanaOrganization with the scheme

	now := metav1.Now()

	tests := []struct {
		name          string
		initialObjs   []client.Object
		expected      []string
		expectErr     bool
		expectedError string
	}{
		{
			name:        "case 0: no GrafanaOrganizations",
			initialObjs: []client.Object{},
			expected:    []string{},
			expectErr:   false,
		},
		{
			name: "case 1: single GrafanaOrganization with tenants",
			initialObjs: []client.Object{
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org1", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-a", "tenant-b"},
					},
				},
			},
			expected:  []string{"tenant-a", "tenant-b"},
			expectErr: false,
		},
		{
			name: "case 2: multiple GrafanaOrganizations with unique and overlapping tenants",
			initialObjs: []client.Object{
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org1", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-a", "tenant-b"},
					},
				},
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org2", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-b", "tenant-c"},
					},
				},
			},
			expected:  []string{"tenant-a", "tenant-b", "tenant-c"},
			expectErr: false,
		},
		{
			name: "case 3: GrafanaOrganization marked for deletion",
			initialObjs: []client.Object{
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org1", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-a"},
					},
				},
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "org-deleted",
						Namespace:         "default",
						DeletionTimestamp: &now,
						Finalizers:        []string{"finalizer.grafana.giantswarm.io"},
					},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-deleted"},
					},
				},
			},
			expected:  []string{"tenant-a"},
			expectErr: false,
		},
		{
			name: "case 4: GrafanaOrganization with no tenants",
			initialObjs: []client.Object{
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org1", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{},
					},
				},
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org2", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"tenant-x"},
					},
				},
			},
			expected:  []string{"tenant-x"},
			expectErr: false,
		},
		{
			name: "case 5: unsorted tenants in spec, ensure output is sorted",
			initialObjs: []client.Object{
				&v1alpha1.GrafanaOrganization{
					ObjectMeta: metav1.ObjectMeta{Name: "org1", Namespace: "default"},
					Spec: v1alpha1.GrafanaOrganizationSpec{
						Tenants: []v1alpha1.TenantID{"zebra", "apple", "banana"},
					},
				},
			},
			expected:  []string{"apple", "banana", "zebra"},
			expectErr: false,
		},
		// Note: Testing k8sClient.List errors with the fake client is non-trivial
		// as it doesn't easily simulate network or API server errors without custom reactors.
		// For this example, we focus on the logic after a successful List.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjs...).Build()
			repo := NewKubernetesRepository(fakeClient)

			result, err := repo.List(context.Background())

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error message '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("Expected tenants %v, but got %v", tt.expected, result)
				}
			}
		})
	}
}
