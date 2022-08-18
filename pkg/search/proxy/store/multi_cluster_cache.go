package store

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// temporarily suppress lint: xxx is unused
// TODO: remove it after these structs are used
var _ = clusterCache{}
var _ = resourceCache{}

// MultiClusterCache caches resource from multi member clusters
type MultiClusterCache struct {
}

// HasResource return whether resource is cached.
func (c *MultiClusterCache) HasResource(_ schema.GroupVersionResource) bool {
	return false
}

// GetClusterForResource returns which cluster the resource belong to.
func (c *MultiClusterCache) GetClusterForResource(ctx context.Context, gvr schema.GroupVersionResource) (*clusterv1alpha1.Cluster, error) {
	return nil, nil
}
