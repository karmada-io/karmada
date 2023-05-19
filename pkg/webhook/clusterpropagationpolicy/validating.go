package clusterpropagationpolicy

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates ClusterPropagationPolicy object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	policy := &policyv1alpha1.ClusterPropagationPolicy{}

	err := v.decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ClusterPropagationPolicy(%s) for request: %s", policy.Name, req.Operation)

	if req.Operation == admissionv1.Update {
		oldPolicy := &policyv1alpha1.ClusterPropagationPolicy{}
		err = v.decoder.DecodeRaw(req.OldObject, oldPolicy)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if policy.Spec.SchedulerName != oldPolicy.Spec.SchedulerName {
			err = fmt.Errorf("the schedulerName should not be updated")
			klog.Error(err)
			return admission.Denied(err.Error())
		}
	}

	errs := validation.ValidatePropagationSpec(policy.Spec)
	if len(errs) != 0 {
		klog.Error(errs)
		return admission.Denied(errs.ToAggregate().Error())
	}
	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (v *ValidatingAdmission) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
