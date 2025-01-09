/*
Copyright 2024 The Karmada Authors.

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

package ctrlutil

import workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"

// WorkOption is a function that applies changes to a Work object.
// It is used to configure Work fields for clients of CreateOrUpdateWork.
type WorkOption func(work *workv1alpha1.Work)

// WithSuspendDispatching sets the SuspendDispatching field of the Work Spec.
func WithSuspendDispatching(suspendDispatching bool) WorkOption {
	return func(work *workv1alpha1.Work) {
		work.Spec.SuspendDispatching = &suspendDispatching
	}
}

// WithPreserveResourcesOnDeletion sets the PreserveResourcesOnDeletion field of the Work Spec.
func WithPreserveResourcesOnDeletion(preserveResourcesOnDeletion bool) WorkOption {
	return func(work *workv1alpha1.Work) {
		work.Spec.PreserveResourcesOnDeletion = &preserveResourcesOnDeletion
	}
}

func applyWorkOptions(work *workv1alpha1.Work, options []WorkOption) {
	for _, option := range options {
		option(work)
	}
}
