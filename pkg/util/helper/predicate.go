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

// NewWorkPredicate generates an event filter function with Work for karmada-controller-manager.
func NewWorkPredicate(mgr controllerruntime.Manager) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			obj := createEvent.Object.(*workv1alpha1.Work)
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
			clusterName, err := names.GetClusterName(updateEvent.ObjectOld.GetNamespace())
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", updateEvent.ObjectOld.GetNamespace(),
					updateEvent.ObjectOld.GetName())
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

// NewClusterPredicateByAgent generates an event filter function with Cluster for karmada-agent.
func NewClusterPredicateByAgent(clusterName string) predicate.Funcs {
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

// NewPredicateForServiceExportControllerByAgent generates an event filter function for ServiceExport controller running by karmada-agent.
func NewPredicateForServiceExportControllerByAgent(curClusterName string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			clusterName, err := names.GetClusterName(updateEvent.ObjectOld.GetNamespace())
			if err != nil {
				klog.Errorf("Failed to get member cluster name for work %s/%s", updateEvent.ObjectOld.GetNamespace(),
					updateEvent.ObjectOld.GetName())
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
