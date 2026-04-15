package credential

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// SecretName returns the name of the Secret for the given AgentCredential.
// Defaults to the CR's name when spec.secretName is empty.
func SecretName(cred *observabilityv1alpha1.AgentCredential) string {
	if cred.Spec.SecretName != "" {
		return cred.Spec.SecretName
	}
	return cred.Name
}

// Renderer creates or updates the per-credential basic-auth Secret backing
// an AgentCredential.
type Renderer struct {
	Client            client.Client
	PasswordGenerator PasswordGenerator
}

// NewRenderer builds a Renderer with the default password generator.
func NewRenderer(c client.Client) *Renderer {
	return &Renderer{
		Client:            c,
		PasswordGenerator: NewPasswordGenerator(),
	}
}

// Render creates or updates the basic-auth Secret for the given AgentCredential.
// Returns the rendered Secret so callers can update status.
func (r *Renderer) Render(ctx context.Context, cred *observabilityv1alpha1.AgentCredential) (*corev1.Secret, error) {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(cred),
			Namespace: cred.Namespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(cred, secret, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		secret.Labels["app.kubernetes.io/part-of"] = "observability-operator"
		secret.Labels["app.kubernetes.io/managed-by"] = "observability-operator"

		secret.Type = corev1.SecretTypeOpaque
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		// Preserve existing password across reconciles; only generate a new
		// one when missing. This also adopts Secrets pre-existing from the
		// old auth manager without rotating the password.
		password := string(secret.Data[SecretKeyPassword])
		if password == "" {
			newPassword, err := r.PasswordGenerator.GeneratePassword(32)
			if err != nil {
				return fmt.Errorf("failed to generate password: %w", err)
			}
			password = newPassword
		}

		htpasswdEntry, err := r.PasswordGenerator.GenerateHtpasswd(cred.Spec.AgentName, password)
		if err != nil {
			return fmt.Errorf("failed to generate htpasswd: %w", err)
		}

		secret.Data[SecretKeyUsername] = []byte(cred.Spec.AgentName)
		secret.Data[SecretKeyPassword] = []byte(password)
		secret.Data[SecretKeyHtpasswd] = []byte(htpasswdEntry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to render agent credential secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	logger.Info("agent credential secret processed", "secret", secret.Name, "result", result)
	return secret, nil
}
