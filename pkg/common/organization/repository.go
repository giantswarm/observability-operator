package organization

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrOrganizationLabelMissing is returned when the organization label is not found on the cluster's namespace
	// or if the label value is empty.
	ErrOrganizationLabelMissing = errors.New("cluster namespace missing organization label or label value is empty")
	organizationLabel           = "giantswarm.io/organization"
)

// OrganizationRepository defines an interface for reading organization information.
type OrganizationRepository interface {
	Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error)
}

// NamespaceOrganizationRepository implements OrganizationRepository by reading
// the organization from the cluster's namespace labels.
type NamespaceOrganizationRepository struct {
	client.Client
}

// NewNamespaceRepository creates a new NamespaceOrganizationRepository.
func NewNamespaceRepository(client client.Client) OrganizationRepository {
	return NamespaceOrganizationRepository{Client: client}
}

// Read fetches the organization ID from the labels of the namespace
// where the given CAPI cluster resides.
// It returns ErrOrganizationLabelMissing if the label is not present or its value is empty.
func (r NamespaceOrganizationRepository) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	namespace := &corev1.Namespace{}
	key := client.ObjectKey{Name: cluster.GetNamespace()}

	if err := r.Get(ctx, key, namespace); err != nil {
		return "", fmt.Errorf("failed to get namespace: %w", err)
	}

	if namespace.Labels == nil {
		return "", ErrOrganizationLabelMissing
	}

	organization, ok := namespace.Labels[organizationLabel]
	if !ok || organization == "" {
		return "", ErrOrganizationLabelMissing
	}

	return organization, nil
}
