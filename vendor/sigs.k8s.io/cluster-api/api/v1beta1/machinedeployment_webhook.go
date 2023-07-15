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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/admission/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/version"
)

func (m *MachineDeployment) SetupWebhookWithManager(mgr ctrl.Manager) error {
	// This registers MachineDeployment as a validating webhook and
	// machineDeploymentDefaulter as a defaulting webhook.
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		WithDefaulter(MachineDeploymentDefaulter(mgr.GetScheme())).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1beta1-machinedeployment,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinedeployments,versions=v1beta1,name=validation.machinedeployment.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1beta1-machinedeployment,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinedeployments,versions=v1beta1,name=default.machinedeployment.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.CustomDefaulter = &machineDeploymentDefaulter{}
var _ webhook.Validator = &MachineDeployment{}

// MachineDeploymentDefaulter creates a new CustomDefaulter for MachineDeployments.
func MachineDeploymentDefaulter(scheme *runtime.Scheme) webhook.CustomDefaulter {
	return &machineDeploymentDefaulter{
		decoder: admission.NewDecoder(scheme),
	}
}

// machineDeploymentDefaulter implements a defaulting webhook for MachineDeployment.
type machineDeploymentDefaulter struct {
	decoder *admission.Decoder
}

// Default implements webhook.CustomDefaulter.
func (webhook *machineDeploymentDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	m, ok := obj.(*MachineDeployment)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a MachineDeployment but got a %T", obj))
	}

	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	dryRun := false
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	var oldMD *MachineDeployment
	if req.Operation == v1.Update {
		oldMD = &MachineDeployment{}
		if err := webhook.decoder.DecodeRaw(req.OldObject, oldMD); err != nil {
			return errors.Wrapf(err, "failed to decode oldObject to MachineDeployment")
		}
	}

	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterNameLabel] = m.Spec.ClusterName

	replicas, err := calculateMachineDeploymentReplicas(ctx, oldMD, m, dryRun)
	if err != nil {
		return err
	}
	m.Spec.Replicas = pointer.Int32(replicas)

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

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateCreate() (admission.Warnings, error) {
	return nil, m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldMD, ok := old.(*MachineDeployment)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a MachineDeployment but got a %T", old))
	}
	return nil, m.validate(oldMD)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *MachineDeployment) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
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

	// MachineSet preflight checks that should be skipped could also be set as annotation on the MachineDeployment
	// since MachineDeployment annotations are synced to the MachineSet.
	if feature.Gates.Enabled(feature.MachineSetPreflightChecks) {
		if err := validateSkippedMachineSetPreflightChecks(m); err != nil {
			allErrs = append(allErrs, err)
		}
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

// calculateMachineDeploymentReplicas calculates the default value of the replicas field.
// The value will be calculated based on the following logic:
// * if replicas is already set on newMD, keep the current value
// * if the autoscaler min size and max size annotations are set:
//   - if it's a new MachineDeployment, use min size
//   - if the replicas field of the old MachineDeployment is < min size, use min size
//   - if the replicas field of the old MachineDeployment is > max size, use max size
//   - if the replicas field of the old MachineDeployment is in the (min size, max size) range, keep the value from the oldMD
//
// * otherwise use 1
//
// The goal of this logic is to provide a smoother UX for clusters using the Kubernetes autoscaler.
// Note: Autoscaler only takes over control of the replicas field if the replicas value is in the (min size, max size) range.
//
// We are supporting the following use cases:
// * A new MD is created and replicas should be managed by the autoscaler
//   - Either via the default annotation or via the min size and max size annotations the replicas field
//     is defaulted to a value which is within the (min size, max size) range so the autoscaler can take control.
//
// * An existing MD which initially wasn't controlled by the autoscaler should be later controlled by the autoscaler
//   - To adopt an existing MD users can use the default, min size and max size annotations to enable the autoscaler
//     and to ensure the replicas field is within the (min size, max size) range. Without the annotations handing over
//     control to the autoscaler by unsetting the replicas field would lead to the field being set to 1. This is very
//     disruptive for existing Machines and if 1 is outside the (min size, max size) range the autoscaler won't take
//     control.
//
// Notes:
//   - While the min size and max size annotations of the autoscaler provide the best UX, other autoscalers can use the
//     DefaultReplicasAnnotation if they have similar use cases.
func calculateMachineDeploymentReplicas(ctx context.Context, oldMD *MachineDeployment, newMD *MachineDeployment, dryRun bool) (int32, error) {
	// If replicas is already set => Keep the current value.
	if newMD.Spec.Replicas != nil {
		return *newMD.Spec.Replicas, nil
	}

	log := ctrl.LoggerFrom(ctx)

	// If both autoscaler annotations are set, use them to calculate the default value.
	minSizeString, hasMinSizeAnnotation := newMD.Annotations[AutoscalerMinSizeAnnotation]
	maxSizeString, hasMaxSizeAnnotation := newMD.Annotations[AutoscalerMaxSizeAnnotation]
	if hasMinSizeAnnotation && hasMaxSizeAnnotation {
		minSize, err := strconv.ParseInt(minSizeString, 10, 32)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to caculate MachineDeployment replicas value: could not parse the value of the %q annotation", AutoscalerMinSizeAnnotation)
		}
		maxSize, err := strconv.ParseInt(maxSizeString, 10, 32)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to caculate MachineDeployment replicas value: could not parse the value of the %q annotation", AutoscalerMaxSizeAnnotation)
		}

		// If it's a new MachineDeployment => Use the min size.
		// Note: This will result in a scale up to get into the range where autoscaler takes over.
		if oldMD == nil {
			if !dryRun {
				log.V(2).Info(fmt.Sprintf("Replica field has been defaulted to %d based on the %s annotation (MD is a new MD)", minSize, AutoscalerMinSizeAnnotation))
			}
			return int32(minSize), nil
		}

		// Otherwise we are handing over the control for the replicas field for an existing MachineDeployment
		// to the autoscaler.

		switch {
		// If the old MachineDeployment doesn't have replicas set => Use the min size.
		// Note: As defaulting always sets the replica field, this case should not be possible
		// We only have this handling to be 100% safe against panics.
		case oldMD.Spec.Replicas == nil:
			if !dryRun {
				log.V(2).Info(fmt.Sprintf("Replica field has been defaulted to %d based on the %s annotation (old MD didn't have replicas set)", minSize, AutoscalerMinSizeAnnotation))
			}
			return int32(minSize), nil
		// If the old MachineDeployment replicas are lower than min size => Use the min size.
		// Note: This will result in a scale up to get into the range where autoscaler takes over.
		case *oldMD.Spec.Replicas < int32(minSize):
			if !dryRun {
				log.V(2).Info(fmt.Sprintf("Replica field has been defaulted to %d based on the %s annotation (old MD had replicas below min size)", minSize, AutoscalerMinSizeAnnotation))
			}
			return int32(minSize), nil
		// If the old MachineDeployment replicas are higher than max size => Use the max size.
		// Note: This will result in a scale down to get into the range where autoscaler takes over.
		case *oldMD.Spec.Replicas > int32(maxSize):
			if !dryRun {
				log.V(2).Info(fmt.Sprintf("Replica field has been defaulted to %d based on the %s annotation (old MD had replicas above max size)", maxSize, AutoscalerMaxSizeAnnotation))
			}
			return int32(maxSize), nil
		// If the old MachineDeployment replicas are between min and max size => Keep the current value.
		default:
			if !dryRun {
				log.V(2).Info(fmt.Sprintf("Replica field has been defaulted to %d based on replicas of the old MachineDeployment (old MD had replicas within min size / max size range)", *oldMD.Spec.Replicas))
			}
			return *oldMD.Spec.Replicas, nil
		}
	}

	// If neither the default nor the autoscaler annotations are set => Default to 1.
	if !dryRun {
		log.V(2).Info("Replica field has been defaulted to 1")
	}
	return 1, nil
}
