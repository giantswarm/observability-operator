package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// HeartbeatFinalizer ensures the external monitor is removed before the
	// resource disappears.
	HeartbeatFinalizer = "observability.giantswarm.io/heartbeat"

	// HeartbeatConditionReady is true once the provider holds a monitor matching
	// this spec.
	HeartbeatConditionReady = "Ready"
)

// HeartbeatProviderType selects which provider variant is in use.
// +kubebuilder:validation:Enum=cronitor
type HeartbeatProviderType string

const (
	// HeartbeatProviderCronitor manages a heartbeat monitor in Cronitor.
	HeartbeatProviderCronitor HeartbeatProviderType = "cronitor"
)

// HeartbeatSpec defines the desired state of Heartbeat.
type HeartbeatSpec struct {
	// Provider is the external service the monitor is created in.
	Provider HeartbeatProvider `json:"provider"`
}

// HeartbeatProvider is a discriminated union: exactly the variant named by Type
// must be set.
//
// Every field of a variant is that provider's own vocabulary -- there is no
// neutral heartbeat vocabulary underneath -- so supporting another service means
// adding a variant here rather than widening a flat spec.
// +kubebuilder:validation:XValidation:rule="self.type != 'cronitor' || has(self.cronitor)",message="spec.provider.cronitor is required when type is 'cronitor'"
// +kubebuilder:validation:XValidation:rule="self.type == 'cronitor' || !has(self.cronitor)",message="spec.provider.cronitor may only be set when type is 'cronitor'"
type HeartbeatProvider struct {
	// Type selects the provider variant.
	Type HeartbeatProviderType `json:"type"`

	// Cronitor configures a Cronitor heartbeat monitor. Required when type is "cronitor".
	// +optional
	Cronitor *CronitorProvider `json:"cronitor,omitempty"`
}

// CronitorProvider describes a Cronitor heartbeat monitor. Field names and
// values follow the Cronitor monitor API.
// +kubebuilder:validation:XValidation:rule="size(self.credentialsRef.name) != 0",message="spec.provider.cronitor.credentialsRef.name must not be empty"
type CronitorProvider struct {
	// CredentialsRef names a Secret holding the Cronitor API keys, with keys
	// managementKey and pingKey:
	//
	//   - managementKey authenticates monitor create, update and delete against
	//     the Cronitor monitor API.
	//   - pingKey is used to ping the monitor once, which is what associates it
	//     with an environment; Cronitor has no API field for that.
	//
	// Both are required. This is the contract between this resource and whatever
	// ships it -- the keys are read from a Secret rather than the operator's own
	// environment precisely so the monitor definition can be shipped by another
	// application.
	//
	// The Secret is read from THIS resource's namespace; cross-namespace
	// references are not possible.
	CredentialsRef corev1.LocalObjectReference `json:"credentialsRef"`

	// Key is the monitor's unique identifier in Cronitor. Defaults to metadata.name.
	//
	// Whatever pings this monitor must use the same key: it is the last path
	// segment of https://cronitor.link/p/<pingKey>/<key>. A mismatch is silent
	// until the grace period expires and the monitor pages.
	//
	// Uniqueness is NOT enforced. Two Heartbeat resources naming one key both
	// write the same monitor, last writer wins; status.monitorKey on each of them
	// is what makes that visible.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9][a-zA-Z0-9._-]*$"
	Key string `json:"key,omitempty"`

	// Schedule is how often the monitor expects to be pinged, in Cronitor
	// schedule syntax: a cron expression or an interval phrase.
	// +kubebuilder:example="every 30 minutes"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Schedule string `json:"schedule"`

	// GraceSeconds is how long Cronitor waits past a missed ping before alerting.
	// +kubebuilder:validation:Minimum=0
	GraceSeconds int `json:"graceSeconds"`

	// RealertInterval is how often Cronitor re-notifies while the monitor stays
	// failed. Empty means it notifies once.
	// +optional
	// +kubebuilder:example="every 24 hours"
	// +kubebuilder:validation:MaxLength=256
	RealertInterval string `json:"realertInterval,omitempty"`

	// Notify lists the Cronitor notification lists alerted when the monitor
	// fails. These lists, and any integration behind them, exist only in the
	// Cronitor account -- this field references them by name and cannot create
	// them. Empty means Cronitor's account default.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=256
	Notify []string `json:"notify,omitempty"`

	// Tags are set on the monitor in Cronitor, for grouping and filtering there.
	//
	// Order is significant to the operator, not to Cronitor: the list is compared
	// as-is against the monitor's current tags to decide whether an update is
	// needed, so an unstable order causes a needless write on every reconcile.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=256
	Tags []string `json:"tags,omitempty"`

	// Note is free text shown on the monitor in Cronitor and carried into its
	// alerts, which makes it the place to put a runbook link.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Note string `json:"note,omitempty"`
}

// HeartbeatStatus defines the observed state of Heartbeat.
type HeartbeatStatus struct {
	// ObservedGeneration is the .metadata.generation this status was set from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// MonitorKey is the key of the monitor this resource last wrote.
	//
	// Keys are not unique across resources by design, so this reports what was
	// actually written rather than what was asked for. Listing it across
	// namespaces is how a collision between two Heartbeat resources is spotted.
	// +optional
	// +kubebuilder:validation:MaxLength=253
	MonitorKey string `json:"monitorKey,omitempty"`

	// Conditions represent the latest available observations of the heartbeat's state.
	// Types: Ready (the provider holds a monitor matching this spec).
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:JSONPath=".spec.provider.type",name=Provider,type=string
//+kubebuilder:printcolumn:JSONPath=".status.monitorKey",name=Key,type=string
//+kubebuilder:printcolumn:JSONPath=".status.conditions[?(@.type=='Ready')].status",name=Ready,type=string
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date

// Heartbeat declares a dead-man's-switch monitor in an external service: a
// monitor that expects to be pinged on a schedule and alerts when the pings stop.
//
// The operator owns the monitor's definition, not the pings. Something else --
// typically an Alertmanager route firing on an always-on alert -- has to do the
// pinging, and it has to address the monitor by the same key. The two halves are
// only connected by that string, so a mismatch produces a monitor that exists,
// is never pinged, and alerts once the grace period elapses.
//
// # Scope
//
// This resource is namespaced so that the provider credentials it references
// stay next to it and are resolved by name only.
type Heartbeat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HeartbeatSpec   `json:"spec,omitempty"`
	Status HeartbeatStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HeartbeatList contains a list of Heartbeat.
type HeartbeatList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Heartbeat `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Heartbeat{}, &HeartbeatList{})
}
