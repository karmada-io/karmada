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
	decoder *admission.Decoder
}

// Check if our MutatingAdmission implements necessary interface
var _ admission.Handler = &MutatingAdmission{}
var _ admission.DecoderInjector = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
func (a *MutatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.OverridePolicy{}

	err := a.decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Set default namespace for all resource selector if not set.
	for i := range policy.Spec.ResourceSelectors {
		if len(policy.Spec.ResourceSelectors[i].Namespace) == 0 {
			klog.Infof("Setting resource selector default namespace for policy: %s/%s", policy.Namespace, policy.Name)
			policy.Spec.ResourceSelectors[i].Namespace = policy.Namespace
		}
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
