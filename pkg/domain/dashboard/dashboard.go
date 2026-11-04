package dashboard

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
)

// apiGroup is the API group of the Grafana Dashboard schema.
const apiGroup = "dashboard.grafana.app"

// v2APIVersionPattern matches the v2 family of the Grafana Dashboard schema:
// "dashboard.grafana.app/v2" and its pre-releases, such as "v2alpha1" and "v2beta1".
var v2APIVersionPattern = regexp.MustCompile(`^dashboard\.grafana\.app/v2((alpha|beta)[1-9][0-9]*)?$`)

// SupportedAPIVersions lists the apiVersion values accepted for Grafana Dashboard
// schema dashboards. Classic dashboards have no apiVersion.
var SupportedAPIVersions = []string{
	apiGroup + "/v2",
	apiGroup + "/v2beta1",
	apiGroup + "/v2alpha1",
}

// Dashboard represents a Grafana dashboard domain object
type Dashboard struct {
	uid          string
	organization string
	folderPath   string
	// sourceNamespace and sourceName identify the ConfigMap the dashboard comes from.
	sourceNamespace string
	sourceName      string
	content         map[string]any
}

// New creates a new Dashboard domain object, extracting the UID from the content.
// sourceNamespace and sourceName identify the ConfigMap the dashboard comes from.
func New(organization string, folderPath string, sourceNamespace string, sourceName string, content map[string]any) *Dashboard {
	return &Dashboard{
		uid:             extractUID(content),
		organization:    organization,
		folderPath:      folderPath,
		sourceNamespace: sourceNamespace,
		sourceName:      sourceName,
		content:         content,
	}
}

// IsV2 reports whether the dashboard content uses the Grafana Dashboard schema
// (apiVersion "dashboard.grafana.app/v2" or one of its pre-releases), as opposed
// to the classic flat schema.
func IsV2(content map[string]any) bool {
	apiVersion, _ := content["apiVersion"].(string)
	return v2APIVersionPattern.MatchString(apiVersion)
}

// V2Version returns the version part of a Grafana Dashboard schema apiVersion,
// such as "v2" or "v2alpha1". It returns an empty string for classic dashboards.
func V2Version(content map[string]any) string {
	if !IsV2(content) {
		return ""
	}
	apiVersion, _ := content["apiVersion"].(string)
	return strings.TrimPrefix(apiVersion, apiGroup+"/")
}

// extractUID reads the dashboard UID, supporting both the classic flat schema
// (top-level "uid") and the Grafana Dashboard schema (metadata.name). Grafana derives
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

// SourcePath returns the source ConfigMap reference, formatted as
// "configmap <namespace>/<name>".
func (d *Dashboard) SourcePath() string {
	return fmt.Sprintf("configmap %s/%s", d.sourceNamespace, d.sourceName)
}

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

	// Validate apiVersion: classic dashboards have none, v2 dashboards use the v2 family
	if apiVersion, ok := d.content["apiVersion"]; ok && apiVersion != "" && !IsV2(d.content) {
		errors = append(errors, fmt.Errorf("%w %q: accepted values are %s, or no apiVersion for a classic dashboard",
			ErrUnsupportedAPIVersion, fmt.Sprint(apiVersion), strings.Join(SupportedAPIVersions, ", ")))
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
