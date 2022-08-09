package manager

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// typeIndexerFuncMap contains internal type and its indexerFunc used to set up indexer.
var typeIndexerFuncMap = map[client.Object]client.IndexerFunc{
	&workv1alpha2.ResourceBinding{}:        indexForResourceBinding(),
	&workv1alpha1.ResourceBinding{}:        indexForResourceBinding(),
	&workv1alpha2.ClusterResourceBinding{}: indexForClusterResourceBinding(),
	&workv1alpha1.ClusterResourceBinding{}: indexForClusterResourceBinding(),
	&workv1alpha1.Work{}:                   indexForWork(),
}

type labelIndexerManager struct {
	manager.Manager
}

// NewLabelIndexerManager returns a labelIndexerManager which set up field indexer for all needed internal type using typeIndexerFuncMap.
// This file need to be refactored when controller-runtime support labelIndexer and client-go support multiIndexer in the future.
// refer to:
//		https://github.com/karmada-io/karmada/issues/2234
//		https://github.com/kubernetes/kubernetes/pull/109334
//      https://github.com/kubernetes-sigs/controller-runtime/pull/1838
func NewLabelIndexerManager(config *rest.Config, options manager.Options) (manager.Manager, error) {
	manager, err := manager.New(config, options)
	if err != nil {
		return nil, err
	}

	lim := &labelIndexerManager{
		Manager: manager,
	}

	// register label indexer
	ctx := context.Background()
	for obj, indexerFunc := range typeIndexerFuncMap {
		err := lim.Manager.GetFieldIndexer().IndexField(ctx, obj, "metadata.labels", indexerFunc)
		if err != nil {
			return nil, err
		}
	}

	return lim, nil
}

func indexForResourceBinding() client.IndexerFunc {
	return func(object client.Object) []string {
		return indexKeys(object, []string{
			policyv1alpha1.PropagationPolicyNamespaceLabel,
			policyv1alpha1.PropagationPolicyNameLabel,
			policyv1alpha1.ClusterPropagationPolicyLabel,
		})
	}
}

func indexForClusterResourceBinding() client.IndexerFunc {
	return func(object client.Object) []string {
		return indexKeys(object, []string{
			policyv1alpha1.ClusterPropagationPolicyLabel,
		})
	}
}

func indexForWork() client.IndexerFunc {
	return func(object client.Object) []string {
		return indexKeys(object, []string{
			workv1alpha2.ResourceBindingReferenceKey,
			workv1alpha2.ClusterResourceBindingReferenceKey,
			util.ServiceNamespaceLabel,
			util.ServiceNameLabel,
			util.FederatedResourceQuotaNamespaceLabel,
			util.FederatedResourceQuotaNameLabel,
		})
	}
}

func indexKeys(obj client.Object, keys []string) []string {
	var indices []string

	if labels := obj.GetLabels(); labels != nil {
		for _, labelKey := range keys {
			if labelValue, ok := labels[labelKey]; ok {
				indices = append(indices, fmt.Sprintf("%s=%s", labelKey, labelValue))
			}
		}

		if len(indices) > 0 {
			return []string{strings.Join(indices, "&")}
		}
	}

	return indices
}
