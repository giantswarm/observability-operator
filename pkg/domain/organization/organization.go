package organization

// Organization represents a Grafana organization domain object
type Organization struct {
	id        int64
	name      string
	tenantIDs []string
	admins    []string
	editors   []string
	viewers   []string
}

// New creates a new Organization domain object
func New(id int64, name string, tenantIDs []string, admins []string, editors []string, viewers []string) *Organization {
	return &Organization{
		id:        id,
		name:      name,
		tenantIDs: tenantIDs,
		admins:    admins,
		editors:   editors,
		viewers:   viewers,
	}
}

// NewFromGrafana creates a new Organization with only ID and name (for basic cases)
func NewFromGrafana(id int64, name string) *Organization {
	return &Organization{
		id:        id,
		name:      name,
		tenantIDs: nil,
		admins:    nil,
		editors:   nil,
		viewers:   nil,
	}
}

// Getters (pure accessors)
func (o *Organization) ID() int64           { return o.id }
func (o *Organization) Name() string        { return o.name }
func (o *Organization) TenantIDs() []string { return o.tenantIDs }
func (o *Organization) Admins() []string    { return o.admins }
func (o *Organization) Editors() []string   { return o.editors }
func (o *Organization) Viewers() []string   { return o.viewers }

// SetID updates the organization ID (used when the organization is created in Grafana)
func (o *Organization) SetID(id int64) {
	o.id = id
}

// String provides a string representation for debugging
func (o *Organization) String() string {
	return "Organization{id: " + string(rune(o.id)) + ", name: " + o.name + "}"
}
