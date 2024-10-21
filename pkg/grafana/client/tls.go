package client

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// buildTLSConfiguration builds the tls.Config object based on the content of the grafana-tls secret
func buildTLSConfiguration(ctx context.Context, client client.Client) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}

	secret := &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: grafanaNamespace,
		Name:      grafanaTLSSecretName,
	}, secret)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("empty tls secret: %v/%v", secret.Namespace, secret.Name)
	}

	crt, crtPresent := secret.Data["tls.crt"]
	key, keyPresent := secret.Data["tls.key"]

	if (crtPresent && !keyPresent) || (keyPresent && !crtPresent) {
		return nil, fmt.Errorf("invalid secret %v/%v. tls.crt and tls.key needs to be present together when one of them is declared", secret.Namespace, secret.Name)
	} else if crtPresent && keyPresent {
		loadedCrt, err := tls.X509KeyPair(crt, key)
		if err != nil {
			return nil, fmt.Errorf("certificate from secret %v/%v cannot be parsed : %w", secret.Namespace, secret.Name, err)
		}
		tlsConfig.Certificates = []tls.Certificate{loadedCrt}
	}

	return tlsConfig, nil
}
