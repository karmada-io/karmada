package status

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var workPredicateFn = builder.WithPredicates(predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool { return false },
	UpdateFunc: func(e event.UpdateEvent) bool {
		var oldStatus, newStatus workv1alpha1.WorkStatus

		switch oldWork := e.ObjectOld.(type) {
		case *workv1alpha1.Work:
			oldStatus = oldWork.Status
		default:
			return false
		}

		switch newWork := e.ObjectNew.(type) {
		case *workv1alpha1.Work:
			newStatus = newWork.Status
		default:
			return false
		}

		return !reflect.DeepEqual(oldStatus, newStatus)
	},
	DeleteFunc:  func(event.DeleteEvent) bool { return true },
	GenericFunc: func(event.GenericEvent) bool { return false },
})

// updateResourceStatus will try to calculate the summary status and
// update to original object that the ResourceBinding refer to.
func updateResourceStatus(
	dynamicClient dynamic.Interface,
	restMapper meta.RESTMapper,
	interpreter resourceinterpreter.ResourceInterpreter,
	resource *unstructured.Unstructured,
	bindingStatus workv1alpha2.ResourceBindingStatus,
) error {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(resource.GetAPIVersion(), resource.GetKind()))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK(%s/%s), Error: %v", resource.GetAPIVersion(), resource.GetKind(), err)
		return err
	}

	if !interpreter.HookEnabled(resource.GroupVersionKind(), configv1alpha1.InterpreterOperationAggregateStatus) {
		return nil
	}
	newObj, err := interpreter.AggregateStatus(resource, bindingStatus.AggregatedStatus)
	if err != nil {
		klog.Errorf("Failed to aggregate status for resource(%s/%s/%s, Error: %v", gvr, resource.GetNamespace(), resource.GetName(), err)
		return err
	}
	if reflect.DeepEqual(resource, newObj) {
		klog.V(3).Infof("Ignore update resource(%s/%s/%s) status as up to date.", gvr, resource.GetNamespace(), resource.GetName())
		return nil
	}

	if _, err = dynamicClient.Resource(gvr).Namespace(resource.GetNamespace()).UpdateStatus(context.TODO(), newObj, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update resource(%s/%s/%s), Error: %v", gvr, resource.GetNamespace(), resource.GetName(), err)
		return err
	}
	klog.V(3).Infof("Update resource(%s/%s/%s) status successfully.", gvr, resource.GetNamespace(), resource.GetName())
	return nil
}
