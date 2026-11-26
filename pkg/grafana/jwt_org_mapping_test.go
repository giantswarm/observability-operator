package grafana

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const jwtTestAdminsGroup = "admins"

var jwtOrgMappingSecretKey = types.NamespacedName{Name: JWTOrgMappingSecretName, Namespace: JWTOrgMappingSecretNamespace}

func newJWTTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func TestConfigureJWTOrgMappingSecret_CreatesSecret(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, []string{"editors"}, []string{"viewers"}),
		organization.New(2, "Giant Swarm", nil, []string{"giantswarm-admins"}, nil, nil),
	}

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, secret))

	expected, err := generateGrafanaOrgsMapping(organizations)
	require.NoError(t, err)
	assert.Equal(t, expected, string(secret.Data[JWTOrgMappingSecretKey]))
	assert.Equal(t, `"admins:Shared Org:Admin" "editors:Shared Org:Editor" "viewers:Shared Org:Viewer" "giantswarm-admins:Giant Swarm:Admin"`, string(secret.Data[JWTOrgMappingSecretKey]))
	assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	assert.Equal(t, "observability-operator", secret.Labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "observability-operator", secret.Labels["app.kubernetes.io/part-of"])
}

func TestConfigureJWTOrgMappingSecret_UpdatesSecretInPlace(t *testing.T) {
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTOrgMappingSecretName,
			Namespace: JWTOrgMappingSecretNamespace,
			UID:       "existing-uid",
			Labels:    map[string]string{"custom": "kept"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			JWTOrgMappingSecretKey: []byte(`"stale:Old Org:Admin"`),
			"other":                []byte("kept"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).WithObjects(existing).Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, nil, nil),
	}

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, secret))

	assert.Equal(t, types.UID("existing-uid"), secret.UID)
	assert.Equal(t, `"admins:Shared Org:Admin"`, string(secret.Data[JWTOrgMappingSecretKey]))
	assert.Equal(t, "kept", string(secret.Data["other"]))
	assert.Equal(t, "kept", secret.Labels["custom"])
	assert.Equal(t, "observability-operator", secret.Labels["app.kubernetes.io/managed-by"])
}

func TestConfigureJWTOrgMappingSecret_OrderDoesNotChangeSecret(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).Build()
	sharedOrg := organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, nil, nil)
	giantSwarmOrg := organization.New(2, "Giant Swarm", nil, []string{"giantswarm-admins"}, nil, nil)
	pendingOrg := organization.New(0, "Pending Org", nil, []string{"pending-admins"}, nil, nil)

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, []*organization.Organization{sharedOrg, giantSwarmOrg, pendingOrg}))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, secret))
	resourceVersion := secret.ResourceVersion

	shuffled := []*organization.Organization{giantSwarmOrg, pendingOrg, sharedOrg}
	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, shuffled))

	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, secret))
	assert.Equal(t, resourceVersion, secret.ResourceVersion)
	assert.Equal(t, `"pending-admins:Pending Org:Admin" "admins:Shared Org:Admin" "giantswarm-admins:Giant Swarm:Admin"`, string(secret.Data[JWTOrgMappingSecretKey]))
	// The caller's slice is left untouched.
	assert.Equal(t, []*organization.Organization{giantSwarmOrg, pendingOrg, sharedOrg}, shuffled)
}

func TestConfigureJWTOrgMappingSecret_InvalidOrganization(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).Build()
	organizations := []*organization.Organization{
		organization.New(1, "", nil, []string{jwtTestAdminsGroup}, nil, nil),
	}

	require.Error(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	err := c.Get(context.Background(), jwtOrgMappingSecretKey, &corev1.Secret{})
	assert.True(t, apierrors.IsNotFound(err))
}
