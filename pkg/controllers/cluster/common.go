package cluster

import (
	"context"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	rbClusterKeyIndex  = "rbSpec.clusters"
	crbClusterKeyIndex = "crbSpec.clusters"
)

// IndexField registers Indexer functions to controller manager.
func IndexField(mgr controllerruntime.Manager) error {
	rbIndexerFunc := func(obj client.Object) []string {
		rb, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			return nil
		}

		res := sets.NewString(util.GetBindingClusterNames(&rb.Spec)...)
		res.Insert(util.GetEvictionClusterNames(&rb.Spec)...)
		return res.List()
	}

	crbIndexerFunc := func(obj client.Object) []string {
		crb, ok := obj.(*workv1alpha2.ClusterResourceBinding)
		if !ok {
			return nil
		}

		res := sets.NewString(util.GetBindingClusterNames(&crb.Spec)...)
		res.Insert(util.GetEvictionClusterNames(&crb.Spec)...)
		return res.List()
	}

	return utilerrors.NewAggregate([]error{
		mgr.GetFieldIndexer().IndexField(context.TODO(), &workv1alpha2.ResourceBinding{}, rbClusterKeyIndex, rbIndexerFunc),
		mgr.GetFieldIndexer().IndexField(context.TODO(), &workv1alpha2.ClusterResourceBinding{}, crbClusterKeyIndex, crbIndexerFunc),
	})
}
