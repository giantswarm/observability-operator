package predicates

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/observability-operator/pkg/config"
)

// NewAlertmanagerSecretPredicate returns a predicate that filters only the Alertmanager secret created by the observability-operator Helm chart.
func NewAlertmanagerSecretPredicate(conf config.Config) predicate.Predicate {
	filter := func(object client.Object) bool {
		if object == nil {
			return false
		}

		secret, ok := object.(*v1.Secret)
		if !ok {
			return false
		}

		if !secret.DeletionTimestamp.IsZero() {
			return false
		}

		labels := secret.GetLabels()

		ok = secret.GetName() == conf.Monitoring.AlertmanagerSecretName &&
			secret.GetNamespace() == conf.OperatorNamespace &&
			labels != nil &&
			labels["app.kubernetes.io/name"] == "observability-operator"

		return ok
	}

	p := predicate.NewPredicateFuncs(filter)

	return p
}

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
