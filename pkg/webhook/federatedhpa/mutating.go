package federatedhpa

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	federatedHPA := &autoscalingv1alpha1.FederatedHPA{}

	err := a.Decoder.Decode(req, federatedHPA)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(2).Infof("Mutating federatedHPA(%s/%s) for request: %s", federatedHPA.Namespace, federatedHPA.Name, req.Operation)

	lifted.SetDefaultsFederatedHPA(federatedHPA)

	marshaledBytes, err := json.Marshal(federatedHPA)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledBytes)
}
