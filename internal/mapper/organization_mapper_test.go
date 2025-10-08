package mapper

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
)

func TestNewOrganizationMapper(t *testing.T) {
	mapper := NewOrganizationMapper()
	if mapper == nil {
		t.Error("Expected NewOrganizationMapper to return a non-nil mapper")
	}
}

func TestFromGrafanaOrganization(t *testing.T) {
	mapper := NewOrganizationMapper()

	tests := []struct {
		name           string
		grafanaOrg     *v1alpha1.GrafanaOrganization
		expectedOrgID  int64
		expectedName   string
		expectedTenants int
	}{
		{
			name: "regular organization",
			grafanaOrg: &v1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-org",
				},
				Spec: v1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Test Organization",
					Tenants:     []v1alpha1.TenantID{"tenant1", "tenant2"},
					RBAC: &v1alpha1.RBAC{
						Admins:  []string{"admin1"},
						Editors: []string{"editor1"},
						Viewers: []string{"viewer1"},
					},
				},
				Status: v1alpha1.GrafanaOrganizationStatus{
					OrgID: 42,
				},
			},
			expectedOrgID:   42,
			expectedName:    "Test Organization",
			expectedTenants: 2,
		},
		{
			name: "shared organization special case",
			grafanaOrg: &v1alpha1.GrafanaOrganization{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shared-org",
				},
				Spec: v1alpha1.GrafanaOrganizationSpec{
					DisplayName: "Shared Org",
					Tenants:     []v1alpha1.TenantID{"shared"},
					RBAC: &v1alpha1.RBAC{
						Admins:  []string{"admin1"},
						Editors: []string{"editor1"},
						Viewers: []string{"viewer1"},
					},
				},
				Status: v1alpha1.GrafanaOrganizationStatus{
					OrgID: 99, // Should be overridden to 1
				},
			},
			expectedOrgID:   1, // Special case: Shared Org always gets ID 1
			expectedName:    "Shared Org",
			expectedTenants: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domainOrg := mapper.FromGrafanaOrganization(tt.grafanaOrg)

			if domainOrg.ID() != tt.expectedOrgID {
				t.Errorf("Expected OrgID %d, got %d", tt.expectedOrgID, domainOrg.ID())
			}

			if domainOrg.Name() != tt.expectedName {
				t.Errorf("Expected Name %s, got %s", tt.expectedName, domainOrg.Name())
			}

			if len(domainOrg.TenantIDs()) != tt.expectedTenants {
				t.Errorf("Expected %d tenants, got %d", tt.expectedTenants, len(domainOrg.TenantIDs()))
			}

			// Verify tenant conversion
			for i, tenant := range tt.grafanaOrg.Spec.Tenants {
				if domainOrg.TenantIDs()[i] != string(tenant) {
					t.Errorf("Expected tenant %s, got %s", string(tenant), domainOrg.TenantIDs()[i])
				}
			}

			// Verify RBAC fields
			if len(domainOrg.Admins()) != len(tt.grafanaOrg.Spec.RBAC.Admins) {
				t.Errorf("Expected %d admins, got %d", len(tt.grafanaOrg.Spec.RBAC.Admins), len(domainOrg.Admins()))
			}
			if len(domainOrg.Editors()) != len(tt.grafanaOrg.Spec.RBAC.Editors) {
				t.Errorf("Expected %d editors, got %d", len(tt.grafanaOrg.Spec.RBAC.Editors), len(domainOrg.Editors()))
			}
			if len(domainOrg.Viewers()) != len(tt.grafanaOrg.Spec.RBAC.Viewers) {
				t.Errorf("Expected %d viewers, got %d", len(tt.grafanaOrg.Spec.RBAC.Viewers), len(domainOrg.Viewers()))
			}
		})
	}
}
