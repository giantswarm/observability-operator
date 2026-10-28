package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
