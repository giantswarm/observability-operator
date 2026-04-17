package credential

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

const (
	gatewayNamespace   = "mimir"
	ingressSecretName  = "mimir-gateway-ingress-auth"
	httprouteSecretKey = "mimir-gateway-httproute-auth"
)

func newGatewayConfigs() GatewayConfigs {
	return GatewayConfigs{
		observabilityv1alpha1.CredentialBackendMetrics: NewGatewayConfig(gatewayNamespace, ingressSecretName, httprouteSecretKey),
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

	credB, secB := credentialWithSecret("b", "ns1", "b", observabilityv1alpha1.CredentialBackendMetrics, "b:{SHA}xx")
	credA, secA := credentialWithSecret("a", "ns2", "a", observabilityv1alpha1.CredentialBackendMetrics, "a:{SHA}yy")
	credLogs, secLogs := credentialWithSecret("c", "ns2", "c", observabilityv1alpha1.CredentialBackendLogs, "c:{SHA}zz")

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, credB, secB, credA, secA, credLogs, secLogs).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	ingress := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: ingressSecretName}, ingress))

	got := string(ingress.Data[IngressDataKey])
	expected := strings.Join([]string{"a:{SHA}yy", "b:{SHA}xx"}, "\n")
	assert.Equal(t, expected, got, "expected entries sorted alphabetically and only metrics-backend entries")
}

func TestAggregate_ExcludesDeletingCredentials(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	deleting, secDel := credentialWithSecret("deleting", "ns1", "deleting", observabilityv1alpha1.CredentialBackendMetrics, "deleting:{SHA}xx")
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{observabilityv1alpha1.AgentCredentialFinalizer}

	keep, secKeep := credentialWithSecret("keep", "ns1", "keep", observabilityv1alpha1.CredentialBackendMetrics, "keep:{SHA}yy")

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, deleting, secDel, keep, secKeep).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	ingress := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: ingressSecretName}, ingress))

	got := string(ingress.Data[IngressDataKey])
	assert.Equal(t, "keep:{SHA}yy", got)
}

func TestAggregate_NoCredentials_WritesEmpty(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	ingress := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: ingressSecretName}, ingress))
	assert.Empty(t, string(ingress.Data[IngressDataKey]))
}

func TestAggregate_SkipsCredentialsWithoutSecret(t *testing.T) {
	scheme := newScheme(t)
	gwNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: gatewayNamespace}}

	withSecret, secret := credentialWithSecret("with", "ns1", "with", observabilityv1alpha1.CredentialBackendMetrics, "with:{SHA}xx")
	withoutSecret := newAgentCredential("without", "ns1", "without", observabilityv1alpha1.CredentialBackendMetrics)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(gwNs, withSecret, secret, withoutSecret).Build()
	a := NewAggregator(c, newGatewayConfigs())

	require.NoError(t, a.Aggregate(context.Background(), observabilityv1alpha1.CredentialBackendMetrics))

	ingress := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: gatewayNamespace, Name: ingressSecretName}, ingress))
	assert.Equal(t, "with:{SHA}xx", string(ingress.Data[IngressDataKey]))
}
