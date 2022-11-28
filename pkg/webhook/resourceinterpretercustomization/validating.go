package resourceinterpretercustomization

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// ValidatingAdmission validates ResourceInterpreterCustomization object when creating/updating.
type ValidatingAdmission struct {
	client.Client
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	configuration := &configv1alpha1.ResourceInterpreterCustomization{}

	err := v.decoder.Decode(req, configuration)
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

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (v *ValidatingAdmission) InjectDecoder(decoder *admission.Decoder) error {
	v.decoder = decoder
	return nil
}
