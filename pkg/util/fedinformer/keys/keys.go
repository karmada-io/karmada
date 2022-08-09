package keys

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// ClusterWideKey is the object key which is a unique identifier under a cluster, across all resources.
type ClusterWideKey struct {
	// Group is the API Group of resource being referenced.
	Group string

	// Version is the API Version of the resource being referenced.
	Version string

	// Kind is the type of resource being referenced.
	Kind string

	// Namespace is the name of a namespace.
	Namespace string

	// Name is the name of resource being referenced.
	Name string
}

// String returns the key's printable info with format:
// "<GroupVersion>, kind=<Kind>, <NamespaceKey>"
func (k ClusterWideKey) String() string {
	return fmt.Sprintf("%s, kind=%s, %s", k.GroupVersion().String(), k.Kind, k.NamespaceKey())
}

// NamespaceKey returns the traditional key of an object.
func (k *ClusterWideKey) NamespaceKey() string {
	if len(k.Namespace) > 0 {
		return k.Namespace + "/" + k.Name
	}

	return k.Name
}

// GroupVersionKind returns the group, version, and kind of resource being referenced.
func (k *ClusterWideKey) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   k.Group,
		Version: k.Version,
		Kind:    k.Kind,
	}
}

// GroupVersion returns the group and version of resource being referenced.
func (k *ClusterWideKey) GroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   k.Group,
		Version: k.Version,
	}
}

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (ClusterWideKey, error) {
	key := ClusterWideKey{}

	runtimeObject, ok := obj.(runtime.Object)
	if !ok {
		klog.Errorf("Invalid object")
		return key, fmt.Errorf("not runtime object")
	}

	metaInfo, err := meta.Accessor(obj)
	if err != nil { // should not happen
		return key, fmt.Errorf("object has no meta: %v", err)
	}

	gvk := runtimeObject.GetObjectKind().GroupVersionKind()
	key.Group = gvk.Group
	key.Version = gvk.Version
	key.Kind = gvk.Kind
	key.Namespace = metaInfo.GetNamespace()
	key.Name = metaInfo.GetName()

	return key, nil
}

// FederatedKey is the object key which is a unique identifier across all clusters in federation.
type FederatedKey struct {
	// Cluster is the cluster name of the referencing object.
	Cluster string
	ClusterWideKey
}

// String returns the key's printable info with format:
// "cluster=<Cluster>, <GroupVersion>, kind=<Kind>, <NamespaceKey>"
func (f FederatedKey) String() string {
	return fmt.Sprintf("cluster=%s, %s, kind=%s, %s", f.Cluster, f.GroupVersion().String(), f.Kind, f.NamespaceKey())
}

// FederatedKeyFunc generates a FederatedKey for object.
func FederatedKeyFunc(cluster string, obj interface{}) (FederatedKey, error) {
	key := FederatedKey{}

	if len(cluster) == 0 {
		return key, fmt.Errorf("empty cluster name is not allowed")
	}

	cwk, err := ClusterWideKeyFunc(obj)
	if err != nil {
		klog.Errorf("Invalid object")
		return key, err
	}

	key.Cluster = cluster
	key.ClusterWideKey = cwk

	return key, nil
}
