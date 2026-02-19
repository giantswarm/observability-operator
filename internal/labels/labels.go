package labels

const (
	// TODO migrate to observability.giantswarm.io/kind
	DashboardSelectorLabelName  = "app.giantswarm.io/kind"
	DashboardSelectorLabelValue = "dashboard"

	// GrafanaOrganizationAnnotation is the annotation/label key for the Grafana organization.
	GrafanaOrganizationAnnotation = "observability.giantswarm.io/organization"

	// GrafanaFolderAnnotation is the annotation/label key for the Grafana folder path.
	GrafanaFolderAnnotation = "observability.giantswarm.io/folder"
)
