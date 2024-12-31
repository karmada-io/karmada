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

package clusteroverridepolicy

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates ClusterOverridePolicy object when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterOverridePolicy{}

	err := v.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ClusterOverridePolicy(%s) for request: %s", policy.Name, req.Operation)

	if errs := validation.ValidateOverrideSpec(&policy.Spec); len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}
