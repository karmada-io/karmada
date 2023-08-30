package resourcebinding

import (
	"context"
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	binding := &workv1alpha2.ResourceBinding{}

	err := a.Decoder.Decode(req, binding)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// part of mcs implementation, always set Service kind ResourceBinding's propagateDeps to true
	if binding.Spec.Resource.APIVersion == "v1" && binding.Spec.Resource.Kind == util.ServiceKind {
		binding.Spec.PropagateDeps = true
	}

	marshaledBytes, err := json.Marshal(binding)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
