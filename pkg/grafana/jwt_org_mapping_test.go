package grafana

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/giantswarm/observability-operator/pkg/domain/organization"
)

const (
	jwtTestAdminsGroup           = "admins"
	jwtTestGiantSwarmAdminsGroup = "giantswarm-admins"
)

var jwtOrgMappingSecretKey = types.NamespacedName{Name: JWTOrgMappingSecretName, Namespace: JWTOrgMappingSecretNamespace}

func newJWTTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	return scheme
}

var jwtOrgMappingDeploymentKey = types.NamespacedName{Name: JWTOrgMappingDeploymentName, Namespace: JWTOrgMappingSecretNamespace}

func newJWTTestDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTOrgMappingDeploymentName,
			Namespace: JWTOrgMappingSecretNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"custom": "kept"},
				},
			},
		},
	}
}

func jwtOrgMappingChecksum(t *testing.T, c client.Client) string {
	t.Helper()
	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, secret))
	sum := sha256.Sum256(secret.Data[JWTOrgMappingSecretKey])
	return hex.EncodeToString(sum[:])
}

func TestConfigureJWTOrgMappingSecret_CreatesSecret(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, []string{"editors"}, []string{"viewers"}),
		organization.New(2, "Giant Swarm", nil, []string{jwtTestGiantSwarmAdminsGroup}, nil, nil),
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
	giantSwarmOrg := organization.New(2, "Giant Swarm", nil, []string{jwtTestGiantSwarmAdminsGroup}, nil, nil)
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

func TestConfigureJWTOrgMappingSecret_OrderDoesNotChangeChecksum(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).WithObjects(newJWTTestDeployment()).Build()
	sharedOrg := organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, []string{"editors"}, nil)
	giantSwarmOrg := organization.New(2, "Giant Swarm", nil, []string{jwtTestGiantSwarmAdminsGroup}, nil, []string{"viewers"})
	teamOrg := organization.New(3, "Team Org", nil, []string{"team-admins"}, nil, nil)

	permutations := [][]*organization.Organization{
		{sharedOrg, giantSwarmOrg, teamOrg},
		{teamOrg, sharedOrg, giantSwarmOrg},
		{giantSwarmOrg, teamOrg, sharedOrg},
	}

	var checksum string
	for _, organizations := range permutations {
		require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

		deployment := &appsv1.Deployment{}
		require.NoError(t, c.Get(context.Background(), jwtOrgMappingDeploymentKey, deployment))
		got := deployment.Spec.Template.Annotations[JWTOrgMappingChecksumAnnotation]
		require.NotEmpty(t, got)
		if checksum == "" {
			checksum = got
		}
		assert.Equal(t, checksum, got)
	}
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

func TestConfigureJWTOrgMappingSecret_SetsChecksumOnDeployment(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).WithObjects(newJWTTestDeployment()).Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, nil, nil),
	}

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	deployment := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingDeploymentKey, deployment))
	firstChecksum := deployment.Spec.Template.Annotations[JWTOrgMappingChecksumAnnotation]
	assert.Equal(t, jwtOrgMappingChecksum(t, c), firstChecksum)
	assert.Equal(t, "kept", deployment.Spec.Template.Annotations["custom"])

	// An unchanged mapping keeps the checksum.
	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingDeploymentKey, deployment))
	assert.Equal(t, firstChecksum, deployment.Spec.Template.Annotations[JWTOrgMappingChecksumAnnotation])

	// A changed mapping sets a new checksum.
	organizations = append(organizations, organization.New(2, "Giant Swarm", nil, []string{jwtTestGiantSwarmAdminsGroup}, nil, nil))
	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))
	require.NoError(t, c.Get(context.Background(), jwtOrgMappingDeploymentKey, deployment))
	assert.NotEqual(t, firstChecksum, deployment.Spec.Template.Annotations[JWTOrgMappingChecksumAnnotation])
	assert.Equal(t, jwtOrgMappingChecksum(t, c), deployment.Spec.Template.Annotations[JWTOrgMappingChecksumAnnotation])
	assert.Equal(t, "kept", deployment.Spec.Template.Annotations["custom"])
}

func TestConfigureJWTOrgMappingSecret_MissingDeployment(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newJWTTestScheme(t)).Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, nil, nil),
	}

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	require.NoError(t, c.Get(context.Background(), jwtOrgMappingSecretKey, &corev1.Secret{}))
	err := c.Get(context.Background(), jwtOrgMappingDeploymentKey, &appsv1.Deployment{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestConfigureJWTOrgMappingSecret_OnlyPatchesDeployment(t *testing.T) {
	type patchCall struct {
		key       types.NamespacedName
		patchType types.PatchType
	}
	var (
		patches []patchCall
		reads   []string
	)
	isDeployment := func(obj runtime.Object) bool {
		switch obj.(type) {
		case *appsv1.Deployment, *appsv1.DeploymentList:
			return true
		}
		return false
	}
	c := fake.NewClientBuilder().
		WithScheme(newJWTTestScheme(t)).
		WithObjects(newJWTTestDeployment()).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if isDeployment(obj) {
					reads = append(reads, "get")
				}
				return c.Get(ctx, key, obj, opts...)
			},
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if isDeployment(list) {
					reads = append(reads, "list")
				}
				return c.List(ctx, list, opts...)
			},
			Watch: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
				if isDeployment(list) {
					reads = append(reads, "watch")
				}
				return c.Watch(ctx, list, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if isDeployment(obj) {
					patches = append(patches, patchCall{key: client.ObjectKeyFromObject(obj), patchType: patch.Type()})
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()
	organizations := []*organization.Organization{
		organization.New(1, "Shared Org", nil, []string{jwtTestAdminsGroup}, nil, nil),
	}

	require.NoError(t, ConfigureJWTOrgMappingSecret(context.Background(), c, organizations))

	assert.Empty(t, reads)
	assert.Equal(t, []patchCall{{key: jwtOrgMappingDeploymentKey, patchType: types.MergePatchType}}, patches)
}
