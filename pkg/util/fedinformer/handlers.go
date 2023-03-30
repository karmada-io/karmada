package fedinformer

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

// NewHandlerOnAllEvents builds a ResourceEventHandler that the function 'fn' will be called on all events(add/update/delete).
func NewHandlerOnAllEvents(fn func(runtime.Object)) cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(cur interface{}) {
			curObj := cur.(runtime.Object)
			fn(curObj)
		},
		UpdateFunc: func(old, cur interface{}) {
			curObj := cur.(runtime.Object)
			if !reflect.DeepEqual(old, cur) {
				fn(curObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			curObj, ok := obj.(runtime.Object)

			// When a delete is dropped, the relist will notice a pod in the store not
			// in the list, leading to the insertion of a tombstone object which contains
			// the deleted key/value. Note that this value might be stale. If the Pod
			// changed labels the new deployment will not be woken up till the periodic resync.
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
					return
				}
				curObj, ok = tombstone.Obj.(runtime.Object)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %#v", obj))
					return
				}
			}
			fn(curObj)
		},
	}
}

// NewHandlerOnEvents builds a ResourceEventHandler.
func NewHandlerOnEvents(addFunc func(obj interface{}), updateFunc func(oldObj, newObj interface{}), deleteFunc func(obj interface{})) cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		UpdateFunc: updateFunc,
		DeleteFunc: deleteFunc,
	}
}

// NewFilteringHandlerOnAllEvents builds a FilteringResourceEventHandler applies the provided filter to all events
// coming in, ensuring the appropriate nested handler method is invoked.
//
// Note: An object that starts passing the filter after an update is considered an add, and
// an object that stops passing the filter after an update is considered a delete.
// Like the handlers, the filter MUST NOT modify the objects it is given.
func NewFilteringHandlerOnAllEvents(filterFunc func(obj interface{}) bool, addFunc func(obj interface{}),
	updateFunc func(oldObj, newObj interface{}), deleteFunc func(obj interface{})) cache.ResourceEventHandler {
	return &cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: deleteFunc,
		},
	}
}
