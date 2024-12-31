/*
Copyright 2023 The Karmada Authors.

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

package resourcebinding

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	rb := &workv1alpha2.ResourceBinding{}

	err := a.Decoder.Decode(req, rb)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Mutating resourceBinding(%s/%s) for request: %s", rb.Namespace, rb.Name, req.Operation)

	if util.GetLabelValue(rb.Labels, workv1alpha2.ResourceBindingPermanentIDLabel) == "" {
		util.MergeLabel(rb, workv1alpha2.ResourceBindingPermanentIDLabel, uuid.New().String())
	}

	marshaledBytes, err := json.Marshal(rb)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
