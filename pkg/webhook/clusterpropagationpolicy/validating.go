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

package clusterpropagationpolicy

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates ClusterPropagationPolicy object when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterPropagationPolicy{}

	err := v.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ClusterPropagationPolicy(%s) for request: %s", policy.Name, req.Operation)

	if req.Operation == admissionv1.Update {
		oldPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
		err = v.Decoder.DecodeRaw(req.OldObject, oldPolicy)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if policy.Spec.SchedulerName != oldPolicy.Spec.SchedulerName {
			err = fmt.Errorf("the schedulerName should not be updated")
			klog.Error(err)
			return admission.Denied(err.Error())
		}

		if policy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] !=
			oldPolicy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel] {
			return admission.Denied(fmt.Sprintf("label %s is immutable, it can only be set by the system during creation",
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel))
		}
	}
	if _, exist := policy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]; !exist {
		return admission.Denied(fmt.Sprintf("label %s is required, it should be set by the mutating admission webhook during creation",
			policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel))
	}

	errs := validation.ValidatePropagationSpec(policy.Spec)
	if len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}
	return admission.Allowed("")
}
