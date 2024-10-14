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
		Namespace: grafanaNamespace,
		Name:      grafanaAdminCredentialsSecretName,
	}, grafanaAdminSecret)
	if err != nil {
		return adminCredentials{}, errors.WithStack(err)
	}

	if grafanaAdminSecret.Data == nil {
		return adminCredentials{}, fmt.Errorf("empty credential secret: %v/%v", grafanaAdminSecret.Namespace, grafanaAdminSecret.Name)
	}

	adminUser, userPresent := grafanaAdminSecret.Data["admin-user"]
	adminPassword, passwordPresent := grafanaAdminSecret.Data["admin-password"]

	if (userPresent && !passwordPresent) || (!userPresent && passwordPresent) {
		return adminCredentials{}, fmt.Errorf("invalid secret %v/%v. admin-secret and admin-user needs to be present together when one of them is declared", grafanaAdminSecret.Namespace, grafanaAdminSecret.Name)
	} else if userPresent && passwordPresent {
		return adminCredentials{Username: string(adminUser), Password: string(adminPassword)}, nil
	}
}
