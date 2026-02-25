/*
Copyright 2020 The Karmada Authors.

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

package fedinformer

import (
	"k8s.io/client-go/tools/cache"
)

// NewHandlerOnEvents builds a ResourceEventHandler.
func NewHandlerOnEvents(addFunc func(obj any, isInInitialList bool), updateFunc func(oldObj, newObj any), deleteFunc func(obj any)) cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerDetailedFuncs{
		AddFunc:    addFunc,
		UpdateFunc: updateFunc,
		DeleteFunc: deleteFunc,
	}
}

// NewFilteringHandlerOnAllEvents builds a FilteringResourceEventHandler applies the provided filter to all events
// coming in, ensuring the appropriate nested handler method is invoked.
//
// Note: An object that starts passing the filter after an update is considered an add, and
// an object that stops passing the filter after an update is considered a deletion.
// Like the handlers, the filter MUST NOT modify the objects it is given.
func NewFilteringHandlerOnAllEvents(filterFunc func(obj any) bool, addFunc func(obj any, isInInitialList bool),
	updateFunc func(oldObj, newObj any), deleteFunc func(obj any)) cache.ResourceEventHandler {
	return &cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler: cache.ResourceEventHandlerDetailedFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: deleteFunc,
		},
	}
}
