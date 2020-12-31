package informermanager

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// NewHandlerOnAllEvents builds a ResourceEventHandler that the function 'fn' will be called on all events(add/update/delete).
func NewHandlerOnAllEvents(fn func(runtime.Object)) cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			curObj := cur.(runtime.Object)
			klog.V(2).Infof("Receive add event, obj is: %+v", curObj)
			fn(curObj)
		},
		UpdateFunc: func(old, cur interface{}) {
			curObj := cur.(runtime.Object)
			if !reflect.DeepEqual(old, cur) {
				klog.V(2).Infof("Receive update event, obj is: %+v", curObj)
				fn(curObj)
			}
		},
		DeleteFunc: func(old interface{}) {
			if deleted, ok := old.(cache.DeletedFinalStateUnknown); ok {
				// This object might be stale but ok for our current usage.
				old = deleted.Obj
				if old == nil {
					return
				}
			}
			oldObj := old.(runtime.Object)
			klog.V(2).Infof("Receive delete event, obj is: %+v", oldObj)
			fn(oldObj)
		},
	}
}
