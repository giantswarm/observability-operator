package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Finalizer needs to follow the format "domain name, a forward slash and the name of the finalizer"
	// See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#finalizers
	GrafanaOrganizationFinalizer = "observability.giantswarm.io/grafanaorganization"
)

// GrafanaOrganizationSpec defines the desired state of GrafanaOrganization
type GrafanaOrganizationSpec struct {
	// DisplayName is the name displayed when viewing the organization in Grafana. It can be different from the actual org's name.
	// +kubebuilder:example="Giant Swarm"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:unique=true
	DisplayName string `json:"displayName"`

	// Access rules defines user permissions for interacting with the organization in Grafana.
	RBAC *RBAC `json:"rbac"`

	// Tenants is a list of tenants that are associated with the Grafana organization.
	// +kubebuilder:example={"giantswarm"}
	// +kube:validation:MinItems=1
	Tenants []TenantID `json:"tenants"`
}

// TenantID is a unique identifier for a tenant. Must follow both Grafana Mimir tenant ID restrictions
// and Alloy component naming restrictions.
// See: https://grafana.com/docs/mimir/latest/configure/about-tenant-ids/
// See: https://grafana.com/docs/alloy/latest/get-started/configuration-syntax/syntax/#identifiers
// Allowed characters: alphanumeric (a-z, A-Z, 0-9) and underscore (_)
// Must start with a letter or underscore, max 150 characters (Mimir tenant limit)
// Forbidden value: "__mimir_cluster" (enforced by validating webhook)
// +kubebuilder:validation:Pattern="^[a-zA-Z_][a-zA-Z0-9_]{0,149}$"
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=150
type TenantID string

// RBAC defines the RoleBasedAccessControl configuration for the Grafana organization.
// Each fields represents the mapping to a Grafana role:
//
//	Admin: full access to the Grafana organization
//	Editor: edit resources in the Grafana organization
//	Viewer: read only access to the Grafana organization
//
// Each fields holds a list of string which represents values for a specific auth provider org attribute.
// The org attribute is looked up using the `org_attribute_path` which is configured on your Grafana instance, see `https://<YOUR_GRAFANA_INSTANCE>/admin/settings`.
// A user is granted a role when one of its org attribute value is contained within one of the role's values defined here.
// More info on Grafana org attribute and role mapping at [Configure role mapping](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/generic-oauth/#configure-role-mapping)
type RBAC struct {
	// Admins is a list of user organizations that have admin access to the grafanaorganization.
	Admins []string `json:"admins"`

	// Editors is a list of user organizations that have editor access to the grafanaorganization.
	// +optional
	Editors []string `json:"editors"`

	// Viewers is a list of user organizations that have viewer access to the grafanaorganization.
	// +optional
	Viewers []string `json:"viewers"`
}

// GrafanaOrganizationStatus defines the observed state of GrafanaOrganization
type GrafanaOrganizationStatus struct {
	// OrgID is the actual organisation ID in grafana.
	// +optional
	OrgID int64 `json:"orgID"`

	// DataSources is a list of grafana data sources that are available to the Grafana organization.
	// +optional
	DataSources []DataSource `json:"dataSources"`
}

// DataSource defines the name and id for data sources.
type DataSource struct {
	// ID is the unique id of the data source.
	ID int64 `json:"ID"`

	// Name is the name of the data source.
	Name string `json:"name"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".spec.displayName",name=DisplayName,type=string
//+kubebuilder:printcolumn:JSONPath=".status.orgID",name=OrgID,type=integer

// GrafanaOrganization is the Schema describing a Grafana organization. Its lifecycle is managed by the observability-operator.
type GrafanaOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaOrganizationSpec   `json:"spec,omitempty"`
	Status GrafanaOrganizationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// GrafanaOrganizationList contains a list of GrafanaOrganization
type GrafanaOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrafanaOrganization{}, &GrafanaOrganizationList{})
}
