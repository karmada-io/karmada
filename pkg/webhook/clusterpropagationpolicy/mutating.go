/*
Copyright The Karmada Authors.

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
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}
var _ admission.DecoderInjector = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterPropagationPolicy{}

	err := a.decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Set default spread constraints if both 'SpreadByField' and 'SpreadByLabel' not set.
	helper.SetDefaultSpreadConstraints(policy.Spec.Placement.SpreadConstraints)

	if len(policy.Name) > validation.LabelValueMaxLength {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("ClusterPropagationPolicy's name should be no more than %d characters", validation.LabelValueMaxLength))
	}

	addedResourceSelectors := helper.GetFollowedResourceSelectorsWhenMatchServiceImport(policy.Spec.ResourceSelectors)
	if addedResourceSelectors != nil {
		policy.Spec.ResourceSelectors = append(policy.Spec.ResourceSelectors, addedResourceSelectors...)
	}

	marshaledBytes, err := json.Marshal(policy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (a *MutatingAdmission) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
