package binding

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

var workPredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		var statusesOld, statusesNew []workv1alpha1.ManifestStatus

		switch oldWork := e.ObjectOld.(type) {
		case *workv1alpha1.Work:
			statusesOld = oldWork.Status.ManifestStatuses
		default:
			return false
		}

		switch newWork := e.ObjectNew.(type) {
		case *workv1alpha1.Work:
			statusesNew = newWork.Status.ManifestStatuses
		default:
			return false
		}

		return !reflect.DeepEqual(statusesOld, statusesNew)
	},
	DeleteFunc: func(event.DeleteEvent) bool {
		return false
	},
	GenericFunc: func(event.GenericEvent) bool {
		return false
	},
})
