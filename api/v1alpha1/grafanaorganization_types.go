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

// GrafanaOrganizationSpec defines the desired state of GrafanaOrganization
type GrafanaOrganizationSpec struct {
	// DisplayName is the displayed name of the organization. It can be different from the actual org's name.
	DisplayName string `json:"displayName"`

	// Access role that can be assumed to access the organization.
	Rbac *Rbac `json:"rbac,omitempty"`
}

// RBAC defines the RBAC configuration for the organization.
type Rbac struct {
	// Admins is a list of service accounts that have admin access to the organization.
	Admins []string `json:"admins"`

	// Editors is a list of service accounts that have editor access to the organization.
	// +optional
	Editors []string `json:"editors"`

	// Viewers is a list of service accounts that have viewer access to the organization.
	// +optional
	Viewers []string `json:"viewers"`
}

// GrafanaOrganizationStatus defines the observed state of GrafanaOrganization
type GrafanaOrganizationStatus struct {
	// OrgID is the unique id of the org.
	OrgID string `json:"orgID"`

	// DataSources is a list of grafana data sources that are available to the organization.
	DataSources []DataSources `json:"dataSources"`
}

// DataSource defines the name and id for data sources.
type DataSources struct {
	// Name is the name of the data source.
	Name string `json:"name"`

	// ID is the unique id of the data source.
	Id string `json:"id"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GrafanaOrganization is the Schema for the grafanaorganizations API
type GrafanaOrganization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaOrganizationSpec   `json:"spec,omitempty"`
	Status GrafanaOrganizationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GrafanaOrganizationList contains a list of GrafanaOrganization
type GrafanaOrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaOrganization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GrafanaOrganization{}, &GrafanaOrganizationList{})
}
