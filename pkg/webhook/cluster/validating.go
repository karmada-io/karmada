package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

// ValidatingAdmission validates cluster object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &clusterv1alpha1.Cluster{}

	err := v.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating cluster(%s) for request: %s", cluster.Name, req.Operation)

	if errs := validation.ValidateClusterName(cluster.Name); len(errs) != 0 {
		errMsg := fmt.Sprintf("invalid cluster name(%s): %s", cluster.Name, strings.Join(errs, ";"))
		klog.Info(errMsg)
		return admission.Denied(errMsg)
	}

	if len(cluster.Spec.ProxyURL) > 0 {
		if errs := validation.ValidateClusterProxyURL(cluster.Spec.ProxyURL); len(errs) != 0 {
			errMsg := fmt.Sprintf("invalid proxy URL(%s): %s", cluster.Spec.ProxyURL, strings.Join(errs, ";"))
			klog.Info(errMsg)
			return admission.Denied(errMsg)
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
