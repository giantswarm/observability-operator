package predicates

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// GrafanaPodRecreatedPredicate implements an event handler predicate function.
// This predicate is used to trigger a reconciliation when the Grafana pod is recreated to ensure the Grafana instance is up to date.
type GrafanaPodRecreatedPredicate struct {
	predicate.Funcs
}

func (GrafanaPodRecreatedPredicate) Create(e event.CreateEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}

func (GrafanaPodRecreatedPredicate) Delete(e event.DeleteEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}

// When a grafana pod becomes ready, we want to trigger a reconciliation.
func (GrafanaPodRecreatedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		return false
	}
	newPod, ok := e.ObjectNew.(*corev1.Pod)
	if !ok {
		return false
	}

	if !newPod.DeletionTimestamp.IsZero() {
		return false
	}

	return isGrafanaPod(newPod) && isPodReady(newPod)
}

// isGrafanaPod checks if the object is a Grafana pod.
func isGrafanaPod(pod *corev1.Pod) bool {
	return pod != nil &&
		strings.HasPrefix(pod.GetName(), "grafana") &&
		pod.GetNamespace() == "monitoring" &&
		pod.GetLabels() != nil &&
		pod.GetLabels()["app.kubernetes.io/instance"] == "grafana"
}

// isPodReady checks if the pod is ready by inspecting its conditions.
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (GrafanaPodRecreatedPredicate) Generic(e event.GenericEvent) bool {
	// Do nothing as we want to act on Grafana pod creation event only.
	return false
}
