package dashboard

import (
	"fmt"
	"maps"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
)

// Dashboard represents a Grafana dashboard domain object
type Dashboard struct {
	uid          string
	organization string
	folderPath   string
	content      map[string]any
}

// New creates a new Dashboard domain object, extracting the UID from the content
func New(organization string, folderPath string, content map[string]any) *Dashboard {
	// Extract UID from content
	uid := ""
	if content != nil {
		if uidValue, ok := content["uid"].(string); ok {
			uid = uidValue
		}
	}

	return &Dashboard{
		uid:          uid,
		organization: organization,
		folderPath:   folderPath,
		content:      content,
	}
}

// Getters (pure accessors)
func (d *Dashboard) UID() string          { return d.uid }
func (d *Dashboard) Organization() string { return d.organization }
func (d *Dashboard) FolderPath() string   { return d.folderPath }

// Content returns a copy of the content to prevent external mutation
func (d *Dashboard) Content() map[string]any {
	content := make(map[string]any)
	maps.Copy(content, d.content)
	return content
}

// Validate performs domain validation logic
func (d *Dashboard) Validate() []error {
	var errors []error

	// Validate UID is present
	if d.uid == "" {
		errors = append(errors, ErrMissingUID)
	}

	// Validate organization is present
	if d.organization == "" {
		errors = append(errors, ErrMissingOrganization)
	}

	// Validate content is not nil (though empty content might be valid)
	if d.content == nil {
		errors = append(errors, ErrInvalidJSON)
	}

	// Validate folder path if present
	if err := folder.ValidatePath(d.folderPath); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// String provides a string representation for debugging
func (d *Dashboard) String() string {
	return fmt.Sprintf("Dashboard{uid: %s, organization: %s}", d.uid, d.organization)
}
