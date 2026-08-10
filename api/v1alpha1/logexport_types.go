package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Condition types set on LogExportStatus.

	// LogExportConditionReady is true once the export pipeline is configured and
	// the exporter has accepted it.
	LogExportConditionReady = "Ready"
	// LogExportConditionExporterAvailable reports whether alloy-logexporter is
	// actually running on this installation. The app ships to every management
	// cluster in a disabled state, so a valid LogExport can exist while nothing is
	// there to act on it.
	LogExportConditionExporterAvailable = "ExporterAvailable"
)

// LogExportDestinationType selects which destination variant is in use.
// +kubebuilder:validation:Enum=s3;loki
type LogExportDestinationType string

const (
	// LogExportDestinationS3 writes objects to an S3 or S3-compatible bucket.
	LogExportDestinationS3 LogExportDestinationType = "s3"
	// LogExportDestinationLoki pushes to a Loki-compatible endpoint.
	LogExportDestinationLoki LogExportDestinationType = "loki"
)

// LogExportSpec defines the desired state of LogExport.
type LogExportSpec struct {
	// Selector chooses which log lines are exported, using LogQL syntax.
	//
	// Only the subset the exporter can render is accepted: a stream selector with at
	// least one exact match, optional line filters, an optional "| json", and optional
	// label filters. Aggregations and time ranges are rejected -- an export is a
	// continuous tee, not a query, so it has no start or end.
	//
	// The same expression can be pasted into Grafana Explore to see exactly which
	// lines it matches before committing it.
	//
	// This is NOT limited by the namespace of this resource: a selector matches
	// against every log line arriving at the installation's write path, from any
	// cluster. See the LogExport type documentation.
	// +kubebuilder:example="{scrape_job=\"audit-logs\"}"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Selector string `json:"selector"`

	// Destination is where the selected lines are written.
	Destination LogExportDestination `json:"destination"`
}

// LogExportDestination is a discriminated union: exactly the variant named by
// Type must be set.
//
// Adding support for a new kind of sink means adding a variant here, which keeps
// the API open-ended without accepting arbitrary agent configuration.
// +kubebuilder:validation:XValidation:rule="self.type != 's3' || has(self.s3)",message="spec.destination.s3 is required when type is 's3'"
// +kubebuilder:validation:XValidation:rule="self.type != 'loki' || has(self.loki)",message="spec.destination.loki is required when type is 'loki'"
// +kubebuilder:validation:XValidation:rule="self.type == 's3' || !has(self.s3)",message="spec.destination.s3 may only be set when type is 's3'"
// +kubebuilder:validation:XValidation:rule="self.type == 'loki' || !has(self.loki)",message="spec.destination.loki may only be set when type is 'loki'"
type LogExportDestination struct {
	// Type selects the destination variant.
	Type LogExportDestinationType `json:"type"`

	// S3 configures an S3 or S3-compatible bucket. Required when type is "s3".
	// +optional
	S3 *S3Destination `json:"s3,omitempty"`

	// Loki configures a Loki-compatible push endpoint. Required when type is "loki".
	// +optional
	Loki *LokiDestination `json:"loki,omitempty"`
}

// S3Destination writes gzipped, newline-delimited JSON objects to a bucket.
type S3Destination struct {
	// Bucket is the destination bucket name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Bucket string `json:"bucket"`

	// Region is the bucket's region.
	// +kubebuilder:example="eu-west-2"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Region string `json:"region"`

	// Prefix is the key prefix objects are written under. Object keys are
	// <prefix>/year=…/month=…/day=…/hour=…/minute=…/logs_<uuidv7>.txt.gz, using the
	// time the object was uploaded rather than the time of the events in it.
	// +optional
	// +kubebuilder:example="giantswarm/audit"
	// +kubebuilder:validation:MaxLength=512
	Prefix string `json:"prefix,omitempty"`

	// Endpoint overrides the S3 endpoint. Leave empty for AWS; set it to target an
	// S3-compatible store.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	Endpoint string `json:"endpoint,omitempty"`

	// ForcePathStyle addresses the bucket as a path rather than a subdomain.
	// S3-compatible stores generally require this; AWS generally does not.
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`

	// RoleARN is an IAM role to assume before writing, for cross-account delivery.
	// When set, CredentialsRef is usually unnecessary: the exporter authenticates
	// with its own ServiceAccount identity and assumes this role.
	// +optional
	// +kubebuilder:validation:Pattern="^$|^arn:aws[a-zA-Z-]*:iam::[0-9]{12}:role/.+$"
	// +kubebuilder:validation:MaxLength=2048
	RoleARN string `json:"roleARN,omitempty"`

	// CredentialsRef names a Secret holding static credentials, with keys
	// AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
	//
	// The Secret is read from THIS resource's namespace; cross-namespace
	// references are not possible. Omit it to authenticate by workload identity
	// (IRSA) instead, optionally combined with RoleARN.
	// +optional
	CredentialsRef *corev1.LocalObjectReference `json:"credentialsRef,omitempty"`
}

// LokiDestination pushes to a Loki-compatible endpoint.
type LokiDestination struct {
	// URL is the push endpoint, including the path.
	// +kubebuilder:example="https://logs.example.com/loki/api/v1/push"
	// +kubebuilder:validation:Pattern="^https?://.+$"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	URL string `json:"url"`

	// TenantID is sent as the X-Scope-OrgID header. Required by multi-tenant Loki
	// deployments, ignored by single-tenant ones.
	// +optional
	// +kubebuilder:validation:MaxLength=150
	TenantID string `json:"tenantID,omitempty"`

	// CredentialsRef names a Secret holding basic-auth credentials, with keys
	// username and password.
	//
	// The Secret is read from THIS resource's namespace; cross-namespace
	// references are not possible.
	// +optional
	CredentialsRef *corev1.LocalObjectReference `json:"credentialsRef,omitempty"`
}

// LogExportStatus defines the observed state of LogExport.
type LogExportStatus struct {
	// ObservedGeneration is the .metadata.generation this status was set from.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the export's state.
	// Types: Ready (the pipeline is configured), ExporterAvailable (alloy-logexporter
	// is running on this installation).
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:JSONPath=".spec.destination.type",name=Destination,type=string
//+kubebuilder:printcolumn:JSONPath=".status.conditions[?(@.type=='Ready')].status",name=Ready,type=string
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date

// LogExport continuously copies a selection of the installation's logs to a
// destination outside the observability platform, for archival or for delivery to
// a system Giant Swarm does not run.
//
// Logs are teed off the write path, so an export can never slow down or break log
// ingestion. The same property makes delivery best-effort: if the exporter is
// unavailable, lines are dropped rather than queued, and there is no backfill.
//
// # Scope
//
// This resource is namespaced so that credentials stay next to it, but the
// namespace is NOT a data boundary: a LogExport in any namespace can select logs
// from every cluster on the installation. Permission to create one should be
// treated as privileged access to log data.
type LogExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogExportSpec   `json:"spec,omitempty"`
	Status LogExportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LogExportList contains a list of LogExport.
type LogExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogExport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogExport{}, &LogExportList{})
}
