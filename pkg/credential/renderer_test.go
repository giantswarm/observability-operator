package credential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// fixedPasswordGenerator returns deterministic values so tests can assert on them.
type fixedPasswordGenerator struct {
	password string
}

func (g *fixedPasswordGenerator) GeneratePassword(length int) (string, error) {
	return g.password, nil
}

func (g *fixedPasswordGenerator) GenerateHtpasswd(username, password string) (string, error) {
	return username + ":{SHA}" + password, nil
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, observabilityv1alpha1.AddToScheme(scheme))
	return scheme
}

func newAgentCredential(name, namespace, agentName string, backend observabilityv1alpha1.CredentialBackend) *observabilityv1alpha1.AgentCredential {
	return &observabilityv1alpha1.AgentCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: observabilityv1alpha1.AgentCredentialSpec{
			Backend:   backend,
			AgentName: agentName,
		},
	}
}

func TestRender_CreatesSecret(t *testing.T) {
	scheme := newScheme(t)
	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred).Build()

	r := &Renderer{Client: c, PasswordGenerator: &fixedPasswordGenerator{password: "p1"}}
	secret, err := r.Render(context.Background(), cred)
	require.NoError(t, err)

	assert.Equal(t, "c1", secret.Name)
	assert.Equal(t, "ns1", secret.Namespace)
	assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	assert.Equal(t, "agent-a", string(secret.Data[SecretKeyUsername]))
	assert.Equal(t, "p1", string(secret.Data[SecretKeyPassword]))
	assert.Equal(t, "agent-a:{SHA}p1", string(secret.Data[SecretKeyHtpasswd]))

	// Owner reference points at the AgentCredential.
	require.Len(t, secret.OwnerReferences, 1)
	assert.Equal(t, "c1", secret.OwnerReferences[0].Name)
	assert.Equal(t, "AgentCredential", secret.OwnerReferences[0].Kind)
}

func TestRender_PreservesExistingPassword(t *testing.T) {
	scheme := newScheme(t)
	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)

	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns1"},
		Data: map[string][]byte{
			SecretKeyPassword: []byte("kept-password"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred, existing).Build()

	r := &Renderer{Client: c, PasswordGenerator: &fixedPasswordGenerator{password: "should-not-be-used"}}
	secret, err := r.Render(context.Background(), cred)
	require.NoError(t, err)

	assert.Equal(t, "kept-password", string(secret.Data[SecretKeyPassword]))
	assert.Equal(t, "agent-a:{SHA}kept-password", string(secret.Data[SecretKeyHtpasswd]))
}

func TestRender_UsesSpecSecretName(t *testing.T) {
	scheme := newScheme(t)
	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
	cred.Spec.SecretName = "custom-secret-name"
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred).Build()

	r := &Renderer{Client: c, PasswordGenerator: &fixedPasswordGenerator{password: "p1"}}
	secret, err := r.Render(context.Background(), cred)
	require.NoError(t, err)

	assert.Equal(t, "custom-secret-name", secret.Name)

	// And the secret is actually persisted under that name.
	got := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: "ns1", Name: "custom-secret-name"}, got))
}
