package core

import (
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
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
	// Check whether the policy allows rescheduling.
	appliedPlacement := util.GetLabelValue(binding.Annotations, util.PolicyPlacementAnnotation)
	if len(appliedPlacement) == 0 {
		return false
	}
	placement := &policyv1alpha1.Placement{}
	if err := json.Unmarshal([]byte(appliedPlacement), placement); err != nil {
		klog.ErrorS(err, "Failed to unmarshal placement when validating", "ResourceBinding", klog.KObj(binding))
		return false
	}
	return helper.IsReplicaDynamicDivided(placement.ReplicaScheduling)
}
