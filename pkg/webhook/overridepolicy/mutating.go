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

package overridepolicy

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.OverridePolicy{}

	err := a.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Set default namespace for all resource selector if not set.
	// We need to get the default namespace from the request because for kube-apiserver < v1.24,
	// the namespace of the object with namespace unset in the mutating webook chain of a create request
	// is not populated yet. See https://github.com/kubernetes/kubernetes/pull/94637 for more details.
	for i := range policy.Spec.ResourceSelectors {
		if len(policy.Spec.ResourceSelectors[i].Namespace) == 0 {
			klog.Infof("Setting resource selector default namespace for policy: %s/%s", req.Namespace, policy.Name)
			policy.Spec.ResourceSelectors[i].Namespace = req.Namespace
		}
	}

	marshaledBytes, err := json.Marshal(policy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
