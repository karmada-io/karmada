package propagationpolicy

import (
	"context"
	"net/http"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates PropagationPolicy object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.PropagationPolicy{}

	err := v.decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating PropagationPolicy(%s/%s) for request: %s", policy.Namespace, policy.Name, req.Operation)

	if err := validation.ValidateSpreadConstraint(policy.Spec.Placement.SpreadConstraints); err != nil {
		klog.Error(err)
		return admission.Denied(err.Error())
	}

	if policy.Spec.Placement.ClusterAffinity != nil && policy.Spec.Placement.ClusterAffinity.FieldSelector != nil {
		err := validation.ValidatePolicyFieldSelector(policy.Spec.Placement.ClusterAffinity.FieldSelector)
		if err != nil {
			klog.Error(err)
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (v *ValidatingAdmission) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
