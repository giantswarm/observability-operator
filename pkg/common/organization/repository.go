package organization

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OrganizationLabel string = "giantswarm.io/organization"
)

type OrganizationRepository interface {
	Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error)
}

type NamespaceOrganizationRepository struct {
	client.Client
}

func NewNamespaceRepository(client client.Client) OrganizationRepository {
	return NamespaceOrganizationRepository{client}
}

func (r NamespaceOrganizationRepository) Read(ctx context.Context, cluster *clusterv1.Cluster) (string, error) {
	namespace := &corev1.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: cluster.GetNamespace()}, namespace)
	if err != nil {
		return "", err
	}

	if organization, ok := namespace.Labels[OrganizationLabel]; ok {
		return organization, nil
	}
	return "", errors.New("cluster namespace missing organization label")
}
