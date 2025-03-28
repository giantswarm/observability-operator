package predicates

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	mimirNamespace             = "mimir"
	mimirInstance              = "mimir"
	mimirAlertmanagerComponent = "alertmanager"
)

// NewAlertmanagerPodPredicate returns a predicate that filters only the Mimir Alertmanager pod.
func NewAlertmanagerPodPredicate() predicate.Predicate {
	filter := func(object client.Object) bool {
		if object == nil {
			return false
		}

		pod, ok := object.(*v1.Pod)
		if !ok {
			return false
		}

		if !pod.DeletionTimestamp.IsZero() {
			return false
		}

		labels := pod.GetLabels()

		ok = pod.GetNamespace() == mimirNamespace &&
			labels != nil &&
			labels["app.kubernetes.io/component"] == mimirAlertmanagerComponent &&
			labels["app.kubernetes.io/instance"] == mimirInstance &&
			isPodReady(pod)

		return ok
	}

	p := predicate.NewPredicateFuncs(filter)

	return p
}
