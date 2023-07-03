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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
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

	if errs := v.validateCronFederatedHPASpec(cronFHPA); len(errs) != 0 {
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
func (v *ValidatingAdmission) validateCronFederatedHPASpec(cronFHPA *autoscalingv1alpha1.CronFederatedHPA) field.ErrorList {
	errs := field.ErrorList{}
	scaleFHPA := false

	scaleTargetRef := cronFHPA.Spec.ScaleTargetRef
	if scaleTargetRef.APIVersion == autoscalingv1alpha1.GroupVersion.String() {
		if scaleTargetRef.Kind != autoscalingv1alpha1.FederatedHPAKind {
			kindFieldPath := field.NewPath("spec").Child("scaleTargetRef").Child("kind")
			fieldError := field.Invalid(kindFieldPath, scaleTargetRef.Kind,
				fmt.Sprintf("invalid scaleTargetRef kind: %s, only support %s", scaleTargetRef.Kind, autoscalingv1alpha1.FederatedHPAKind))
			errs = append(errs, fieldError)
			return errs
		}
		scaleFHPA = true
	}

	errs = append(errs, v.validateCronFederatedHPARules(cronFHPA.Spec.Rules, scaleFHPA, scaleTargetRef.Kind)...)

	return errs
}

// validateCronFederatedHPARules validates CronFederatedHPA rules
func (v *ValidatingAdmission) validateCronFederatedHPARules(rules []autoscalingv1alpha1.CronFederatedHPARule,
	scaleFHPA bool, scaleTargetKind string) field.ErrorList {
	errs := field.ErrorList{}

	ruleFieldPath := field.NewPath("spec").Child("rules")
	ruleNameSet := sets.NewString()
	for index, rule := range rules {
		if ruleNameSet.Has(rule.Name) {
			errs = append(errs, field.Duplicate(field.NewPath("spec").
				Child("rules").Index(index).Child("name"), rule.Name))
		}
		ruleNameSet.Insert(rule.Name)

		// Validate cron format
		cronValidator := gronx.New()
		if !cronValidator.IsValid(rule.Schedule) {
			errs = append(errs, field.Invalid(ruleFieldPath.Index(index).Child("schedule"), rule.Schedule, "invalid cron format"))
		}

		// Validate timezone
		if rule.TimeZone != nil {
			_, err := time.LoadLocation(*rule.TimeZone)
			if err != nil {
				errs = append(errs, field.Invalid(ruleFieldPath.Index(index).Child("timeZone"), rule.TimeZone, err.Error()))
			}
		}

		if scaleFHPA {
			// Validate targetMinReplicas and targetMaxReplicas
			if rule.TargetMinReplicas == nil && rule.TargetMaxReplicas == nil {
				errMsg := "targetMinReplicas and targetMaxReplicas cannot be nil at the same time if you want to scale FederatedHPA"
				errs = append(errs, field.Invalid(ruleFieldPath.Index(index), "", errMsg))
			}
			continue
		}

		// Validate targetReplicas
		if rule.TargetReplicas == nil {
			errMsg := fmt.Sprintf("targetReplicas cannot be nil if you want to scale %s", scaleTargetKind)
			errs = append(errs, field.Invalid(ruleFieldPath.Index(index), "", errMsg))
		}
	}

	return errs
}
