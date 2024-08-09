package labels

func Common() map[string]string {
	labels := make(map[string]string)
	labels["giantswarm.io/managed-by"] = "observability-operator"
	labels["application.giantswarm.io/team"] = "atlas"

	return labels
}
