/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	capilabels "sigs.k8s.io/cluster-api/internal/labels"
	"sigs.k8s.io/cluster-api/util/version"
)

func (m *MachineSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1beta1-machineset,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinesets,versions=v1beta1,name=validation.machineset.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1beta1-machineset,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinesets,versions=v1beta1,name=default.machineset.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &MachineSet{}
var _ webhook.Validator = &MachineSet{}

// Default sets default MachineSet field values.
func (m *MachineSet) Default() {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterNameLabel] = m.Spec.ClusterName

	if m.Spec.DeletePolicy == "" {
		randomPolicy := string(RandomMachineSetDeletePolicy)
		m.Spec.DeletePolicy = randomPolicy
	}

	if m.Spec.Selector.MatchLabels == nil {
		m.Spec.Selector.MatchLabels = make(map[string]string)
	}

	if m.Spec.Template.Labels == nil {
		m.Spec.Template.Labels = make(map[string]string)
	}

	if len(m.Spec.Selector.MatchLabels) == 0 && len(m.Spec.Selector.MatchExpressions) == 0 {
		// Note: MustFormatValue is used here as the value of this label will be a hash if the MachineSet name is longer than 63 characters.
		m.Spec.Selector.MatchLabels[MachineSetNameLabel] = capilabels.MustFormatValue(m.Name)
		m.Spec.Template.Labels[MachineSetNameLabel] = capilabels.MustFormatValue(m.Name)
	}

	if m.Spec.Template.Spec.Version != nil && !strings.HasPrefix(*m.Spec.Template.Spec.Version, "v") {
		normalizedVersion := "v" + *m.Spec.Template.Spec.Version
		m.Spec.Template.Spec.Version = &normalizedVersion
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineSet) ValidateCreate() error {
	return m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineSet) ValidateUpdate(old runtime.Object) error {
	oldMS, ok := old.(*MachineSet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a MachineSet but got a %T", old))
	}
	return m.validate(oldMS)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineSet) ValidateDelete() error {
	return nil
}

func (m *MachineSet) validate(old *MachineSet) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")
	selector, err := metav1.LabelSelectorAsSelector(&m.Spec.Selector)
	if err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(
				specPath.Child("selector"),
				m.Spec.Selector,
				err.Error(),
			),
		)
	} else if !selector.Matches(labels.Set(m.Spec.Template.Labels)) {
		allErrs = append(
			allErrs,
			field.Invalid(
				specPath.Child("template", "metadata", "labels"),
				m.Spec.Template.ObjectMeta.Labels,
				fmt.Sprintf("must match spec.selector %q", selector.String()),
			),
		)
	}

	if old != nil && old.Spec.ClusterName != m.Spec.ClusterName {
		allErrs = append(
			allErrs,
			field.Forbidden(
				specPath.Child("clusterName"),
				"field is immutable",
			),
		)
	}

	if m.Spec.Template.Spec.Version != nil {
		if !version.KubeSemver.MatchString(*m.Spec.Template.Spec.Version) {
			allErrs = append(
				allErrs,
				field.Invalid(
					specPath.Child("template", "spec", "version"),
					*m.Spec.Template.Spec.Version,
					"must be a valid semantic version",
				),
			)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("MachineSet").GroupKind(), m.Name, allErrs)
}
