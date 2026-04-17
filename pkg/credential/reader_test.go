package credential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

func TestReader_ReadsCredentials(t *testing.T) {
	scheme := newScheme(t)

	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns1"},
		Data: map[string][]byte{
			SecretKeyUsername: []byte("agent-a"),
			SecretKeyPassword: []byte("hunter2"),
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred, secret).Build()
	r := NewReader(c)

	username, password, err := r.ReadPassword(context.Background(), "ns1", "c1")
	require.NoError(t, err)
	assert.Equal(t, "agent-a", username)
	assert.Equal(t, "hunter2", password)
}

func TestReader_FailsWhenSecretMissing(t *testing.T) {
	scheme := newScheme(t)
	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred).Build()
	r := NewReader(c)

	_, _, err := r.ReadPassword(context.Background(), "ns1", "c1")
	assert.Error(t, err)
}

func TestReader_FailsWhenCredentialMissing(t *testing.T) {
	scheme := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := NewReader(c)

	_, _, err := r.ReadPassword(context.Background(), "ns1", "missing")
	assert.Error(t, err)
}

func TestReader_FailsWhenPasswordEmpty(t *testing.T) {
	scheme := newScheme(t)
	cred := newAgentCredential("c1", "ns1", "agent-a", observabilityv1alpha1.CredentialBackendMetrics)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns1"},
		Data: map[string][]byte{
			SecretKeyUsername: []byte("agent-a"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred, secret).Build()
	r := NewReader(c)

	_, _, err := r.ReadPassword(context.Background(), "ns1", "c1")
	assert.Error(t, err)
}

func TestClusterCredentialAndSecretNames(t *testing.T) {
	assert.Equal(t, "my-cluster-observability-metrics", ClusterCredentialName("my-cluster", observabilityv1alpha1.CredentialBackendMetrics))
	assert.Equal(t, "my-cluster-observability-logs-auth", ClusterSecretName("my-cluster", observabilityv1alpha1.CredentialBackendLogs))
}
