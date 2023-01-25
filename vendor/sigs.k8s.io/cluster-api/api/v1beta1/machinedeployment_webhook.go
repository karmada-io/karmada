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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/util/version"
)

func (m *MachineDeployment) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1beta1-machinedeployment,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinedeployments,versions=v1beta1,name=validation.machinedeployment.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1beta1-machinedeployment,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinedeployments,versions=v1beta1,name=default.machinedeployment.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &MachineDeployment{}
var _ webhook.Validator = &MachineDeployment{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (m *MachineDeployment) Default() {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterNameLabel] = m.Spec.ClusterName

	if m.Spec.MinReadySeconds == nil {
		m.Spec.MinReadySeconds = pointer.Int32(0)
	}

	if m.Spec.RevisionHistoryLimit == nil {
		m.Spec.RevisionHistoryLimit = pointer.Int32(1)
	}

	if m.Spec.ProgressDeadlineSeconds == nil {
		m.Spec.ProgressDeadlineSeconds = pointer.Int32(600)
	}

	if m.Spec.Selector.MatchLabels == nil {
		m.Spec.Selector.MatchLabels = make(map[string]string)
	}

	if m.Spec.Strategy == nil {
		m.Spec.Strategy = &MachineDeploymentStrategy{}
	}

	if m.Spec.Strategy.Type == "" {
		m.Spec.Strategy.Type = RollingUpdateMachineDeploymentStrategyType
	}

	if m.Spec.Template.Labels == nil {
		m.Spec.Template.Labels = make(map[string]string)
	}

	// Default RollingUpdate strategy only if strategy type is RollingUpdate.
	if m.Spec.Strategy.Type == RollingUpdateMachineDeploymentStrategyType {
		if m.Spec.Strategy.RollingUpdate == nil {
			m.Spec.Strategy.RollingUpdate = &MachineRollingUpdateDeployment{}
		}
		if m.Spec.Strategy.RollingUpdate.MaxSurge == nil {
			ios1 := intstr.FromInt(1)
			m.Spec.Strategy.RollingUpdate.MaxSurge = &ios1
		}
		if m.Spec.Strategy.RollingUpdate.MaxUnavailable == nil {
			ios0 := intstr.FromInt(0)
			m.Spec.Strategy.RollingUpdate.MaxUnavailable = &ios0
		}
	}

	// If no selector has been provided, add label and selector for the
	// MachineDeployment's name as a default way of providing uniqueness.
	if len(m.Spec.Selector.MatchLabels) == 0 && len(m.Spec.Selector.MatchExpressions) == 0 {
		m.Spec.Selector.MatchLabels[MachineDeploymentNameLabel] = m.Name
		m.Spec.Template.Labels[MachineDeploymentNameLabel] = m.Name
	}
	// Make sure selector and template to be in the same cluster.
	m.Spec.Selector.MatchLabels[ClusterNameLabel] = m.Spec.ClusterName
	m.Spec.Template.Labels[ClusterNameLabel] = m.Spec.ClusterName

	// tolerate version strings without a "v" prefix: prepend it if it's not there
	if m.Spec.Template.Spec.Version != nil && !strings.HasPrefix(*m.Spec.Template.Spec.Version, "v") {
		normalizedVersion := "v" + *m.Spec.Template.Spec.Version
		m.Spec.Template.Spec.Version = &normalizedVersion
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateCreate() error {
	return m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateUpdate(old runtime.Object) error {
	oldMD, ok := old.(*MachineDeployment)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a MachineDeployment but got a %T", old))
	}
	return m.validate(oldMD)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateDelete() error {
	return nil
}

func (m *MachineDeployment) validate(old *MachineDeployment) error {
	var allErrs field.ErrorList
	// The MachineDeployment name is used as a label value. This check ensures names which are not be valid label values are rejected.
	if errs := validation.IsValidLabelValue(m.Name); len(errs) != 0 {
		for _, err := range errs {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("metadata", "name"),
					m.Name,
					fmt.Sprintf("must be a valid label value: %s", err),
				),
			)
		}
	}
	specPath := field.NewPath("spec")
	selector, err := metav1.LabelSelectorAsSelector(&m.Spec.Selector)
	if err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(specPath.Child("selector"), m.Spec.Selector, err.Error()),
		)
	} else if !selector.Matches(labels.Set(m.Spec.Template.Labels)) {
		allErrs = append(
			allErrs,
			field.Forbidden(
				specPath.Child("template", "metadata", "labels"),
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

	if m.Spec.Strategy != nil && m.Spec.Strategy.RollingUpdate != nil {
		total := 1
		if m.Spec.Replicas != nil {
			total = int(*m.Spec.Replicas)
		}

		if m.Spec.Strategy.RollingUpdate.MaxSurge != nil {
			if _, err := intstr.GetScaledValueFromIntOrPercent(m.Spec.Strategy.RollingUpdate.MaxSurge, total, true); err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(specPath.Child("strategy", "rollingUpdate", "maxSurge"),
						m.Spec.Strategy.RollingUpdate.MaxSurge, fmt.Sprintf("must be either an int or a percentage: %v", err.Error())),
				)
			}
		}

		if m.Spec.Strategy.RollingUpdate.MaxUnavailable != nil {
			if _, err := intstr.GetScaledValueFromIntOrPercent(m.Spec.Strategy.RollingUpdate.MaxUnavailable, total, true); err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(specPath.Child("strategy", "rollingUpdate", "maxUnavailable"),
						m.Spec.Strategy.RollingUpdate.MaxUnavailable, fmt.Sprintf("must be either an int or a percentage: %v", err.Error())),
				)
			}
		}
	}

	if m.Spec.Template.Spec.Version != nil {
		if !version.KubeSemver.MatchString(*m.Spec.Template.Spec.Version) {
			allErrs = append(allErrs, field.Invalid(specPath.Child("template", "spec", "version"), *m.Spec.Template.Spec.Version, "must be a valid semantic version"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("MachineDeployment").GroupKind(), m.Name, allErrs)
}
