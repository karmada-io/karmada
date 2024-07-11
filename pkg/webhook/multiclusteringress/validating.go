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

package multiclusteringress

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ValidatingAdmission validates MultiClusterIngress object when creating/updating.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	mci := &networkingv1alpha1.MultiClusterIngress{}

	err := v.Decoder.Decode(req, mci)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Validating MultiClusterIngress(%s/%s) for request: %s", mci.Namespace, mci.Name, req.Operation)

	if req.Operation == admissionv1.Update {
		oldMci := &networkingv1alpha1.MultiClusterIngress{}
		err = v.Decoder.DecodeRaw(req.OldObject, oldMci)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if errs := validateMCIUpdate(oldMci, mci); len(errs) != 0 {
			klog.Errorf("%v", errs)
			return admission.Denied(errs.ToAggregate().Error())
		}
	} else {
		if errs := validateMCI(mci); len(errs) != 0 {
			klog.Errorf("%v", errs)
			return admission.Denied(errs.ToAggregate().Error())
		}
	}
	return admission.Allowed("")
}

func validateMCIUpdate(oldMci, newMci *networkingv1alpha1.MultiClusterIngress) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMetaUpdate(&newMci.ObjectMeta, &oldMci.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, validateMCI(newMci)...)
	allErrs = append(allErrs, lifted.ValidateIngressLoadBalancerStatus(&newMci.Status.LoadBalancer, field.NewPath("status", "loadBalancer"))...)
	return allErrs
}

func validateMCI(mci *networkingv1alpha1.MultiClusterIngress) field.ErrorList {
	allErrs := apimachineryvalidation.ValidateObjectMeta(&mci.ObjectMeta, true,
		apimachineryvalidation.NameIsDNSSubdomain, field.NewPath("metadata"))
	opts := lifted.IngressValidationOptions{
		AllowInvalidSecretName:       false,
		AllowInvalidWildcardHostRule: false,
	}
	allErrs = append(allErrs, lifted.ValidateIngressSpec(&mci.Spec, field.NewPath("spec"), opts)...)
	return allErrs
}
