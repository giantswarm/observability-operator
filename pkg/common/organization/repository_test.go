package organization

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNamespaceOrganizationRepository_Read(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	testOrgLabelValue := "my-org"

	tests := []struct {
		name           string
		cluster        *clusterv1.Cluster
		initialObjects []client.Object
		expectedOrg    string
		expectedError  error
	}{
		{
			name: "case 0: success - label found",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-ns"},
			},
			initialObjects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
						Labels: map[string]string{
							organizationLabel: testOrgLabelValue,
						},
					},
				},
			},
			expectedOrg:   testOrgLabelValue,
			expectedError: nil,
		},
		{
			name: "case 1: error - namespace not found",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "non-existent-ns"},
			},
			initialObjects: []client.Object{},
			expectedOrg:    "",
			// The actual error from fake client for "not found" might vary slightly in text
			// but it will be a "not found" type error.
			expectedError: errors.New("namespaces \"non-existent-ns\" not found"),
		},
		{
			name: "case 2: error - label missing",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-ns-no-label"},
			},
			initialObjects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test-ns-no-label",
						Labels: map[string]string{"other-label": "value"},
					},
				},
			},
			expectedOrg:   "",
			expectedError: ErrOrganizationLabelMissing,
		},
		{
			name: "case 3: success - label found with other labels present",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-ns-many-labels"},
			},
			initialObjects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns-many-labels",
						Labels: map[string]string{
							"other-label":     "value",
							organizationLabel: testOrgLabelValue,
							"another-label":   "another-value",
						},
					},
				},
			},
			expectedOrg:   testOrgLabelValue,
			expectedError: nil,
		},
		{
			name: "case 4: error - label present but empty string", // Updated expectation
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-ns-empty-label"},
			},
			initialObjects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns-empty-label",
						Labels: map[string]string{
							organizationLabel: "", // Empty value
						},
					},
				},
			},
			expectedOrg:   "",
			expectedError: ErrOrganizationLabelMissing, // Now expects ErrOrganizationLabelMissing
		},
		{
			name: "case 5: error - namespace has no labels map (nil labels)",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "test-ns-nil-labels"},
			},
			initialObjects: []client.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns-nil-labels",
						// Labels map is nil by default if not specified
					},
				},
			},
			expectedOrg:   "",
			expectedError: ErrOrganizationLabelMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initialObjects...).Build()
			// Use NewNamespaceRepository which returns the interface type
			repo := NewNamespaceRepository(fakeClient)

			org, err := repo.Read(context.Background(), tt.cluster)

			if org != tt.expectedOrg {
				t.Errorf("Expected organization %q, got %q", tt.expectedOrg, org)
			}

			if tt.expectedError == nil && err != nil {
				t.Errorf("Expected no error, got %v", err)
			} else if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if !errors.Is(err, tt.expectedError) && err.Error() != tt.expectedError.Error() {
					// Using errors.Is for our defined error, and string comparison for others (like k8s client errors)
					t.Errorf("Expected error %v (or string match for client errors), got %v", tt.expectedError, err)
				}
			}
		})
	}
}
