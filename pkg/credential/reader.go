package credential

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// ClusterCredentialName returns the AgentCredential name used by the cluster
// controller for the given cluster and backend. Kept here so collectors and
// the cluster controller agree on the naming scheme.
func ClusterCredentialName(clusterName string, backend observabilityv1alpha1.CredentialBackend) string {
	return fmt.Sprintf("%s-observability-%s", clusterName, backend)
}

// ClusterSecretName returns the Secret name used by the cluster controller for
// the given cluster and backend. The "-auth" suffix matches the pre-CRD legacy
// Secret name, so existing Alloy collectors keep their references unchanged.
func ClusterSecretName(clusterName string, backend observabilityv1alpha1.CredentialBackend) string {
	return fmt.Sprintf("%s-auth", ClusterCredentialName(clusterName, backend))
}

// Reader reads basic-auth credentials from the Secrets backing AgentCredentials.
// Collectors use it to embed the password into the Alloy agent configuration.
type Reader interface {
	// ReadPassword returns (username, password) for the AgentCredential with
	// the given name in the given namespace.
	ReadPassword(ctx context.Context, namespace, credentialName string) (username, password string, err error)
}

type reader struct {
	client client.Client
}

// NewReader returns a Reader backed by a controller-runtime client.
func NewReader(c client.Client) Reader {
	return &reader{client: c}
}

func (r *reader) ReadPassword(ctx context.Context, namespace, credentialName string) (string, string, error) {
	cred := &observabilityv1alpha1.AgentCredential{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: credentialName}, cred); err != nil {
		return "", "", fmt.Errorf("failed to get agent credential %s/%s: %w", namespace, credentialName, err)
	}

	secretName := SecretName(cred)
	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret); err != nil {
		return "", "", fmt.Errorf("failed to get agent credential secret %s/%s: %w", namespace, secretName, err)
	}

	username := string(secret.Data[SecretKeyUsername])
	password := string(secret.Data[SecretKeyPassword])
	if username == "" {
		return "", "", fmt.Errorf("username not found in agent credential secret %s/%s", namespace, secretName)
	}
	if password == "" {
		return "", "", fmt.Errorf("password not found in agent credential secret %s/%s", namespace, secretName)
	}
	return username, password, nil
}
