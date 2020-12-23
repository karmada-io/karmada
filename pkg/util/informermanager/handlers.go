package informermanager

import (
	"reflect"

	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// NewTriggerOnAllChanges Returns cache.ResourceEventHandlerFuncs that trigger the given function
// on all object changes.
func NewTriggerOnAllChanges(triggerFunc func(pkgruntime.Object) error) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(old interface{}) {
			if deleted, ok := old.(cache.DeletedFinalStateUnknown); ok {
				// This object might be stale but ok for our current usage.
				old = deleted.Obj
				if old == nil {
					return
				}
			}
			oldObj := old.(pkgruntime.Object)
			klog.V(2).Infof("Receive delete event, obj is: %+v", oldObj)
			err := triggerFunc(oldObj)
			if err != nil {
				klog.V(2).Infof("Failed to exec triggerFunc. Error: %v.", err)
			}
		},
		AddFunc: func(cur interface{}) {
			curObj := cur.(pkgruntime.Object)
			klog.V(2).Infof("Receive add event, obj is: %+v", curObj)
			err := triggerFunc(curObj)
			if err != nil {
				klog.V(2).Infof("Failed to exec triggerFunc. Error: %v.", err)
			}
		},
		UpdateFunc: func(old, cur interface{}) {
			curObj := cur.(pkgruntime.Object)
			if !reflect.DeepEqual(old, cur) {
				klog.V(2).Infof("Receive update event, obj is: %+v", curObj)
				err := triggerFunc(curObj)
				if err != nil {
					klog.V(2).Infof("Failed to exec triggerFunc. Error: %v.", err)
				}
			}
		},
	}
}
