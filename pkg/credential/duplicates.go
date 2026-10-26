package credential

import (
	observabilityv1alpha1 "github.com/giantswarm/observability-operator/api/v1alpha1"
)

// AgentNameOwners returns, for every agent name used on the given backend, the
// AgentCredential whose htpasswd entry the gateway serves. When several
// credentials share an agent name, the oldest one owns it: the earliest
// creation timestamp wins, then the lowest namespace, then the lowest name.
// Credentials being deleted never own an agent name.
func AgentNameOwners(credentials []observabilityv1alpha1.AgentCredential, backend observabilityv1alpha1.CredentialBackend) map[string]*observabilityv1alpha1.AgentCredential {
	owners := make(map[string]*observabilityv1alpha1.AgentCredential)
	for i := range credentials {
		cred := &credentials[i]
		if cred.Spec.Backend != backend || !cred.DeletionTimestamp.IsZero() {
			continue
		}
		if owner, ok := owners[cred.Spec.AgentName]; !ok || olderThan(cred, owner) {
			owners[cred.Spec.AgentName] = cred
		}
	}
	return owners
}

func olderThan(a, b *observabilityv1alpha1.AgentCredential) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	return a.Name < b.Name
}
