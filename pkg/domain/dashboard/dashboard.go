package dashboard

import (
	"fmt"
	"maps"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
)

// v2APIVersion identifies dashboards using the Grafana App Platform schema
// ("dashboard.grafana.app/v2"), as opposed to the classic flat schema.
const v2APIVersion = "dashboard.grafana.app/v2"

// Dashboard represents a Grafana dashboard domain object
type Dashboard struct {
	uid          string
	organization string
	folderPath   string
	content      map[string]any
}

// New creates a new Dashboard domain object, extracting the UID from the content
func New(organization string, folderPath string, content map[string]any) *Dashboard {
	return &Dashboard{
		uid:          extractUID(content),
		organization: organization,
		folderPath:   folderPath,
		content:      content,
	}
}

// IsV2 reports whether the dashboard content uses the Grafana App Platform schema
// (apiVersion "dashboard.grafana.app/v2"), as opposed to the classic flat schema.
func IsV2(content map[string]any) bool {
	apiVersion, _ := content["apiVersion"].(string)
	return apiVersion == v2APIVersion
}

// extractUID reads the dashboard UID, supporting both the classic flat schema
// (top-level "uid") and the App Platform schema (metadata.name). Grafana derives
// the stored dashboard UID from metadata.name for v2 dashboards.
func extractUID(content map[string]any) string {
	if content == nil {
		return ""
	}

	if IsV2(content) {
		metadata, ok := content["metadata"].(map[string]any)
		if !ok {
			return ""
		}
		name, _ := metadata["name"].(string)
		return name
	}

	uid, _ := content["uid"].(string)
	return uid
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
