package grafana

const ManagedDashboardTag = "managed-by-obs-operator"

// InjectManagedTag ensures the managed tag is present in the dashboard content's tags array.
// This is idempotent - if the tag already exists, it does nothing.
func InjectManagedTag(content map[string]any) {
	tags, ok := content["tags"].([]any)
	if !ok {
		tags = []any{}
	}

	for _, tag := range tags {
		if tag == ManagedDashboardTag {
			return
		}
	}

	content["tags"] = append(tags, ManagedDashboardTag)
}
