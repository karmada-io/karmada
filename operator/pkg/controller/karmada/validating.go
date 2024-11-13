/*
Copyright 2024 The Karmada Authors.

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

package karmada

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func validateCRDTarball(crdTarball *operatorv1alpha1.CRDTarball, fldPath *field.Path) (errs field.ErrorList) {
	if crdTarball == nil || crdTarball.HTTPSource == nil {
		return nil
	}

	if _, err := url.ParseRequestURI(crdTarball.HTTPSource.URL); err != nil {
		errs = append(errs, field.Invalid(fldPath.Child("httpSource").Child("url"), crdTarball.HTTPSource.URL, "invalid CRDs remote URL"))
	}

	return errs
}

func validateKarmadaAPIServer(karmadaAPIServer *operatorv1alpha1.KarmadaAPIServer, hostCluster *operatorv1alpha1.HostCluster, fldPath *field.Path) (errs field.ErrorList) {
	if karmadaAPIServer == nil {
		return nil
	}

	serviceType := karmadaAPIServer.ServiceType
	if serviceType != corev1.ServiceTypeClusterIP && serviceType != corev1.ServiceTypeNodePort && serviceType != corev1.ServiceTypeLoadBalancer {
		errs = append(errs, field.Invalid(fldPath.Child("serviceType"), serviceType, "unsupported service type for Karmada API server"))
	}
	if !util.IsInCluster(hostCluster) && serviceType == corev1.ServiceTypeClusterIP {
		errs = append(errs, field.Invalid(fldPath.Child("serviceType"), serviceType, "if karmada is installed in a remote cluster, the service type of karmada-apiserver must be either NodePort or LoadBalancer"))
	}

	return errs
}

func validateETCD(etcd *operatorv1alpha1.Etcd, karmadaName string, fldPath *field.Path) (errs field.ErrorList) {
	if etcd == nil || (etcd.Local == nil && etcd.External == nil) {
		errs = append(errs, field.Invalid(fldPath, etcd, "unexpected empty etcd configuration"))
		return errs
	}

	if etcd.External != nil {
		expectedSecretName := util.EtcdCertSecretName(karmadaName)
		if etcd.External.SecretRef.Name != expectedSecretName {
			errs = append(errs, field.Invalid(fldPath.Child("external").Child("secretRef").Child("name"), etcd.External.SecretRef.Name, "secret name for external etcd client must be "+expectedSecretName))
		}
	}

	if etcd.Local != nil && etcd.Local.CommonSettings.Replicas != nil {
		replicas := *etcd.Local.CommonSettings.Replicas

		if (replicas % 2) == 0 {
			klog.Warningf("invalid etcd replicas %d, expected an odd number", replicas)
		}
	}

	return errs
}

func validate(karmada *operatorv1alpha1.Karmada) error {
	var errs field.ErrorList

	errs = append(errs, validateCRDTarball(karmada.Spec.CRDTarball, field.NewPath("spec").Child("crdTarball"))...)

	if karmada.Spec.Components != nil {
		components, fldPath := karmada.Spec.Components, field.NewPath("spec").Child("components")

		errs = append(errs, validateKarmadaAPIServer(components.KarmadaAPIServer, karmada.Spec.HostCluster, fldPath.Child("karmadaAPIServer"))...)
		errs = append(errs, validateETCD(components.Etcd, karmada.Name, fldPath.Child("etcd"))...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %v", errs)
	}
	return nil
}

func (ctrl *Controller) validateKarmada(ctx context.Context, karmada *operatorv1alpha1.Karmada) error {
	if err := validate(karmada); err != nil {
		ctrl.EventRecorder.Event(karmada, corev1.EventTypeWarning, ValidationErrorReason, err.Error())

		newCondition := metav1.Condition{
			Type:               string(operatorv1alpha1.Ready),
			Status:             metav1.ConditionFalse,
			Reason:             ValidationErrorReason,
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		}
		meta.SetStatusCondition(&karmada.Status.Conditions, newCondition)
		if updateErr := ctrl.Status().Update(ctx, karmada); updateErr != nil {
			return fmt.Errorf("failed to update validate condition, validate error: %+v, update err: %+v", err, updateErr)
		}
		return err
	}
	return nil
}
