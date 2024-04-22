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

package workloadrebalancer

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

// ValidatingAdmission validates WorkloadRebalancer object when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	rebalancer := &appsv1alpha1.WorkloadRebalancer{}

	err := v.Decoder.Decode(req, rebalancer)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating WorkloadRebalancer(%s) for request: %s", rebalancer.Name, req.Operation)

	if req.Operation == admissionv1.Update {
		oldRebalancer := &appsv1alpha1.WorkloadRebalancer{}
		err = v.Decoder.DecodeRaw(req.OldObject, oldRebalancer)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if !reflect.DeepEqual(rebalancer.Spec, oldRebalancer.Spec) {
			err = fmt.Errorf("the spec field should not be modified")
			klog.Error(err)
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}
