/*
Copyright 2022 The Karmada Authors.

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

package backendstore

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Default is the default BackendStore
type Default struct {
	resourceEventHandler cache.ResourceEventHandler
}

// NewDefaultBackend create a new default BackendStore
func NewDefaultBackend(cluster string) *Default {
	klog.Infof("create default backend store: %s", cluster)
	return &Default{
		resourceEventHandler: &cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				us, ok := obj.(*unstructured.Unstructured)
				if !ok {
					klog.Errorf("unexpected type %T", obj)
					return
				}
				klog.V(4).Infof("AddFunc Cluster(%s) GVK(%s) Name(%s/%s)",
					cluster, us.GroupVersionKind().String(), us.GetNamespace(), us.GetName())
			},
			UpdateFunc: func(_, curObj interface{}) {
				us, ok := curObj.(*unstructured.Unstructured)
				if !ok {
					klog.Errorf("unexpected type %T", curObj)
					return
				}
				klog.V(4).Infof("UpdateFunc Cluster(%s) GVK(%s) Name(%s/%s)",
					cluster, us.GroupVersionKind().String(), us.GetNamespace(), us.GetName())
			},
			DeleteFunc: func(obj interface{}) {
				us, ok := obj.(*unstructured.Unstructured)
				if !ok {
					klog.Errorf("unexpected type %T", obj)
					return
				}
				klog.V(4).Infof("DeleteFunc Cluster(%s) GVK(%s) Name(%s/%s)",
					cluster, us.GroupVersionKind().String(), us.GetNamespace(), us.GetName())
			}}}
}

// ResourceEventHandlerFuncs returns the ResourceEventHandler
func (d *Default) ResourceEventHandlerFuncs() cache.ResourceEventHandler {
	return d.resourceEventHandler
}

// Close close the BackendStore
func (*Default) Close() {}
