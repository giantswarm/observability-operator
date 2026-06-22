package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AgentCredentialFinalizer ensures gateway htpasswd is re-aggregated before
	// the credential disappears.
	AgentCredentialFinalizer = "observability.giantswarm.io/agentcredential"

	// Condition types set on AgentCredentialStatus.
	AgentCredentialConditionReady         = "Ready"
	AgentCredentialConditionGatewaySynced = "GatewaySynced"
)

// CredentialBackend is the observability backend this credential grants access to.
// +kubebuilder:validation:Enum=metrics;logs;traces
type CredentialBackend string

const (
	CredentialBackendMetrics CredentialBackend = "metrics"
	CredentialBackendLogs    CredentialBackend = "logs"
	CredentialBackendTraces  CredentialBackend = "traces"
)

// AgentCredentialSpec defines the desired state of an AgentCredential.
type AgentCredentialSpec struct {
	// Backend is the observability backend this credential grants access to
	// (metrics → Mimir, logs → Loki, traces → Tempo). Drives which gateway's
	// htpasswd Secret this credential is aggregated into.
	Backend CredentialBackend `json:"backend"`

	// AgentName identifies the telemetry agent this credential belongs to.
	// Used as the basic-auth username in the generated Secret and htpasswd entry.
	// RFC 7617 forbids ':' in basic-auth usernames.
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9][a-zA-Z0-9_-]{0,252}$"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	AgentName string `json:"agentName"`

	// SecretName overrides the generated Secret name. Defaults to metadata.name.
	// Must be a valid DNS-1123 subdomain.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$"
	SecretName string `json:"secretName,omitempty"`
}

// AgentCredentialStatus defines the observed state of an AgentCredential.
type AgentCredentialStatus struct {
	// SecretRef points to the rendered Secret in the same namespace.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`

	// Conditions represent the latest available observations of the credential's state.
	// Types: Ready (secret rendered), GatewaySynced (htpasswd aggregated).
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:JSONPath=".spec.backend",name=Backend,type=string
//+kubebuilder:printcolumn:JSONPath=".spec.agentName",name=Agent,type=string
//+kubebuilder:printcolumn:JSONPath=".status.conditions[?(@.type=='Ready')].status",name=Ready,type=string
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date

// AgentCredential represents a single basic-auth credential scoped to one
// telemetry agent and one observability backend. Its lifecycle is managed
// by the observability-operator.
type AgentCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentCredentialSpec   `json:"spec,omitempty"`
	Status AgentCredentialStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentCredentialList contains a list of AgentCredential.
type AgentCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentCredential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentCredential{}, &AgentCredentialList{})
}
