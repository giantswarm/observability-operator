package credential

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// Aggregator rebuilds the per-backend gateway htpasswd Secrets by concatenating
// entries from every AgentCredential matching a given backend.
type Aggregator struct {
	Client         client.Client
	GatewayConfigs GatewayConfigs
}

// NewAggregator builds an Aggregator.
func NewAggregator(c client.Client, configs GatewayConfigs) *Aggregator {
	return &Aggregator{Client: c, GatewayConfigs: configs}
}

// Aggregate rewrites both gateway secrets (ingress and HTTPRoute) for the given
// backend to reflect the current set of AgentCredentials.
func (a *Aggregator) Aggregate(ctx context.Context, backend observabilityv1alpha1.CredentialBackend) error {
	cfg, ok := a.GatewayConfigs[backend]
	if !ok {
		return fmt.Errorf("no gateway configuration for backend %q", backend)
	}

	content, err := a.buildHtpasswdContent(ctx, backend)
	if err != nil {
		return fmt.Errorf("failed to build htpasswd content for backend %q: %w", backend, err)
	}

	if err := a.writeGatewaySecret(ctx, cfg.Namespace, cfg.IngressSecretName, cfg.IngressDataKey, content); err != nil {
		return fmt.Errorf("failed to update ingress gateway secret: %w", err)
	}

	if err := a.writeGatewaySecret(ctx, cfg.Namespace, cfg.HTTPRouteSecretName, cfg.HTTPRouteDataKey, content); err != nil {
		return fmt.Errorf("failed to update HTTPRoute gateway secret: %w", err)
	}

	return nil
}

// buildHtpasswdContent collects every htpasswd entry from AgentCredentials for
// the given backend and returns the aggregated, deterministic htpasswd content.
// CRs being deleted are ignored so the gateway drops them immediately.
func (a *Aggregator) buildHtpasswdContent(ctx context.Context, backend observabilityv1alpha1.CredentialBackend) (string, error) {
	credentials := &observabilityv1alpha1.AgentCredentialList{}
	if err := a.Client.List(ctx, credentials); err != nil {
		return "", fmt.Errorf("failed to list agent credentials: %w", err)
	}

	var entries []string
	for i := range credentials.Items {
		cred := &credentials.Items[i]
		if cred.Spec.Backend != backend {
			continue
		}
		if !cred.DeletionTimestamp.IsZero() {
			continue
		}

		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: cred.Namespace, Name: SecretName(cred)}
		if err := a.Client.Get(ctx, key, secret); err != nil {
			if apierrors.IsNotFound(err) {
				// Secret hasn't been rendered yet; skip it — the owning reconcile
				// will aggregate again once it exists.
				continue
			}
			return "", fmt.Errorf("failed to get secret %s: %w", key, err)
		}

		htpasswd, ok := secret.Data[SecretKeyHtpasswd]
		if !ok || len(htpasswd) == 0 {
			continue
		}
		entries = append(entries, string(htpasswd))
	}

	sort.Strings(entries)
	return strings.Join(entries, "\n"), nil
}

func (a *Aggregator) writeGatewaySecret(ctx context.Context, namespace, name, dataKey, content string) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, a.Client, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[dataKey] = []byte(content)
		return nil
	})
	if err != nil {
		// Namespace may be missing on a fresh install or a tear-down path; do
		// not fail the reconcile in that case.
		if apierrors.IsNotFound(err) {
			logger.Info("gateway namespace not found, skipping", "namespace", namespace, "secret", name)
			return nil
		}
		return fmt.Errorf("failed to upsert gateway secret %s/%s: %w", namespace, name, err)
	}

	logger.Info("gateway secret processed", "namespace", namespace, "secret", name, "result", result)
	return nil
}
