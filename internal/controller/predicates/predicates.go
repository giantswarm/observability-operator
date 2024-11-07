package predicates

import (
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// GrafanaPodRecreatedPredicate implements an eevent handler predicate function.
// This predicate is used to trigger a reconciliation when the Grafana pod is recreated to ensure the Grafana instance is up to date.
type GrafanaPodRecreatedPredicate struct {
	predicate.Funcs
}

// When a grafana pod is recreated, we want to trigger a reconciliation.
func (GrafanaPodRecreatedPredicate) Create(e event.CreateEvent) bool {
	if e.Object != nil &&
		strings.Contains(e.Object.GetName(), "grafana") &&
		e.Object.GetNamespace() == "monitoring" {
		// Ensure we don't trigger on the grafana permissions pods or grafana multi-tenant proxy
		if l := e.Object.GetLabels(); l != nil && l["app.kubernetes.io/instance"] == "grafana" {
			return true
		}
	}

	return false
}

func (GrafanaPodRecreatedPredicate) Delete(e event.DeleteEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}

func (GrafanaPodRecreatedPredicate) Update(e event.UpdateEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}

func (GrafanaPodRecreatedPredicate) Generic(e event.GenericEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}
