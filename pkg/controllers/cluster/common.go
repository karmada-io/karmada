/*
Copyright 2022 The Karmada Authors.

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
