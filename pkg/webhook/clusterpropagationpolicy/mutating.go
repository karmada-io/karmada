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
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// NewMutatingHandler builds a new admission.Handler.
func NewMutatingHandler(decoder admission.Decoder) admission.Handler {
	return &MutatingAdmission{
		Decoder: decoder,
	}
}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterPropagationPolicy{}

	err := a.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating ClusterPropagationPolicy(%s/%s) for request: %s", req.Namespace, policy.Name, req.Operation)

	// Set default spread constraints if both 'SpreadByField' and 'SpreadByLabel' not set.
	helper.SetDefaultSpreadConstraints(policy.Spec.Placement.SpreadConstraints)

	if helper.ContainsServiceImport(policy.Spec.ResourceSelectors) {
		policy.Spec.PropagateDeps = true
	}

	// When ReplicaSchedulingType is Divided, set the default value of ReplicaDivisionPreference to Weighted.
	helper.SetReplicaDivisionPreferenceWeighted(&policy.Spec.Placement)

	if policy.Spec.Failover != nil {
		if policy.Spec.Failover.Application != nil {
			helper.SetDefaultGracePeriodSeconds(policy.Spec.Failover.Application)
		}
	}

	if req.Operation == admissionv1.Create {
		util.MergeLabel(policy, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel, uuid.New().String())
		controllerutil.AddFinalizer(policy, util.ClusterPropagationPolicyControllerFinalizer)
	}

	marshaledBytes, err := json.Marshal(policy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
