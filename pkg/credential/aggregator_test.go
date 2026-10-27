package credential

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	ns1 = "ns1"

	gatewayNamespace    = "mimir"
	httprouteSecretName = "mimir-gateway-httproute-auth"
)

func newGatewayConfigs() GatewayConfigs {
	return GatewayConfigs{
		observabilityv1alpha1.CredentialBackendMetrics: NewGatewayConfig(gatewayNamespace, httprouteSecretName),
	}
}

func credentialWithSecret(name, namespace, agentName string, backend observabilityv1alpha1.CredentialBackend, htpasswd string) (*observabilityv1alpha1.AgentCredential, *corev1.Secret) {
	cred := newAgentCredential(name, namespace, agentName, backend)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data: map[string][]byte{
			SecretKeyHtpasswd: []byte(htpasswd),
		},
	}
	return cred, secret
}

func TestAggregate_BuildsSortedHtpasswd(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	credB, secB := credentialWithSecret("b", ns1, "b", observabilityv1alpha1.CredentialBackendMetrics, "b:{SHA}xx")
	credA, secA := credentialWithSecret("a", "ns2", "a", observabilityv1alpha1.CredentialBackendMetrics, "a:{SHA}yy")
	credLogs, secLogs := credentialWithSecret("c", "ns2", "c", observabilityv1alpha1.CredentialBackendLogs, "c:{SHA}zz")

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, credB, secB, credA, secA, credLogs, secLogs).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	httproute := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute))

	got := string(httproute.Data[HTTPRouteDataKey])
	expected := strings.Join([]string{"a:{SHA}yy", "b:{SHA}xx"}, "\n")
	assert.Equal(t, expected, got, "expected entries sorted alphabetically and only metrics-backend entries")

	gatewaySecrets := &corev1.SecretList{}
	require.NoError(t, c.List(context.Background(), gatewaySecrets, client.InNamespace(gatewayNamespace)))
	assert.Len(t, gatewaySecrets.Items, 1, "only the HTTPRoute gateway secret is written")
}

func TestAggregate_ExcludesDeletingCredentials(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	deleting, secDel := credentialWithSecret("deleting", ns1, "deleting", observabilityv1alpha1.CredentialBackendMetrics, "deleting:{SHA}xx")
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{observabilityv1alpha1.AgentCredentialFinalizer}

	keep, secKeep := credentialWithSecret("keep", ns1, "keep", observabilityv1alpha1.CredentialBackendMetrics, "keep:{SHA}yy")

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, deleting, secDel, keep, secKeep).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	httproute := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute))

	got := string(httproute.Data[HTTPRouteDataKey])
	assert.Equal(t, "keep:{SHA}yy", got)
}

func TestAggregate_NoCredentials_WritesEmpty(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	httproute := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute))
	assert.Empty(t, string(httproute.Data[HTTPRouteDataKey]))
}

func TestAggregate_MissingNamespace_ReturnsNil(t *testing.T) {
	scheme := newScheme(t)
	cred, secret := credentialWithSecret("a", ns1, "a", observabilityv1alpha1.CredentialBackendMetrics, "a:{SHA}yy")

	// Deliberately omit the gateway namespace — the aggregator must log and
	// return nil rather than attempting the write and swallowing the error.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cred, secret).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	httproute := &corev1.Secret{}
	err := c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute)
	assert.True(t, apierrors.IsNotFound(err), "gateway secret should not have been created in a missing namespace")
}

func TestAggregate_PropagatesWriteErrors(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}
	cred, secret := credentialWithSecret("a", ns1, "a", observabilityv1alpha1.CredentialBackendMetrics, "a:{SHA}yy")

	// Intercept Create on Secret so the write itself fails — previously this
	// was silently masked by the blanket IsNotFound swallow; it must surface now.
	boom := errors.New("synthetic write failure")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, cred, secret).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok && obj.GetNamespace() == gatewayNamespace {
				return boom
			}
			return cl.Create(ctx, obj, opts...)
		},
	}).Build()

	a := NewAggregator(c, newGatewayConfigs())
	err := a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestAggregate_SkipsCredentialsWithoutSecret(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	withSecret, secret := credentialWithSecret("with", ns1, "with", observabilityv1alpha1.CredentialBackendMetrics, "with:{SHA}xx")
	withoutSecret := newAgentCredential("without", ns1, "without", observabilityv1alpha1.CredentialBackendMetrics)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, withSecret, secret, withoutSecret).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	httproute := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute))
	assert.Equal(t, "with:{SHA}xx", string(httproute.Data[HTTPRouteDataKey]))
}

func TestAggregate_DuplicateAgentName_WritesOnlyOldest(t *testing.T) {
	older := metav1.NewTime(metav1.Now().Add(-time.Hour))
	newer := metav1.Now()

	tests := []struct {
		name     string
		first    *observabilityv1alpha1.AgentCredential
		second   *observabilityv1alpha1.AgentCredential
		expected string
	}{
		{
			name:     "older creation timestamp wins",
			first:    newDuplicate("z-old", "ns2", older, "dup:{SHA}old"),
			second:   newDuplicate("a-new", ns1, newer, "dup:{SHA}new"),
			expected: "dup:{SHA}old",
		},
		{
			name:     "same timestamp falls back to namespace",
			first:    newDuplicate("z", ns1, older, "dup:{SHA}ns1"),
			second:   newDuplicate("a", "ns2", older, "dup:{SHA}ns2"),
			expected: "dup:{SHA}ns1",
		},
		{
			name:     "same timestamp and namespace falls back to name",
			first:    newDuplicate("a", ns1, older, "dup:{SHA}a"),
			second:   newDuplicate("b", ns1, older, "dup:{SHA}b"),
			expected: "dup:{SHA}a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme := newScheme(t)
			gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}
			other, otherSecret := credentialWithSecret("other", ns1, "other", observabilityv1alpha1.CredentialBackendMetrics, "other:{SHA}zz")

			objs := []client.Object{gwNs, other, otherSecret}
			for _, cred := range []*observabilityv1alpha1.AgentCredential{tc.first, tc.second} {
				objs = append(objs, cred, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: cred.Name, Namespace: cred.Namespace},
					Data:       map[string][]byte{SecretKeyHtpasswd: []byte(cred.Annotations["htpasswd"])},
				})
			}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
			a := NewAggregator(c, newGatewayConfigs())

			require.NoError(t, a.Aggregate(t.Context(), observabilityv1alpha1.CredentialBackendMetrics))

			httproute := &corev1.Secret{}
			require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: gatewayNamespace, Name: httprouteSecretName}, httproute))
			assert.Equal(t, tc.expected+"\nother:{SHA}zz", string(httproute.Data[HTTPRouteDataKey]))
		})
	}
}

// newDuplicate returns a metrics AgentCredential named "dup" whose rendered
// htpasswd entry is carried in an annotation for the test to write out.
func newDuplicate(name, namespace string, created metav1.Time, htpasswd string) *observabilityv1alpha1.AgentCredential {
	cred := newAgentCredential(name, namespace, "dup", observabilityv1alpha1.CredentialBackendMetrics)
	cred.CreationTimestamp = created
	cred.Annotations = map[string]string{"htpasswd": htpasswd}
	return cred
}
