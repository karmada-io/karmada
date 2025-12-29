/*
Copyright 2025 The Karmada Authors.

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

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/controller/karmada"
)

// SetupKarmadaWebhookWithManager registers the webhook for Karmada in the manager.
func SetupKarmadaWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&operatorv1alpha1.Karmada{}).
		WithValidator(&KarmadaCustomValidator{}).
		WithDefaulter(&KarmadaCustomDefaulter{}).
		Complete()
}

// KarmadaCustomDefaulter sets default values on Karmada objects.
// +kubebuilder:object:generate=false
type KarmadaCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &KarmadaCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a mutating webhook will be registered for Karmada.
func (d *KarmadaCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	k, ok := obj.(*operatorv1alpha1.Karmada)
	if !ok {
		return fmt.Errorf("expected a Karmada object but got %T", obj)
	}

	// Ensure labels map exists.
	if k.Labels == nil {
		k.Labels = map[string]string{}
	}
	// Default the cascading-deletion label to "false" if not present.
	if _, exists := k.Labels[karmada.DisableCascadingDeletionLabel]; !exists {
		merged := labels.Merge(k.GetLabels(), labels.Set{karmada.DisableCascadingDeletionLabel: "false"})
		k.SetLabels(merged)
		klog.V(2).InfoS("Defaulted label", "namespace", k.Namespace, "name", k.Name, "key", karmada.DisableCascadingDeletionLabel, "value", "false")
	}

	// Apply defaults
	operatorv1alpha1.SetObjectDefaultsKarmada(k)

	klog.V(2).InfoS("Defaulted Karmada", "namespace", k.Namespace, "name", k.Name)
	return nil
}

// KarmadaCustomValidator validates Karmada resources on create/update/delete.
// +kubebuilder:object:generate=false
type KarmadaCustomValidator struct{}

var _ webhook.CustomValidator = &KarmadaCustomValidator{}

// ValidateCreate validates creation of a Karmada object
func (v *KarmadaCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	k, ok := obj.(*operatorv1alpha1.Karmada)
	if !ok {
		return nil, fmt.Errorf("expected a Karmada but got %T", obj)
	}

	// Enforce validation rules
	if err := validateKarmada(k); err != nil {
		klog.ErrorS(err, "Validation failed for Karmada (create)", "namespace", k.Namespace, "name", k.Name)
		return nil, err
	}

	klog.V(2).InfoS("Validated Karmada (create)", "namespace", k.Namespace, "name", k.Name)
	return nil, nil
}

// ValidateUpdate validates updates of a Karmada object
func (v *KarmadaCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldK, okOld := oldObj.(*operatorv1alpha1.Karmada)
	newK, okNew := newObj.(*operatorv1alpha1.Karmada)
	if !okOld || !okNew {
		return nil, fmt.Errorf("expected a Karmada but got %T -> %T", oldObj, newObj)
	}

	// Enforce validation rules
	if err := validateKarmada(newK); err != nil {
		klog.ErrorS(err, "Validation failed for Karmada (update)",
			"namespace", newK.Namespace, "name", newK.Name,
			"oldResourceVersion", oldK.ResourceVersion, "newResourceVersion", newK.ResourceVersion)
		return nil, err
	}

	klog.V(2).InfoS("Validated Karmada (update)",
		"namespace", newK.Namespace, "name", newK.Name,
		"oldResourceVersion", oldK.ResourceVersion, "newResourceVersion", newK.ResourceVersion)
	return nil, nil
}

// ValidateDelete validates deletion of a Karmada object
func (v *KarmadaCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	k, ok := obj.(*operatorv1alpha1.Karmada)
	if !ok {
		return nil, fmt.Errorf("expected a Karmada but got %T", obj)
	}
	klog.V(2).InfoS("Validated Karmada (delete)", "namespace", k.Namespace, "name", k.Name)
	return nil, nil
}

func validateKarmada(k *operatorv1alpha1.Karmada) error {
	err := karmada.Validate(k)
	if err != nil {
		return err
	}
	return nil
}
