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

package helper

import (
	"context"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

// GetWorksByLabelsSet gets WorkList by matching labels.Set.
func GetWorksByLabelsSet(ctx context.Context, c client.Client, ls labels.Set) (*workv1alpha1.WorkList, error) {
	workList := &workv1alpha1.WorkList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return workList, c.List(ctx, workList, listOpt)
}

// GetWorksByBindingID gets WorkList by matching same binding's permanent id.
// Caller should ensure Work is indexed by binding's permanent id.
func GetWorksByBindingID(ctx context.Context, c client.Client, bindingID string, namespaced bool) (*workv1alpha1.WorkList, error) {
	var key string
	if namespaced {
		key = indexregistry.WorkIndexByLabelResourceBindingID
	} else {
		key = indexregistry.WorkIndexByLabelClusterResourceBindingID
	}
	workList := &workv1alpha1.WorkList{}
	listOpt := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(key, bindingID),
	}
	return workList, c.List(ctx, workList, listOpt)
}
