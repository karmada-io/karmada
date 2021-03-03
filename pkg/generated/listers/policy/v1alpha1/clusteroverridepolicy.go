// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ClusterOverridePolicyLister helps list ClusterOverridePolicies.
// All objects returned here must be treated as read-only.
type ClusterOverridePolicyLister interface {
	// List lists all ClusterOverridePolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterOverridePolicy, err error)
	// Get retrieves the ClusterOverridePolicy from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ClusterOverridePolicy, error)
	ClusterOverridePolicyListerExpansion
}

// clusterOverridePolicyLister implements the ClusterOverridePolicyLister interface.
type clusterOverridePolicyLister struct {
	indexer cache.Indexer
}

// NewClusterOverridePolicyLister returns a new ClusterOverridePolicyLister.
func NewClusterOverridePolicyLister(indexer cache.Indexer) ClusterOverridePolicyLister {
	return &clusterOverridePolicyLister{indexer: indexer}
}

// List lists all ClusterOverridePolicies in the indexer.
func (s *clusterOverridePolicyLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterOverridePolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterOverridePolicy))
	})
	return ret, err
}

// Get retrieves the ClusterOverridePolicy from the index for a given name.
func (s *clusterOverridePolicyLister) Get(name string) (*v1alpha1.ClusterOverridePolicy, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusteroverridepolicy"), name)
	}
	return obj.(*v1alpha1.ClusterOverridePolicy), nil
}
