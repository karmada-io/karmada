/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// WorkWithinPushClusterPredicate generates the event filter function to skip events that the controllers are uninterested.
// Used by controllers:
// - execution controller working in karmada-controller-manager
// - work status controller working in karmada-controller-manager
func WorkWithinPushClusterPredicate(mgr controllerruntime.Manager) predicate.Funcs {
	predFunc := func(object client.Object) bool {
		obj := object.(*workv1alpha1.Work)

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
	}

	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return predFunc(createEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return predFunc(updateEvent.ObjectNew) || predFunc(updateEvent.ObjectOld)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return predFunc(deleteEvent.Object)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForServiceExportController generates an event filter function for ServiceExport controller running by karmada-controller-manager.
func NewPredicateForServiceExportController(mgr controllerruntime.Manager) predicate.Funcs {
	predFunc := func(eventType string, object client.Object) bool {
		obj := object.(*workv1alpha1.Work)

		if util.IsWorkSuspendDispatching(obj) {
			klog.V(5).Infof("Ignored Work(%s/%s) %s event as dispatching is suspended.", obj.Namespace, obj.Name, eventType)
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
	}

	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return predFunc("update", updateEvent.ObjectNew) || predFunc("update", updateEvent.ObjectOld)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForEndpointSliceCollectController generates an event filter function for EndpointSliceCollectController running by karmada-controller-manager.
func NewPredicateForEndpointSliceCollectController(mgr controllerruntime.Manager) predicate.Funcs {
	predFunc := func(_ string, object client.Object) bool {
		obj := object.(*workv1alpha1.Work)
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
	}

	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return predFunc("create", createEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return predFunc("update", updateEvent.ObjectNew) || predFunc("update", updateEvent.ObjectOld)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return predFunc("delete", deleteEvent.Object)
		},
		GenericFunc: func(event.GenericEvent) bool {
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
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForServiceExportControllerOnAgent generates an event filter function for ServiceExport controller running by karmada-agent.
func NewPredicateForServiceExportControllerOnAgent(curClusterName string) predicate.Funcs {
	predFunc := func(eventType string, object client.Object) bool {
		obj := object.(*workv1alpha1.Work)

		if util.IsWorkSuspendDispatching(obj) {
			klog.V(5).Infof("Ignored Work(%s/%s) %s event as dispatching is suspended.", obj.Namespace, obj.Name, eventType)
			return false
		}

		clusterName, err := names.GetClusterName(obj.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get member cluster name for work %s/%s", obj.GetNamespace(), obj.GetName())
			return false
		}
		return clusterName == curClusterName
	}

	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return predFunc("update", updateEvent.ObjectNew) || predFunc("update", updateEvent.ObjectOld)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// NewPredicateForEndpointSliceCollectControllerOnAgent generates an event filter function for EndpointSliceCollectController running by karmada-agent.
func NewPredicateForEndpointSliceCollectControllerOnAgent(curClusterName string) predicate.Funcs {
	predFunc := func(_ string, object client.Object) bool {
		obj := object.(*workv1alpha1.Work)
		clusterName, err := names.GetClusterName(obj.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get member cluster name for work %s/%s", obj.GetNamespace(), obj.GetName())
			return false
		}
		return clusterName == curClusterName
	}

	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return predFunc("create", createEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return predFunc("update", updateEvent.ObjectNew) || predFunc("update", updateEvent.ObjectOld)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return predFunc("delete", deleteEvent.Object)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}
