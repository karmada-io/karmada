package cache

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type labelIndexerCache struct {
	cache.Cache
}

// NewLabelIndexerCache returns a labelIndexerCache which support use index when to list object by labelSelector
func NewLabelIndexerCache(config *rest.Config, opts cache.Options) (cache.Cache, error) {
	cache, err := cache.New(config, opts)
	if err != nil {
		return nil, err
	}

	lic := &labelIndexerCache{
		Cache: cache,
	}

	return lic, err
}

// List converts labelSelector to fieldSelector and returns the final result by call the List func of its under cache.
func (lic *labelIndexerCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// convert labelSelector to fieldSelector to use indexer constructed by labelIndexerManager
	selectors := make([]labels.Selector, 0)
	for _, opt := range opts {
		// only care about client.ListOptions type
		listOption, ok := opt.(*client.ListOptions)
		if ok {
			if listOption.LabelSelector != nil && !listOption.LabelSelector.Empty() {
				selectors = append(selectors, listOption.LabelSelector)
			}
		}
	}

	subIndexerKeys := make([]string, 0)
	for _, selector := range selectors {
		requirements, _ := selector.Requirements()
		for _, requirement := range requirements {
			if requirement.Operator() != selection.Equals {
				continue
			}
			for labelValue := range requirement.Values() {
				subIndexerKeys = append(subIndexerKeys, fmt.Sprintf("%s=%s", requirement.Key(), labelValue))
			}
		}
	}

	if len(subIndexerKeys) > 0 {
		opts = append(opts, &client.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.labels", strings.Join(subIndexerKeys, "&"))})
	}

	return lic.Cache.List(ctx, list, opts...)
}
