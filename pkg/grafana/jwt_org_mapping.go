package grafana

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	// JWTOrgMappingDeploymentName is the name of the Deployment of the Grafana instance reading the
	// JWT org mapping. It runs in the namespace of the Secret.
	JWTOrgMappingDeploymentName = "grafana-mcp"
	// JWTOrgMappingChecksumAnnotation is the pod template annotation holding the sha256 of the JWT
	// org mapping. A new value rolls the Deployment's pods.
	JWTOrgMappingChecksumAnnotation = "observability.giantswarm.io/org-mapping-checksum"
)

// ConfigureJWTOrgMappingSecret creates or updates the Secret holding the Grafana org mapping
// derived from the provided organizations. The mapping is the same string as the org_mapping
// written to the generic_oauth SSO settings. The organizations are sorted by ID, then name, so
// the Secret only changes when the mapping does.
//
// The grafana-mcp Grafana instance reads it as GF_AUTH_JWT_ORG_MAPPING. Grafana reads that
// value at startup only, so the sha256 of the mapping is then set on the pod template of the
// grafana-mcp Deployment: a changed mapping rolls its pods, an unchanged one changes nothing.
func ConfigureJWTOrgMappingSecret(ctx context.Context, c client.Client, organizations []*organization.Organization) error {
	logger := log.FromContext(ctx).WithValues("secret", JWTOrgMappingSecretNamespace+"/"+JWTOrgMappingSecretName, "organizations_count", len(organizations))

	sorted := slices.Clone(organizations)
	slices.SortStableFunc(sorted, func(a, b *organization.Organization) int {
		return cmp.Or(cmp.Compare(a.ID(), b.ID()), cmp.Compare(a.Name(), b.Name()))
	})

	orgsMapping, err := generateGrafanaOrgsMapping(sorted)
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

	if result != controllerutil.OperationResultNone {
		logger.Info("configured JWT org mapping secret", "result", result)
	}

	return setJWTOrgMappingChecksum(ctx, c, orgsMapping)
}

// setJWTOrgMappingChecksum sets the sha256 of the org mapping on the pod template of the grafana-mcp
// Deployment with a merge patch. The patch is sent without reading the Deployment first, so the
// operator only needs the patch verb on it. Setting the same value again is a no-op for the API
// server and does not roll the pods. A missing Deployment is skipped.
func setJWTOrgMappingChecksum(ctx context.Context, c client.Client, orgsMapping string) error {
	logger := log.FromContext(ctx).WithValues("deployment", JWTOrgMappingSecretNamespace+"/"+JWTOrgMappingDeploymentName)

	sum := sha256.Sum256([]byte(orgsMapping))
	checksum := hex.EncodeToString(sum[:])

	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]string{
						JWTOrgMappingChecksumAnnotation: checksum,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build JWT org mapping checksum patch: %w", err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTOrgMappingDeploymentName,
			Namespace: JWTOrgMappingSecretNamespace,
		},
	}
	err = c.Patch(ctx, deployment, client.RawPatch(types.MergePatchType, patch))
	if apierrors.IsNotFound(err) {
		logger.V(1).Info("skipping JWT org mapping checksum, deployment not found")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to set JWT org mapping checksum on deployment: %w", err)
	}
	return nil
}
