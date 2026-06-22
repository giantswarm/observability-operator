package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/observability-operator/pkg/config"
)

// ReadCABundle reads the CA certificate PEM from the configured Secret.
// Returns an empty string on public-CA installations where CASecretName is empty,
// in which case Alloy uses the system trust store.
func ReadCABundle(ctx context.Context, c client.Client, clusterCfg config.ClusterConfig) (string, error) {
	if clusterCfg.CASecretName == "" {
		return "", nil
	}
	caSecret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: clusterCfg.CASecretNamespace,
		Name:      clusterCfg.CASecretName,
	}, caSecret); err != nil {
		return "", fmt.Errorf("failed to read CA secret %s/%s: %w",
			clusterCfg.CASecretNamespace, clusterCfg.CASecretName, err)
	}
	return string(caSecret.Data["tls.crt"]), nil
}
