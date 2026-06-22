package credential

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// ErrCredentialNotReady signals that the AgentCredential CR exists but the
// backing Secret has not been rendered yet. Callers should treat this as a
// transient state worthy of a short requeue rather than a hard error. The
// AgentCredentialReconciler produces the Secret asynchronously.
var ErrCredentialNotReady = errors.New("agent credential secret not yet rendered")

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

// BasicAuth holds a resolved basic-auth pair.
type BasicAuth struct {
	Username string
	Password string
}

// BackendCredentials is a per-backend bag of resolved basic-auth pairs. The
// cluster controller resolves it once per reconcile and passes it into each
// collector so the rendering layer stays free of credential-store I/O.
type BackendCredentials map[observabilityv1alpha1.CredentialBackend]BasicAuth

// Get returns the credentials for the given backend plus a boolean indicating
// whether they were present.
func (c BackendCredentials) Get(backend observabilityv1alpha1.CredentialBackend) (BasicAuth, bool) {
	auth, ok := c[backend]
	return auth, ok
}

// Reader reads basic-auth credentials from the Secrets backing AgentCredentials.
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
		if apierrors.IsNotFound(err) {
			return "", "", fmt.Errorf("%w: %s/%s", ErrCredentialNotReady, namespace, secretName)
		}
		return "", "", fmt.Errorf("failed to get agent credential secret %s/%s: %w", namespace, secretName, err)
	}

	username := string(secret.Data[SecretKeyUsername])
	password := string(secret.Data[SecretKeyPassword])
	if username == "" || password == "" {
		return "", "", fmt.Errorf("%w: %s/%s missing basic-auth data", ErrCredentialNotReady, namespace, secretName)
	}
	return username, password, nil
}
