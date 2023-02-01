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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/util/version"
)

const defaultNodeDeletionTimeout = 10 * time.Second

func (m *Machine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1beta1-machine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machines,versions=v1beta1,name=validation.machine.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1beta1-machine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machines,versions=v1beta1,name=default.machine.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &Machine{}
var _ webhook.Defaulter = &Machine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (m *Machine) Default() {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterNameLabel] = m.Spec.ClusterName

	if m.Spec.Bootstrap.ConfigRef != nil && m.Spec.Bootstrap.ConfigRef.Namespace == "" {
		m.Spec.Bootstrap.ConfigRef.Namespace = m.Namespace
	}

	if m.Spec.InfrastructureRef.Namespace == "" {
		m.Spec.InfrastructureRef.Namespace = m.Namespace
	}

	if m.Spec.Version != nil && !strings.HasPrefix(*m.Spec.Version, "v") {
		normalizedVersion := "v" + *m.Spec.Version
		m.Spec.Version = &normalizedVersion
	}

	if m.Spec.NodeDeletionTimeout == nil {
		m.Spec.NodeDeletionTimeout = &metav1.Duration{Duration: defaultNodeDeletionTimeout}
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *Machine) ValidateCreate() error {
	return m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *Machine) ValidateUpdate(old runtime.Object) error {
	oldM, ok := old.(*Machine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Machine but got a %T", old))
	}
	return m.validate(oldM)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *Machine) ValidateDelete() error {
	return nil
}

func (m *Machine) validate(old *Machine) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")
	if m.Spec.Bootstrap.ConfigRef == nil && m.Spec.Bootstrap.DataSecretName == nil {
		allErrs = append(
			allErrs,
			field.Required(
				specPath.Child("bootstrap", "data"),
				"expected either spec.bootstrap.dataSecretName or spec.bootstrap.configRef to be populated",
			),
		)
	}

	if m.Spec.Bootstrap.ConfigRef != nil && m.Spec.Bootstrap.ConfigRef.Namespace != m.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				specPath.Child("bootstrap", "configRef", "namespace"),
				m.Spec.Bootstrap.ConfigRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if m.Spec.InfrastructureRef.Namespace != m.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				specPath.Child("infrastructureRef", "namespace"),
				m.Spec.InfrastructureRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if old != nil && old.Spec.ClusterName != m.Spec.ClusterName {
		allErrs = append(
			allErrs,
			field.Forbidden(specPath.Child("clusterName"), "field is immutable"),
		)
	}

	if m.Spec.Version != nil {
		if !version.KubeSemver.MatchString(*m.Spec.Version) {
			allErrs = append(allErrs, field.Invalid(specPath.Child("version"), *m.Spec.Version, "must be a valid semantic version"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Machine").GroupKind(), m.Name, allErrs)
}
