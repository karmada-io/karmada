package overridepolicy

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates OverridePolicy object when creating/updating/deleting.
type ValidatingAdmission struct {
	Decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.OverridePolicy{}

	err := v.Decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating OverridePolicy(%s/%s) for request: %s", policy.Namespace, policy.Name, req.Operation)

	if errs := validation.ValidateOverrideSpec(&policy.Spec); len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}

	return admission.Allowed("")
}
