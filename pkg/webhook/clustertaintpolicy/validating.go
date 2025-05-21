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

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// ValidatingAdmission validates ClusterTaintPolicy object when creating/updating.
type ValidatingAdmission struct {
	client.Client
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterTaintPolicy{}

	err := v.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ClusterTaintPolicy(%s) for request: %s", policy.Name, req.Operation)

	policyList := &policyv1alpha1.ClusterTaintPolicyList{}
	if err = v.List(ctx, policyList); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	errs := validatePolicySpec(policy.Spec)
	if len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	if err = validatePolicyTaintsWithOtherPolicies(policy, policyList.Items); err != nil {
		klog.Error(err)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func validatePolicySpec(spec policyv1alpha1.ClusterTaintPolicySpec) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = append(allErrs, validatePolicyTaints(spec.Taints, field.NewPath("spec").Child("taints"))...)
	return allErrs
}

func validatePolicyTaints(taints []policyv1alpha1.Taint, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for i := 0; i < len(taints); i++ {
		for j := i + 1; j < len(taints); j++ {
			if taints[i].Key == taints[j].Key && taints[i].Effect == taints[j].Effect {
				allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), fmt.Sprintf("taint(%s:%s) already exist",
					taints[i].Key, taints[i].Effect)))
				break
			}
		}
	}
	return allErrs
}

func validatePolicyTaintsWithOtherPolicies(newPolicy *policyv1alpha1.ClusterTaintPolicy, policies []policyv1alpha1.ClusterTaintPolicy) error {
	var errs []error
	for _, taint := range newPolicy.Spec.Taints {
		err := validateTaintWithPolicies(newPolicy.Name, taint, policies)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func validateTaintWithPolicies(policyName string, newTaint policyv1alpha1.Taint, policies []policyv1alpha1.ClusterTaintPolicy) error {
	for _, oldPolicy := range policies {
		if oldPolicy.Name == policyName {
			continue
		}

		for _, taint := range oldPolicy.Spec.Taints {
			if newTaint.Key == taint.Key && newTaint.Effect == taint.Effect {
				return fmt.Errorf("add the same taint(%s:%s) conflicted with the exist clusterTaintPolicy(%s)",
					taint.Key, taint.Effect, oldPolicy.Name)
			}
		}
	}
	return nil
}
