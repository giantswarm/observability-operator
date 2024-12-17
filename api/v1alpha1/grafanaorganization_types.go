/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// TenantID is a unique identifier for a tenant. It must be lowercase.
// +kubebuilder:validation:Pattern="^[a-z]*$"
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=63
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
