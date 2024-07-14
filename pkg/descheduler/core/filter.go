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

package core

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// TODO(Garrybest): make it as an option
var supportedGVKs = []schema.GroupVersionKind{
	appsv1.SchemeGroupVersion.WithKind("Deployment"),
}

// FilterBindings will filter ResourceBindings that could be descheduled
// based on their GVK and applied placement.
func FilterBindings(bindings []*workv1alpha2.ResourceBinding) []*workv1alpha2.ResourceBinding {
	var res []*workv1alpha2.ResourceBinding
	for _, binding := range bindings {
		if validateGVK(&binding.Spec.Resource) && validatePlacement(binding) {
			res = append(res, binding)
		}
	}
	return res
}

func validateGVK(reference *workv1alpha2.ObjectReference) bool {
	gvr := schema.FromAPIVersionAndKind(reference.APIVersion, reference.Kind)
	for i := range supportedGVKs {
		if gvr == supportedGVKs[i] {
			return true
		}
	}
	return false
}

func validatePlacement(binding *workv1alpha2.ResourceBinding) bool {
	placement, err := helper.GetAppliedPlacement(binding.Annotations)
	if err != nil {
		klog.ErrorS(err, "Failed to get applied placement when validating", "ResourceBinding", klog.KObj(binding))
		return false
	}
	if placement == nil {
		return false
	}
	return helper.IsReplicaDynamicDivided(placement)
}
