package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/giantswarm/observability-operator/api/v1alpha1"
	"github.com/giantswarm/observability-operator/pkg/credential"
)

// Installed AgentCredential CRDs that predate the agentName deprecation still
// require the field, so the Cluster controller must keep setting it.
func TestEnsureAgentCredential_SetsAgentNameForOlderCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "org-test", UID: "uid-1"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	r := &ClusterReconciler{Client: c}

	require.NoError(t, r.ensureAgentCredential(t.Context(), cluster, v1alpha1.CredentialBackendMetrics))

	cred := &v1alpha1.AgentCredential{}
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: credential.ClusterCredentialName(cluster.Name, v1alpha1.CredentialBackendMetrics)}
	require.NoError(t, c.Get(t.Context(), key, cred))
	assert.Equal(t, "my-cluster", cred.Spec.AgentName) //nolint:staticcheck // SA1019: asserting the compatibility value
	assert.Equal(t, v1alpha1.CredentialBackendMetrics, cred.Spec.Backend)
}

// A change to an AgentCredential's Secret, such as a new username, must reach
// the agents without waiting for the periodic resync, so it enqueues the
// Cluster that owns the AgentCredential.
func TestMapCredentialSecretToCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "org-test", UID: "cluster-uid"}}
	cred := &v1alpha1.AgentCredential{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-observability-metrics", Namespace: "org-test", UID: "cred-uid"}}
	require.NoError(t, controllerutil.SetControllerReference(cluster, cred, scheme))
	standalone := &v1alpha1.AgentCredential{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "org-test", UID: "standalone-uid"}}

	credSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-observability-metrics-auth", Namespace: "org-test"}}
	require.NoError(t, controllerutil.SetControllerReference(cred, credSecret, scheme))
	standaloneSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "org-test"}}
	require.NoError(t, controllerutil.SetControllerReference(standalone, standaloneSecret, scheme))
	unrelated := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "org-test"}}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cred, standalone).Build()

	assert.Equal(t,
		[]reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "org-test", Name: "my-cluster"}}},
		mapCredentialSecretToCluster(t.Context(), c, credSecret))
	assert.Empty(t, mapCredentialSecretToCluster(t.Context(), c, standaloneSecret), "AgentCredential not owned by a Cluster")
	assert.Empty(t, mapCredentialSecretToCluster(t.Context(), c, unrelated), "Secret not owned by an AgentCredential")
}
