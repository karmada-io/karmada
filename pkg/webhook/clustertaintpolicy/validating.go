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

package clustertaintpolicy

import (
	"context"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// ValidatingAdmission validates ClusterTaintPolicy object when creating/updating.
type ValidatingAdmission struct {
	Decoder                   admission.Decoder
	AllowNoExecuteTaintPolicy bool
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterTaintPolicy{}

	err := v.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ClusterTaintPolicy(%s) for request: %s", policy.Name, req.Operation)

	errs := validatePolicySpec(policy.Spec, v.AllowNoExecuteTaintPolicy)
	if len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}

func validatePolicySpec(spec policyv1alpha1.ClusterTaintPolicySpec, allowNoExecute bool) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, validatePolicyTaints(spec.Taints, field.NewPath("spec").Child("taints"), allowNoExecute)...)
	return allErrs
}

func validatePolicyTaints(taints []policyv1alpha1.Taint, fldPath *field.Path, allowNoExecute bool) field.ErrorList {
	var allErrs field.ErrorList
	for i := 0; i < len(taints); i++ {
		if !allowNoExecute && taints[i].Effect == corev1.TaintEffectNoExecute {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i), "Configuring taint with NoExecute effect is not allowed, this capability must be explicitly enabled by administrators through command-line flags"))
		}

		for j := i + 1; j < len(taints); j++ {
			if taints[i].Key == taints[j].Key && taints[i].Effect == taints[j].Effect {
				allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), fmt.Sprintf("Duplicate taint with the same key(%s) and effect(%s) is not allowed",
					taints[i].Key, taints[i].Effect)))
				break
			}
		}
	}
	return allErrs
}
