/*
Copyright 2021 The Karmada Authors.

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

package configuration

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// ValidatingAdmission validates ResourceInterpreterWebhookConfiguration object when creating/updating.
type ValidatingAdmission struct {
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	configuration := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}

	err := v.Decoder.Decode(req, configuration)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating ResourceInterpreterWebhookConfiguration(%s) for request: %s", configuration.Name, req.Operation)

	var allErrors field.ErrorList
	hookNames := sets.NewString()
	for i, hook := range configuration.Webhooks {
		allErrors = append(allErrors, validateWebhook(&configuration.Webhooks[i], field.NewPath("webhooks").Index(i))...)
		if hookNames.Has(hook.Name) {
			allErrors = append(allErrors, field.Duplicate(field.NewPath("webhooks").Index(i).Child("name"), hook.Name))
			continue
		}
		hookNames.Insert(hook.Name)
	}

	if len(allErrors) != 0 {
		klog.Error(allErrors.ToAggregate())
		return admission.Denied(allErrors.ToAggregate().Error())
	}

	return admission.Allowed("")
}

var supportedInterpreterOperation = sets.NewString(
	string(configv1alpha1.InterpreterOperationAll),
	string(configv1alpha1.InterpreterOperationInterpretReplica),
	string(configv1alpha1.InterpreterOperationInterpretDependency),
	string(configv1alpha1.InterpreterOperationReviseReplica),
	string(configv1alpha1.InterpreterOperationRetain),
	string(configv1alpha1.InterpreterOperationAggregateStatus),
	string(configv1alpha1.InterpreterOperationInterpretStatus),
	string(configv1alpha1.InterpreterOperationInterpretHealth),
)

var acceptedInterpreterContextVersions = []string{configv1alpha1.GroupVersion.Version}

func validateWebhook(hook *configv1alpha1.ResourceInterpreterWebhook, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	// hook.Name must be fully qualified
	allErrors = append(allErrors, validation.IsFullyQualifiedDomainName(fldPath.Child("name"), hook.Name)...)

	for i := range hook.Rules {
		allErrors = append(allErrors, validateRuleWithOperations(&hook.Rules[i], fldPath.Child("rules").Index(i))...)
	}

	if hook.TimeoutSeconds != nil && (*hook.TimeoutSeconds > 30 || *hook.TimeoutSeconds < 1) {
		allErrors = append(allErrors, field.Invalid(fldPath.Child("timeoutSeconds"), *hook.TimeoutSeconds, "the timeout value must be between 1 and 30 seconds"))
	}

	cc := hook.ClientConfig
	switch {
	case (cc.URL == nil) == (cc.Service == nil):
		allErrors = append(allErrors, field.Required(fldPath.Child("clientConfig"), "exactly one of url or service is required"))
	case cc.URL != nil:
		allErrors = append(allErrors, webhook.ValidateWebhookURL(fldPath.Child("clientConfig").Child("url"), *cc.URL, true)...)
	case cc.Service != nil:
		// Temporary fix: If the service port is not specified, set a default value of 443.
		// This is a workaround to prevent a panic when validating ResourceInterpreterWebhookConfiguration
		// with an unspecified service port. Ideally, this logic should be handled by a MutatingWebhook,
		// but introducing a MutatingWebhook at this stage would require significant changes and is not
		// convenient for cherry-picking to release branches. Therefore, we are temporarily setting the
		// default port here. Once a MutatingWebhook is implemented, this logic can be moved there.
		//
		// Note: The Interpreter framework also sets the same default value (443) when processing Service,
		// so the backend will not encounter issues due to missing port information.
		if cc.Service.Port == nil {
			cc.Service.Port = ptr.To[int32](443)
		}
		allErrors = append(allErrors, webhook.ValidateWebhookService(fldPath.Child("clientConfig").Child("service"), cc.Service.Namespace, cc.Service.Name, cc.Service.Path, *cc.Service.Port)...)
	}

	allErrors = append(allErrors, validateInterpreterContextVersions(hook.InterpreterContextVersions, fldPath.Child("interpreterContextVersions"))...)
	return allErrors
}

func hasWildcardOperation(operations []configv1alpha1.InterpreterOperation) bool {
	for _, o := range operations {
		if o == configv1alpha1.InterpreterOperationAll {
			return true
		}
	}
	return false
}

func validateRuleWithOperations(ruleWithOperations *configv1alpha1.RuleWithOperations, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	if len(ruleWithOperations.Operations) == 0 {
		allErrors = append(allErrors, field.Required(fldPath.Child("operations"), ""))
	}
	if len(ruleWithOperations.Operations) > 1 && hasWildcardOperation(ruleWithOperations.Operations) {
		allErrors = append(allErrors, field.Invalid(fldPath.Child("operations"), ruleWithOperations.Operations, "if '*' is present, must not specify other operations"))
	}
	for i, operation := range ruleWithOperations.Operations {
		if !supportedInterpreterOperation.Has(string(operation)) {
			allErrors = append(allErrors, field.NotSupported(fldPath.Child("operations").Index(i), operation, supportedInterpreterOperation.List()))
		}
	}
	allErrors = append(allErrors, validateRule(&ruleWithOperations.Rule, fldPath)...)
	return allErrors
}

func validateInterpreterContextVersions(versions []string, fldPath *field.Path) field.ErrorList {
	allErrors := field.ErrorList{}

	// Currently, only v1alpha1 accepted in InterpreterContextVersions
	if len(versions) < 1 {
		allErrors = append(allErrors, field.Required(fldPath, fmt.Sprintf("must specify one of %v", strings.Join(acceptedInterpreterContextVersions, ", "))))
	} else {
		visited := map[string]bool{}
		hasAcceptedVersion := false
		for i, v := range versions {
			if visited[v] {
				allErrors = append(allErrors, field.Invalid(fldPath.Index(i), v, "duplicate version"))
				continue
			}
			visited[v] = true
			for _, errString := range validation.IsDNS1035Label(v) {
				allErrors = append(allErrors, field.Invalid(fldPath.Index(i), v, errString))
			}
			if isAcceptedExploreReviewVersions(v) {
				hasAcceptedVersion = true
			}
		}
		if !hasAcceptedVersion {
			allErrors = append(allErrors, field.Invalid(
				fldPath, versions,
				fmt.Sprintf("must include at least one of %v", strings.Join(acceptedInterpreterContextVersions, ", "))))
		}
	}
	return allErrors
}

func isAcceptedExploreReviewVersions(v string) bool {
	for _, version := range acceptedInterpreterContextVersions {
		if v == version {
			return true
		}
	}
	return false
}
