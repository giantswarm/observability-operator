package predicates

import (
	"reflect"
	"strings"

	appv1alpha1 "github.com/giantswarm/apiextensions-application/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// AlloyRulesAppChangedPredicate implements a default update predicate function on app config changes.
type AlloyRulesAppChangedPredicate struct {
	predicate.Funcs
}

func (AlloyRulesAppChangedPredicate) Delete(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	if !strings.Contains(e.Object.GetName(), "alloy-rules") {
		return false
	}

	var ok bool
	if _, ok = e.Object.(*appv1alpha1.App); !ok {
		return false
	}

	return true
}

// Update implements default UpdateEvent filter for validating resource version change.
func (AlloyRulesAppChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	if !strings.Contains(e.ObjectOld.GetName(), "alloy-rules") || !strings.Contains(e.ObjectNew.GetName(), "alloy-rules") {
		return false
	}

	var oldApp, newApp *appv1alpha1.App
	var ok bool
	if oldApp, ok = e.ObjectOld.(*appv1alpha1.App); !ok {
		return false
	}
	if newApp, ok = e.ObjectNew.(*appv1alpha1.App); !ok {
		return false
	}

	return !reflect.DeepEqual(oldApp.Spec.Config, newApp.Spec.Config)
}
