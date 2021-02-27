package detector

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
)

// ClusterWideKey is the object key which is a unique identifier under a cluster, across all resources.
type ClusterWideKey struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

func (k *ClusterWideKey) String() string {
	return k.GVK.String() + "/" + k.Namespace + "/" + k.Name
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (util.QueueKey, error) {
	runtimeObject, ok := obj.(runtime.Object)
	if !ok {
		klog.Errorf("Invalid object")
		return nil, fmt.Errorf("not runtime object")
	}

	metaInfo, err := meta.Accessor(obj)
	if err != nil { // should not happen
		return nil, fmt.Errorf("object has no meta: %v", err)
	}

	key := ClusterWideKey{
		GVK:       runtimeObject.GetObjectKind().GroupVersionKind(),
		Namespace: metaInfo.GetNamespace(),
		Name:      metaInfo.GetName(),
	}

	return key, nil
}
