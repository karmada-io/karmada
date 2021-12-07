package helper

import (
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// NewExecutionPredicate generates the event filter function to skip events that the controllers are uninterested.
// Used by controllers:
// - execution controller working in karmada-controller-manager
// - work status controller working in karmada-controller-manager
func NewExecutionPredicate(mgr controllerruntime.Manager) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) create event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			clusterName, err := names.GetClusterName(obj.Namespace)
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", obj.Namespace, obj.Name)
				return false
			}

			clusterObj, err := util.GetCluster(mgr.GetClient(), clusterName)
			if err != nil {
				klog.Errorf("Failed to get the given member cluster %s", clusterName)
				return false
			}

			return clusterObj.Spec.SyncMode == clusterv1alpha1.Push
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) update event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			clusterName, err := names.GetClusterName(obj.Namespace)
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", obj.Namespace, obj.Name)
				return false
			}

			clusterObj, err := util.GetCluster(mgr.GetClient(), clusterName)
			if err != nil {
				klog.Errorf("Failed to get the given member cluster %s", clusterName)
				return false
			}

			return clusterObj.Spec.SyncMode == clusterv1alpha1.Push
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) delete event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			clusterName, err := names.GetClusterName(obj.Namespace)
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", obj.Namespace, obj.Name)
				return false
			}

			clusterObj, err := util.GetCluster(mgr.GetClient(), clusterName)
			if err != nil {
				klog.Errorf("Failed to get the given member cluster %s", clusterName)
				return false
			}
			return clusterObj.Spec.SyncMode == clusterv1alpha1.Push
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForServiceExportController generates an event filter function for ServiceExport controller running by karmada-controller-manager.
func NewPredicateForServiceExportController(mgr controllerruntime.Manager) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*workv1alpha1.Work)

			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) update event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			clusterName, err := names.GetClusterName(obj.GetNamespace())
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", obj.GetNamespace(), obj.GetName())
				return false
			}

			clusterObj, err := util.GetCluster(mgr.GetClient(), clusterName)
			if err != nil {
				klog.Errorf("Failed to get the given member cluster %s", clusterName)
				return false
			}
			return clusterObj.Spec.SyncMode == clusterv1alpha1.Push
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}

// NewClusterPredicateOnAgent generates an event filter function with Cluster for karmada-agent.
func NewClusterPredicateOnAgent(clusterName string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return createEvent.Object.GetName() == clusterName
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return updateEvent.ObjectOld.GetName() == clusterName
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return deleteEvent.Object.GetName() == clusterName
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForServiceExportControllerOnAgent generates an event filter function for ServiceExport controller running by karmada-agent.
func NewPredicateForServiceExportControllerOnAgent(curClusterName string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*workv1alpha1.Work)

			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) update event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			clusterName, err := names.GetClusterName(obj.GetNamespace())
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", obj.GetNamespace(), obj.GetName())
				return false
			}
			return clusterName == curClusterName
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}

// NewExecutionPredicateOnAgent generates the event filter function to skip events that the controllers are uninterested.
// Used by controllers:
// - execution controller working in agent
// - work status controller working in agent
func NewExecutionPredicateOnAgent() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) create event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			obj := updateEvent.ObjectNew.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) update event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			obj := deleteEvent.Object.(*workv1alpha1.Work)

			// Ignore the object that has been suppressed.
			if util.GetLabelValue(obj.Labels, util.PropagationInstruction) == util.PropagationInstructionSuppressed {
				klog.V(5).Infof("Ignored Work(%s/%s) delete event as propagation instruction is suppressed.", obj.Namespace, obj.Name)
				return false
			}

			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
}
