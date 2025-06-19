package dashboard

import (
	"fmt"
	"maps"
)

// Dashboard represents a Grafana dashboard domain object
type Dashboard struct {
	uid          string
	organization string
	content      map[string]any
}

// New creates a new Dashboard domain object
func New(uid, organization string, content map[string]any) *Dashboard {
	return &Dashboard{
		uid:          uid,
		organization: organization,
		content:      content,
	}
}

// Getters (pure accessors)
func (d *Dashboard) UID() string          { return d.uid }
func (d *Dashboard) Organization() string { return d.organization }

// Content returns a copy of the content to prevent external mutation
func (d *Dashboard) Content() map[string]any {
	content := make(map[string]any)
	maps.Copy(content, d.content)
	return content
}

// Validate performs domain validation logic
func (d *Dashboard) Validate() []error {
	var errs []error

	// Replicate exact existing validation logic
	if d.uid == "" {
		errs = append(errs, ErrMissingUID)
	}

	if d.organization == "" {
		errs = append(errs, ErrMissingOrganization)
	}

	if d.content == nil {
		errs = append(errs, ErrInvalidJSON)
	}

	return errs
}

// String provides a string representation for debugging
func (d *Dashboard) String() string {
	return fmt.Sprintf("Dashboard{uid: %s, organization: %s}", d.uid, d.organization)
}
