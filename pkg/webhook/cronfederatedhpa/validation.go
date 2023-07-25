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

package cronfederatedhpa

import (
	"context"
	"fmt"
	"net/http"
	"time"
	_ "time/tzdata"

	"github.com/adhocore/gronx"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/lifted"
)

// ValidatingAdmission validates CronFederatedHPA object when creating/updating.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	cronFHPA := &autoscalingv1alpha1.CronFederatedHPA{}

	err := v.decoder.Decode(req, cronFHPA)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating CronFederatedHPA(%s) for request: %s", klog.KObj(cronFHPA).String(), req.Operation)

	errs := field.ErrorList{}
	errs = append(errs, apimachineryvalidation.ValidateObjectMeta(&cronFHPA.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	errs = append(errs, validateCronFederatedHPASpec(&cronFHPA.Spec, field.NewPath("spec"))...)

	if len(errs) != 0 {
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

// validateCronFederatedHPASpec validates CronFederatedHPA spec
func validateCronFederatedHPASpec(spec *autoscalingv1alpha1.CronFederatedHPASpec, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	scaleFHPA := false

	scaleTargetRef := spec.ScaleTargetRef
	if scaleTargetRef.APIVersion == autoscalingv1alpha1.GroupVersion.String() {
		if scaleTargetRef.Kind != autoscalingv1alpha1.FederatedHPAKind {
			kindFldPath := fldPath.Child("scaleTargetRef").Child("kind")
			fldError := field.Invalid(kindFldPath, scaleTargetRef.Kind,
				fmt.Sprintf("invalid scaleTargetRef kind: %s, only support %s", scaleTargetRef.Kind, autoscalingv1alpha1.FederatedHPAKind))
			errs = append(errs, fldError)
			return errs
		}
		scaleFHPA = true
	}

	errs = append(errs, lifted.ValidateCrossVersionObjectReference(scaleTargetRef, fldPath.Child("scaleTargetRef"))...)
	errs = append(errs, validateCronFederatedHPARules(spec.Rules, scaleFHPA, scaleTargetRef.Kind, fldPath.Child("rules"))...)

	return errs
}

// validateCronFederatedHPARules validates CronFederatedHPA rules
func validateCronFederatedHPARules(rules []autoscalingv1alpha1.CronFederatedHPARule,
	scaleFHPA bool, scaleTargetKind string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	ruleNameSet := sets.NewString()
	for index, rule := range rules {
		if ruleNameSet.Has(rule.Name) {
			errs = append(errs, field.Duplicate(fldPath.Index(index).Child("name"), rule.Name))
		}
		ruleNameSet.Insert(rule.Name)

		// Validate cron format
		cronValidator := gronx.New()
		if !cronValidator.IsValid(rule.Schedule) {
			errs = append(errs, field.Invalid(fldPath.Index(index).Child("schedule"), rule.Schedule, "invalid cron format"))
		}

		// Validate timezone
		if rule.TimeZone != nil {
			_, err := time.LoadLocation(*rule.TimeZone)
			if err != nil {
				errs = append(errs, field.Invalid(fldPath.Index(index).Child("timeZone"), rule.TimeZone, err.Error()))
			}
		}

		errs = append(errs, validateCronFederatedHPAScalingReplicas(rule, scaleFHPA, scaleTargetKind, fldPath.Index(index))...)
	}

	return errs
}

// validateCronFederatedHPAScalingReplicas validates CronFederatedHPA rules' scaling replicas
func validateCronFederatedHPAScalingReplicas(rule autoscalingv1alpha1.CronFederatedHPARule, scaleFHPA bool,
	scaleTargetKind string, fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	if scaleFHPA {
		// Validate targetMinReplicas and targetMaxReplicas
		if rule.TargetMinReplicas == nil && rule.TargetMaxReplicas == nil {
			errs = append(errs, field.Invalid(fldPath.Child("targetMaxReplicas"), "",
				"targetMinReplicas and targetMaxReplicas cannot be nil at the same time if you want to scale FederatedHPA"))
		}
		if pointer.Int32Deref(rule.TargetMinReplicas, 1) <= 0 {
			errs = append(errs, field.Invalid(fldPath.Child("targetMinReplicas"), "",
				"targetMinReplicas should be larger than 0"))
		}
		if pointer.Int32Deref(rule.TargetMaxReplicas, 1) <= 0 {
			errs = append(errs, field.Invalid(fldPath.Child("targetMaxReplicas"), "",
				"targetMaxReplicas should be larger than 0"))
		}
		if pointer.Int32Deref(rule.TargetMinReplicas, 1) > pointer.Int32Deref(rule.TargetMaxReplicas, 1) {
			errs = append(errs, field.Invalid(fldPath.Child("targetMinReplicas"), "",
				"targetMaxReplicas should be larger than or equal to targetMinReplicas"))
		}
		return errs
	}

	// Validate targetReplicas
	if rule.TargetReplicas == nil {
		errMsg := fmt.Sprintf("targetReplicas cannot be nil if you want to scale %s", scaleTargetKind)
		errs = append(errs, field.Invalid(fldPath.Child("targetReplicas"), "", errMsg))
	}
	// It's allowed to scale to 0, such as remove all the replicas when weekends
	if pointer.Int32Deref(rule.TargetReplicas, 0) < 0 {
		errs = append(errs, field.Invalid(fldPath.Child("targetReplicas"), "",
			"targetReplicas should be larger than or equal to 0"))
	}

	return errs
}
