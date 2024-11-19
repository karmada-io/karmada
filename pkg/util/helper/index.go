package helper

import (
	"context"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// IndexWork creates index for Work.
func IndexWork(ctx context.Context, mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, workv1alpha2.ResourceBindingPermanentIDLabel,
		IndexerFuncBasedOnLabel(workv1alpha2.ResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	err = mgr.GetFieldIndexer().IndexField(ctx, &workv1alpha1.Work{}, workv1alpha2.ClusterResourceBindingPermanentIDLabel,
		IndexerFuncBasedOnLabel(workv1alpha2.ClusterResourceBindingPermanentIDLabel))
	if err != nil {
		klog.Errorf("failed to create index for work, err: %v", err)
		return err
	}
	return nil
}

// IndexerFuncBasedOnLabel returns an IndexerFunc used to index resource with the given key as label key.
func IndexerFuncBasedOnLabel(key string) client.IndexerFunc {
	return func(obj client.Object) []string {
		val, ok := obj.GetLabels()[key]
		if !ok {
			return nil
		}
		return []string{val}
	}
}
