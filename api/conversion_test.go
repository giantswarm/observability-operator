package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/api/v1alpha2"
)

func TestConversion_V1Alpha1_To_V1Alpha2(t *testing.T) {
	// Create a v1alpha1 GrafanaOrganization
	v1alpha1Org := &v1alpha1.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1alpha1.GrafanaOrganizationSpec{
			DisplayName: "Test Organization",
			RBAC: &v1alpha1.RBAC{
				Admins: []string{"admin@example.com"},
			},
			Tenants: []v1alpha1.TenantID{"tenant1", "tenant2"},
		},
		Status: v1alpha1.GrafanaOrganizationStatus{
			OrgID: 123,
		},
	}

	// Convert to v1alpha2 (Hub)
	v1alpha2Org := &v1alpha2.GrafanaOrganization{}
	err := v1alpha1Org.ConvertTo(v1alpha2Org)
	require.NoError(t, err)

	// Verify conversion
	assert.Equal(t, "test-org", v1alpha2Org.Name)
	assert.Equal(t, "Test Organization", v1alpha2Org.Spec.DisplayName)
	assert.Equal(t, []string{"admin@example.com"}, v1alpha2Org.Spec.RBAC.Admins)
	assert.Equal(t, int64(123), v1alpha2Org.Status.OrgID)

	// Verify tenant conversion - should have both types by default
	require.Len(t, v1alpha2Org.Spec.Tenants, 2)

	assert.Equal(t, v1alpha2.TenantID("tenant1"), v1alpha2Org.Spec.Tenants[0].Name)
	assert.Equal(t, []v1alpha2.TenantType{v1alpha2.TenantTypeData, v1alpha2.TenantTypeAlerting}, v1alpha2Org.Spec.Tenants[0].Types)

	assert.Equal(t, v1alpha2.TenantID("tenant2"), v1alpha2Org.Spec.Tenants[1].Name)
	assert.Equal(t, []v1alpha2.TenantType{v1alpha2.TenantTypeData, v1alpha2.TenantTypeAlerting}, v1alpha2Org.Spec.Tenants[1].Types)
}

func TestConversion_V1Alpha2_To_V1Alpha1(t *testing.T) {
	// Create a v1alpha2 GrafanaOrganization
	v1alpha2Org := &v1alpha2.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
		},
		Spec: v1alpha2.GrafanaOrganizationSpec{
			DisplayName: "Test Organization",
			RBAC: &v1alpha2.RBAC{
				Admins: []string{"admin@example.com"},
			},
			Tenants: []v1alpha2.TenantConfig{
				{
					Name:  "tenant1",
					Types: []v1alpha2.TenantType{v1alpha2.TenantTypeData, v1alpha2.TenantTypeAlerting},
				},
				{
					Name:  "tenant2",
					Types: []v1alpha2.TenantType{v1alpha2.TenantTypeData}, // Only data access
				},
			},
		},
		Status: v1alpha2.GrafanaOrganizationStatus{
			OrgID: 456,
		},
	}

	// Convert to v1alpha1
	v1alpha1Org := &v1alpha1.GrafanaOrganization{}
	err := v1alpha1Org.ConvertFrom(v1alpha2Org)
	require.NoError(t, err)

	// Verify conversion
	assert.Equal(t, "test-org", v1alpha1Org.Name)
	assert.Equal(t, "Test Organization", v1alpha1Org.Spec.DisplayName)
	assert.Equal(t, []string{"admin@example.com"}, v1alpha1Org.Spec.RBAC.Admins)
	assert.Equal(t, int64(456), v1alpha1Org.Status.OrgID)

	// Verify tenant conversion - should lose type information
	require.Len(t, v1alpha1Org.Spec.Tenants, 2)
	assert.Equal(t, v1alpha1.TenantID("tenant1"), v1alpha1Org.Spec.Tenants[0])
	assert.Equal(t, v1alpha1.TenantID("tenant2"), v1alpha1Org.Spec.Tenants[1])
}

func TestConversion_RoundTrip(t *testing.T) {
	// Create original v1alpha1 resource
	original := &v1alpha1.GrafanaOrganization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-org",
			Namespace: "test-ns",
		},
		Spec: v1alpha1.GrafanaOrganizationSpec{
			DisplayName: "Test Organization",
			RBAC: &v1alpha1.RBAC{
				Admins:  []string{"admin@example.com"},
				Editors: []string{"editor@example.com"},
				Viewers: []string{"viewer@example.com"},
			},
			Tenants: []v1alpha1.TenantID{"tenant1", "tenant2", "tenant3"},
		},
		Status: v1alpha1.GrafanaOrganizationStatus{
			OrgID: 789,
			DataSources: []v1alpha1.DataSource{
				{ID: 1, Name: "prometheus"},
				{ID: 2, Name: "loki"},
			},
		},
	}

	// Convert v1alpha1 -> v1alpha2 -> v1alpha1
	hub := &v1alpha2.GrafanaOrganization{}
	err := original.ConvertTo(hub)
	require.NoError(t, err)

	roundTrip := &v1alpha1.GrafanaOrganization{}
	err = roundTrip.ConvertFrom(hub)
	require.NoError(t, err)

	// Verify round-trip preserves basic data (types information is lost as expected)
	assert.Equal(t, original.Name, roundTrip.Name)
	assert.Equal(t, original.Namespace, roundTrip.Namespace)
	assert.Equal(t, original.Spec.DisplayName, roundTrip.Spec.DisplayName)
	assert.Equal(t, original.Spec.RBAC.Admins, roundTrip.Spec.RBAC.Admins)
	assert.Equal(t, original.Spec.RBAC.Editors, roundTrip.Spec.RBAC.Editors)
	assert.Equal(t, original.Spec.RBAC.Viewers, roundTrip.Spec.RBAC.Viewers)
	assert.Equal(t, original.Spec.Tenants, roundTrip.Spec.Tenants)
	assert.Equal(t, original.Status.OrgID, roundTrip.Status.OrgID)
	assert.Equal(t, original.Status.DataSources, roundTrip.Status.DataSources)
}
