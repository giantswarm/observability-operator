package labels

const (
	// TODO migrate to observability.giantswarm.io/kind
	DashboardSelectorLabelName  = "app.giantswarm.io/kind"
	DashboardSelectorLabelValue = "dashboard"

	// GrafanaOrganizationKey is the annotation/label key for the Grafana organization.
	GrafanaOrganizationKey = "observability.giantswarm.io/organization"

	// GrafanaFolderKey is the annotation/label key for the Grafana folder path.
	GrafanaFolderKey = "observability.giantswarm.io/folder"
)
