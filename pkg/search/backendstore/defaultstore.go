package backendstore

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Default is the default BackendStore
type Default struct {
	resourceEventHander cache.ResourceEventHandler
}

// NewDefaultBackend create a new default BackendStore
func NewDefaultBackend(cluster string) *Default {
	klog.Infof("create default backend store: %s", cluster)
	return &Default{
		resourceEventHander: &cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				us, ok := obj.(*unstructured.Unstructured)
				if !ok {
					klog.Errorf("unexpected type %T", obj)
					return
				}
				klog.V(4).Infof("AddFunc Cluster(%s) GVK(%s) Name(%s/%s)",
					cluster, us.GroupVersionKind().String(), us.GetNamespace(), us.GetName())
			},
			UpdateFunc: func(oldObj, curObj interface{}) {
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

// ResourceEventHandlerFuncs return the ResourceEventHandlerFuncs
func (d *Default) ResourceEventHandlerFuncs() cache.ResourceEventHandler {
	return d.resourceEventHander
}

// Close close the BackendStore
func (*Default) Close() {}
