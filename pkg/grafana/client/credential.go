package client

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type adminCredentials struct {
	Username string
	Password string
}

func getAdminCredentials(ctx context.Context, client client.Client) (adminCredentials, error) {
	grafanaAdminSecret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: Namespace,
		Name:      GrafanaSecretName,
	}, grafanaAdminSecret)
	if err != nil {
		return adminCredentials{}, errors.WithStack(err)
	}

	adminUser, ok := grafanaAdminSecret.Data["admin-user"]
	if !ok {
		return adminCredentials{}, fmt.Errorf("admin-user not found in secret %v/%v", grafanaAdminSecret.Namespace, grafanaAdminSecret.Name)
	}
	adminPassword, ok := grafanaAdminSecret.Data["admin-password"]
	if !ok {
		return adminCredentials{}, fmt.Errorf("admin-password not found in secret %v/%v", grafanaAdminSecret.Namespace, grafanaAdminSecret.Name)
	}

	return adminCredentials{Username: string(adminUser), Password: string(adminPassword)}, nil
}
