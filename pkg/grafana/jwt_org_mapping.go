package grafana

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const (
	// JWTOrgMappingSecretName is the name of the Secret holding the Grafana org mapping for JWT authentication.
	JWTOrgMappingSecretName = "grafana-mcp-org-mapping" // nolint:gosec // G101: Secret resource name, not a credential
	// JWTOrgMappingSecretNamespace is the namespace of the Secret holding the Grafana org mapping for JWT authentication.
	JWTOrgMappingSecretNamespace = "monitoring"
	// JWTOrgMappingSecretKey is the Secret data key holding the Grafana org mapping.
	JWTOrgMappingSecretKey = "orgMapping"
)

// ConfigureJWTOrgMappingSecret creates or updates the Secret holding the Grafana org mapping
// derived from the provided organizations. The mapping is the same string as the org_mapping
// written to the generic_oauth SSO settings.
//
// The grafana-mcp Grafana instance reads it as GF_AUTH_JWT_ORG_MAPPING. Grafana reads that
// value at startup only, so the consumer is responsible for rolling its pods when the Secret
// changes, e.g. with reloader annotations on the grafana-mcp deployment.
func ConfigureJWTOrgMappingSecret(ctx context.Context, c client.Client, organizations []*organization.Organization) error {
	logger := log.FromContext(ctx).WithValues("secret", JWTOrgMappingSecretNamespace+"/"+JWTOrgMappingSecretName, "organizations_count", len(organizations))

	orgsMapping, err := generateGrafanaOrgsMapping(organizations)
	if err != nil {
		return fmt.Errorf("failed to generate organization mappings for JWT: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTOrgMappingSecretName,
			Namespace: JWTOrgMappingSecretNamespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		secret.Labels["app.kubernetes.io/part-of"] = "observability-operator"
		secret.Labels["app.kubernetes.io/managed-by"] = "observability-operator"

		secret.Type = corev1.SecretTypeOpaque
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[JWTOrgMappingSecretKey] = []byte(orgsMapping)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update JWT org mapping secret: %w", err)
	}

	logger.Info("configured JWT org mapping secret", "result", result)
	return nil
}
