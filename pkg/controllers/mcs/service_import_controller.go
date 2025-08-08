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

package mcs

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ServiceImportControllerName is the controller name that will be used when reporting events and metrics.
const ServiceImportControllerName = "service-import-controller"

// ServiceImportController is to sync derived service from ServiceImport.
type ServiceImportController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *ServiceImportController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling ServiceImport", "name", req.NamespacedName.String())

	svcImport := &mcsv1alpha1.ServiceImport{}
	if err := c.Client.Get(ctx, req.NamespacedName, svcImport); err != nil {
		if apierrors.IsNotFound(err) {
			return c.deleteDerivedService(ctx, req.NamespacedName)
		}

		return controllerruntime.Result{}, err
	}

	if !svcImport.DeletionTimestamp.IsZero() || svcImport.Spec.Type != mcsv1alpha1.ClusterSetIP {
		return controllerruntime.Result{}, nil
	}

	if err := c.deriveServiceFromServiceImport(ctx, svcImport); err != nil {
		c.EventRecorder.Eventf(svcImport, corev1.EventTypeWarning, events.EventReasonSyncDerivedServiceFailed, err.Error())
		return controllerruntime.Result{}, err
	}
	c.EventRecorder.Eventf(svcImport, corev1.EventTypeNormal, events.EventReasonSyncDerivedServiceSucceed, "Sync derived service for serviceImport(%s) succeed.", svcImport.Name)
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ServiceImportController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ServiceImportControllerName).
		For(&mcsv1alpha1.ServiceImport{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *ServiceImportController) deleteDerivedService(ctx context.Context, svcImport types.NamespacedName) (controllerruntime.Result, error) {
	derivedSvc := &corev1.Service{}
	derivedSvcNamespacedName := types.NamespacedName{
		Namespace: svcImport.Namespace,
		Name:      names.GenerateDerivedServiceName(svcImport.Name),
	}
	err := c.Client.Get(ctx, derivedSvcNamespacedName, derivedSvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	err = c.Client.Delete(ctx, derivedSvc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "Delete derived service failed", "name", derivedSvcNamespacedName)
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *ServiceImportController) deriveServiceFromServiceImport(ctx context.Context, svcImport *mcsv1alpha1.ServiceImport) error {
	newDerivedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcImport.Namespace,
			Name:      names.GenerateDerivedServiceName(svcImport.Name),
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: servicePorts(svcImport),
		},
	}

	oldDerivedService := &corev1.Service{}
	err := c.Client.Get(ctx, types.NamespacedName{
		Name:      names.GenerateDerivedServiceName(svcImport.Name),
		Namespace: svcImport.Namespace,
	}, oldDerivedService)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err = c.Client.Create(ctx, newDerivedService); err != nil {
				klog.ErrorS(err, "Create derived service failed", "namespace", newDerivedService.Namespace, "name", newDerivedService.Name)
				return err
			}

			return c.updateServiceStatus(ctx, svcImport, newDerivedService)
		}

		return err
	}

	// retain necessary fields with old service
	retainServiceFields(oldDerivedService, newDerivedService)

	err = c.Client.Update(ctx, newDerivedService)
	if err != nil {
		klog.ErrorS(err, "Update derived service failed", "namespace", newDerivedService.Namespace, "name", newDerivedService.Name)
		return err
	}

	return c.updateServiceStatus(ctx, svcImport, newDerivedService)
}

// updateServiceStatus update loadbalanacer status with provided clustersetIPs
func (c *ServiceImportController) updateServiceStatus(ctx context.Context, svcImport *mcsv1alpha1.ServiceImport, derivedService *corev1.Service) error {
	ingress := make([]corev1.LoadBalancerIngress, 0)
	for _, ip := range svcImport.Spec.IPs {
		ingress = append(ingress, corev1.LoadBalancerIngress{
			IP: ip,
		})
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, derivedService, func() error {
			derivedService.Status = corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: ingress,
				},
			}
			return nil
		})
		return err
	})

	if err != nil {
		klog.ErrorS(err, "Update derived service status failed", "namespace", derivedService.Namespace, "name", derivedService.Name)
		return err
	}
	return nil
}

func servicePorts(svcImport *mcsv1alpha1.ServiceImport) []corev1.ServicePort {
	ports := make([]corev1.ServicePort, len(svcImport.Spec.Ports))
	for i, p := range svcImport.Spec.Ports {
		ports[i] = corev1.ServicePort{
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	}
	return ports
}

func retainServiceFields(oldSvc, newSvc *corev1.Service) {
	newSvc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
	newSvc.ResourceVersion = oldSvc.ResourceVersion
}
