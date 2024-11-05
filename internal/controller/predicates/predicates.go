package predicates

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// GrafanaPodRecreatedPredicate implements a default predicate function for grafana's pod deleted events.
type GrafanaPodRecreatedPredicate struct {
	predicate.Funcs
}

func (GrafanaPodRecreatedPredicate) Delete(e event.DeleteEvent) bool {
	if e.Object != nil && strings.Contains(e.Object.GetName(), "grafana") && e.Object.GetNamespace() == "monitoring" {
		return true
	}

	return false
}
