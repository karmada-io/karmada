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

package resourceinterpretercustomization

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// ValidatingAdmission validates ResourceInterpreterCustomization object when creating/updating.
type ValidatingAdmission struct {
	client.Client
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	configuration := &configv1alpha1.ResourceInterpreterCustomization{}

	err := v.Decoder.Decode(req, configuration)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ResourceInterpreterCustomization(%s) for request: %s", configuration.Name, req.Operation)
	configs := &configv1alpha1.ResourceInterpreterCustomizationList{}
	if err = v.List(ctx, configs); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if err = validateResourceInterpreterCustomizations(configuration, configs); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}
