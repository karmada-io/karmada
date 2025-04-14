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

	// When using a typed client, decoding to a versioned struct (not an internal API type), the apiVersion/kind
	// information will be dropped. Therefore, the APIVersion/Kind information of runtime.Object needs to be verified.
	// See issue: https://github.com/kubernetes/kubernetes/issues/80609
	gvk := runtimeObject.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) == 0 {
		return key, fmt.Errorf("runtime object has no kind")
	}

	if len(gvk.Version) == 0 {
		return key, fmt.Errorf("runtime object has no version")
	}

	key.Group = gvk.Group
	key.Version = gvk.Version
	key.Kind = gvk.Kind
	key.Namespace = metaInfo.GetNamespace()
	key.Name = metaInfo.GetName()

	return key, nil
}

// ClusterWideKeyWithConfig is the object key which is a unique identifier under a cluster, combined with certain config.
type ClusterWideKeyWithConfig struct {
	// ClusterWideKey is the object key which is a unique identifier under a cluster, across all resources.
	ClusterWideKey ClusterWideKey

	// ResourceChangeByKarmada defines whether resource is changed by Karmada
	ResourceChangeByKarmada bool
}

// String returns the key's printable info with format:
// "<GroupVersion>, kind=<Kind>, <NamespaceKey>, ResourceChangeByKarmada=<ResourceChangeByKarmada>"
func (k ClusterWideKeyWithConfig) String() string {
	return fmt.Sprintf("%s, ResourceChangeByKarmada=%v", k.ClusterWideKey.String(), k.ResourceChangeByKarmada)
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

// NamespacedKey is the object key which is a unique identifier under a cluster, across all resources.
type NamespacedKey struct {
	Namespace string
	Name      string
}

// NamespacedKeyFunc generates a NamespacedKey for object.
func NamespacedKeyFunc(obj interface{}) (NamespacedKey, error) {
	key := NamespacedKey{}

	metaInfo, err := meta.Accessor(obj)
	if err != nil { // should not happen
		return key, fmt.Errorf("object has no meta: %v", err)
	}
	key.Namespace = metaInfo.GetNamespace()
	key.Name = metaInfo.GetName()
	return key, nil
}

// String returns the key's printable info with format:
// "namespace=<Namespace>, name=<Name>"
func (k NamespacedKey) String() string {
	return fmt.Sprintf("namespace=%s, name=%s", k.Namespace, k.Name)
}

// NamespaceKey returns the traditional key of an object.
func (k *NamespacedKey) NamespaceKey() string {
	if len(k.Namespace) > 0 {
		return k.Namespace + "/" + k.Name
	}

	return k.Name
}
