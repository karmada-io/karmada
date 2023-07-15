package federatedhpa

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ValidatingAdmission validates FederatedHPA object when creating/updating.
type ValidatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	fhpa := &autoscalingv1alpha1.FederatedHPA{}

	err := v.Decoder.Decode(req, fhpa)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating FederatedHPA(%s) for request: %s", klog.KObj(fhpa).String(), req.Operation)

	if errs := lifted.ValidateFederatedHPA(fhpa); len(errs) != 0 {
		klog.Errorf("%v", errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}
