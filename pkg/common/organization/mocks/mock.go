package mocks

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// MockOrganizationRepository implements a minimal OrganizationRepository with call tracking.
// It can be used in tests to avoid complex setup of real organization repositories.
type MockOrganizationRepository struct {
	CallCount      int
	LastCluster    *clusterv1.Cluster
	OrganizationID string
}

// NewMockOrganizationRepository creates a new mock with a default organization ID.
func NewMockOrganizationRepository(organizationID string) *MockOrganizationRepository {
	return &MockOrganizationRepository{
		OrganizationID: organizationID,
	}
}

// Read returns the configured organization ID and tracks the call.
func (m *MockOrganizationRepository) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	m.CallCount++
	m.LastCluster = cluster
	return m.OrganizationID, nil
}
