package predicates

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/giantswarm/observability-operator/pkg/common/tenancy"
)

const (
	AlertmanagerConfigSelectorLabelName  = "observability.giantswarm.io/kind"
	AlertmanagerConfigSelectorLabelValue = "alertmanager-config"

	mimirNamespace             = "mimir"
	mimirInstance              = "mimir"
	mimirAlertmanagerComponent = "alertmanager"
)

var AlertmanagerConfigSecretLabelSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{
		AlertmanagerConfigSelectorLabelName: AlertmanagerConfigSelectorLabelValue,
	},
	MatchExpressions: []metav1.LabelSelectorRequirement{
		{
			Key:      tenancy.TenantSelectorLabel,
			Operator: metav1.LabelSelectorOpExists,
		},
	},
}

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

// Filter only the Alertmanager configuration secrets
func NewAlertmanagerConfigSecretsPredicate() (predicate.Predicate, error) {
	predicate, err := predicate.LabelSelectorPredicate(AlertmanagerConfigSecretLabelSelector)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return predicate, nil
}
