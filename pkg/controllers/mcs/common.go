/*
Copyright 2025 The Karmada Authors.

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

package mcs

import (
	"context"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	workSuspendDispatchingIndex = "workSpec.suspendDispatching"
)

// IndexField registers Indexer functions to controller manager.
func IndexField(mgr controllerruntime.Manager) error {
	workIndexerFunc := func(obj client.Object) []string {
		work, ok := obj.(*workv1alpha1.Work)
		if !ok {
			return nil
		}
		return util.GetWorkSuspendDispatching(&work.Spec)
	}

	return utilerrors.NewAggregate([]error{
		mgr.GetFieldIndexer().IndexField(context.TODO(), &workv1alpha1.Work{}, workSuspendDispatchingIndex, workIndexerFunc),
	})
}
