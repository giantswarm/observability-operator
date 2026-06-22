package organization

import (
	"errors"
	"slices"
	"strconv"
)

const GiantSwarmDefaultTenant = "giantswarm"

var ErrOrganizationNotFound = errors.New("organization not found")
var SharedOrg = NewFromGrafana(1, "Shared Org")

// TenantConfig represents a tenant with its access types
type TenantConfig struct {
	Name  string
	Types []string // "data" and/or "alerting"
}

// Organization represents a Grafana organization domain object
type Organization struct {
	id      int64
	name    string
	tenants []TenantConfig
	admins  []string
	editors []string
	viewers []string
}

// New creates a new Organization domain object
func New(id int64, name string, tenants []TenantConfig, admins []string, editors []string, viewers []string) *Organization {
	return &Organization{
		id:      id,
		name:    name,
		tenants: tenants,
		admins:  admins,
		editors: editors,
		viewers: viewers,
	}
}

// NewFromGrafana creates a new Organization with only ID and name (for basic cases)
func NewFromGrafana(id int64, name string) *Organization {
	return &Organization{
		id:      id,
		name:    name,
		tenants: nil,
		admins:  nil,
		editors: nil,
		viewers: nil,
	}
}

// Getters (pure accessors)
func (o *Organization) ID() int64               { return o.id }
func (o *Organization) Name() string            { return o.name }
func (o *Organization) Tenants() []TenantConfig { return o.tenants }
func (o *Organization) Admins() []string        { return o.admins }
func (o *Organization) Editors() []string       { return o.editors }
func (o *Organization) Viewers() []string       { return o.viewers }

// TenantIDs returns a slice of all tenant IDs
func (o *Organization) TenantIDs() []string {
	tenantIDs := make([]string, len(o.tenants))
	for i, tenant := range o.tenants {
		tenantIDs[i] = tenant.Name
	}
	return tenantIDs
}

// GetAlertingTenants returns tenants that have alerting access
// The "giantswarm" tenant is only included if the organization name is "Giant Swarm"
func (o *Organization) GetAlertingTenants() []TenantConfig {
	var alertingTenants []TenantConfig
	for _, tenant := range o.tenants {
		if slices.Contains(tenant.Types, "alerting") {
			// Filter: only allow giantswarm tenant for "Giant Swarm" organization
			if tenant.Name == GiantSwarmDefaultTenant && o.name != "Giant Swarm" {
				continue
			}
			alertingTenants = append(alertingTenants, tenant)
		}
	}
	return alertingTenants
}

// SetID updates the organization ID (used when the organization is created in Grafana)
func (o *Organization) SetID(id int64) {
	o.id = id
}

// String provides a string representation for debugging
func (o *Organization) String() string {
	return "Organization{id: " + strconv.FormatInt(o.id, 10) + ", name: " + o.name + "}"
}
