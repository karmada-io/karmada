package mcs

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ServiceImportControllerName is the controller name that will be used when reporting events.
const ServiceImportControllerName = "service-import-controller"

// ServiceImportController is to sync derived service from ServiceImport.
type ServiceImportController struct {
	client.Client
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
	// ResourceInterpreter knows the details of resource structure.
	ResourceInterpreter resourceinterpreter.ResourceInterpreter
}

func (c *ServiceImportController) getSvcClusterNameFromControlSvc(svc *corev1.Service) ([]string, error) {
	binding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name + "-" + strings.ToLower(svc.Kind),
			Namespace: svc.Namespace,
		},
	}
	if bindErr := c.Get(context.TODO(), client.ObjectKey{Name: binding.Name, Namespace: binding.Namespace}, binding); bindErr != nil {
		return nil, bindErr
	}

	var cNames []string
	for _, cluster := range binding.Spec.Clusters {
		name := cluster.Name
		cNames = append(cNames, name)
	}

	if len(cNames) == 0 {
		return nil, fmt.Errorf("binding.Spec.Clusters is nil")
	}
	return cNames, nil
}

func (c *ServiceImportController) setSvcImportIpsFromSvc(svcImport *mcsv1alpha1.ServiceImport) error {
	derivedSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateDerivedServiceName(svcImport.Name),
			Namespace: svcImport.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
	}

	clusterNames, err := c.getServiceImportPropagateClusterNames(derivedSvc)
	if err != nil {
		klog.Errorf("Failed to get derived svc %s/%s propagate clusters, err: %v", derivedSvc.Namespace, derivedSvc.Name, err)
		return err
	}

	for _, cName := range clusterNames {
		if setErr := c.setClusterIPToMemberServiceImport(derivedSvc, svcImport, cName); setErr != nil {
			klog.Errorf("Failed to set %s serviceImport %s/%s clusterIP, %v", cName, derivedSvc.Namespace, derivedSvc.Name, setErr)
			return setErr
		}
	}

	return nil
}

func (c *ServiceImportController) getServiceImportPropagateClusterNames(derivedSvc *corev1.Service) ([]string, error) {
	clusterNames, clusterErr := c.getSvcClusterNameFromControlSvc(derivedSvc)
	if clusterErr != nil {
		return nil, clusterErr
	}
	return clusterNames, nil
}

func (c *ServiceImportController) getClusterIPFromClusterDerivedSvs(derivedSvc *corev1.Service, clusterDynamicClient *util.DynamicClusterClient) (string, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, schema.FromAPIVersionAndKind(derivedSvc.APIVersion, derivedSvc.Kind))
	if err != nil {
		return "", err
	}

	var svcLoad *unstructured.Unstructured
	var getErr error
	if waitErr := wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		svcLoad, getErr = clusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(derivedSvc.Namespace).Get(context.TODO(), derivedSvc.Name, metav1.GetOptions{})
		if getErr != nil {
			klog.Warningf("Failed to get derived service %s/%s from cluster, err: %v", derivedSvc.Namespace, derivedSvc.Name, err)
			return false, nil
		}
		return true, nil
	}); waitErr != nil {
		klog.Errorf("Failed to get derived service %s/%s from cluster, err: %v", derivedSvc.Namespace, derivedSvc.Name, err)
		return "", waitErr
	}

	clusterIP, _, err := unstructured.NestedString(svcLoad.Object, "spec", "clusterIP")
	if err != nil {
		return "", err
	}
	return clusterIP, nil
}

func (c *ServiceImportController) updateClusterServiceImport(svcImport *mcsv1alpha1.ServiceImport, clusterIP string, clusterDynamicClient *util.DynamicClusterClient) error {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, schema.FromAPIVersionAndKind(svcImport.APIVersion, svcImport.Kind))
	if err != nil {
		return err
	}

	var clusterSvcImportObj *unstructured.Unstructured
	var setErr error
	if err := wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		if clusterSvcImportObj, setErr = clusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(svcImport.Namespace).Get(context.TODO(),
			svcImport.Name, metav1.GetOptions{}); setErr != nil {
			klog.Warningf("Failed to get service import %s/%s from cluster, err: %v", svcImport.Namespace, svcImport.Name, err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		klog.Errorf("Failed to get service import %s/%s from cluster, err: %v", svcImport.Namespace, svcImport.Name, err)
		return err
	}

	unstructuredObj, addErr := c.addUnstructSvcImportClusterIP(clusterSvcImportObj, clusterIP)
	if addErr != nil {
		return addErr
	}

	if _, err = clusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(unstructuredObj.GetNamespace()).Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func (c *ServiceImportController) addUnstructSvcImportClusterIP(clusterSvcImportObj *unstructured.Unstructured, clusterIP string) (*unstructured.Unstructured, error) {
	nameSpace := clusterSvcImportObj.GetNamespace()
	name := clusterSvcImportObj.GetName()

	ips, _, err := unstructured.NestedStringSlice(clusterSvcImportObj.Object, "spec", "ips")
	if err != nil {
		return nil, fmt.Errorf("error get ips from serviceImport: %w", err)
	}
	if len(ips) != 0 {
		klog.Warningf("Service import %s/%s already has ips: %v", nameSpace, name, ips)
		return clusterSvcImportObj, nil
	}

	var data []interface{}
	data = append(data, clusterIP)
	// !ok could indicate that a cluster ip was not assigned
	err = unstructured.SetNestedSlice(clusterSvcImportObj.Object, data, "spec", "ips")
	if err != nil {
		return nil, fmt.Errorf("error setting ips for serviceImport: %w", err)
	}
	return clusterSvcImportObj, nil
}

func (c *ServiceImportController) setClusterIPToMemberServiceImport(derivedSvc *corev1.Service, svcImport *mcsv1alpha1.ServiceImport, clusterName string) error {
	clusterDynamicClient, err := util.NewClusterDynamicClientSet(clusterName, c.Client)
	if err != nil {
		return err
	}

	clusterIP, getClusterIPErr := c.getClusterIPFromClusterDerivedSvs(derivedSvc, clusterDynamicClient)
	if getClusterIPErr != nil {
		return getClusterIPErr
	}

	if updateClusterIPErr := c.updateClusterServiceImport(svcImport, clusterIP, clusterDynamicClient); updateClusterIPErr != nil {
		return updateClusterIPErr
	}
	klog.Infof("Set clusterIP(%s) to %s serviceImport %s/%s successfully.", clusterIP, clusterName, svcImport.Namespace, svcImport.Name)
	return nil
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *ServiceImportController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ServiceImport %s.", req.NamespacedName.String())

	svcImport := &mcsv1alpha1.ServiceImport{}
	if err := c.Client.Get(ctx, req.NamespacedName, svcImport); err != nil {
		if apierrors.IsNotFound(err) {
			return c.deleteDerivedService(req.NamespacedName)
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !svcImport.DeletionTimestamp.IsZero() || svcImport.Spec.Type != mcsv1alpha1.ClusterSetIP {
		return controllerruntime.Result{}, nil
	}

	if err := c.deriveServiceFromServiceImport(svcImport); err != nil {
		c.EventRecorder.Eventf(svcImport, corev1.EventTypeWarning, events.EventReasonSyncDerivedServiceFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	if setErr := c.setSvcImportIpsFromSvc(svcImport); setErr != nil {
		c.EventRecorder.Eventf(svcImport, corev1.EventTypeWarning, events.EventReasonSetServiceImportIPsFailed, setErr.Error())
		return controllerruntime.Result{Requeue: true}, setErr
	}

	c.EventRecorder.Eventf(svcImport, corev1.EventTypeNormal, events.EventReasonSyncDerivedServiceSucceed, "Sync derived service for serviceImport(%s) succeed.", svcImport.Name)
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ServiceImportController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&mcsv1alpha1.ServiceImport{}).Complete(c)
}

func (c *ServiceImportController) deleteDerivedService(svcImport types.NamespacedName) (controllerruntime.Result, error) {
	derivedSvc := &corev1.Service{}
	derivedSvcNamespacedName := types.NamespacedName{
		Namespace: svcImport.Namespace,
		Name:      names.GenerateDerivedServiceName(svcImport.Name),
	}
	err := c.Client.Get(context.TODO(), derivedSvcNamespacedName, derivedSvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	err = c.Client.Delete(context.TODO(), derivedSvc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Delete derived service(%s) failed, Error: %v", derivedSvcNamespacedName, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *ServiceImportController) deriveServiceFromServiceImport(svcImport *mcsv1alpha1.ServiceImport) error {
	newDerivedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svcImport.Namespace,
			Name:      names.GenerateDerivedServiceName(svcImport.Name),
			Labels: map[string]string{
				util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: servicePorts(svcImport),
		},
	}

	oldDerivedService := &corev1.Service{}
	err := c.Client.Get(context.TODO(), types.NamespacedName{
		Name:      names.GenerateDerivedServiceName(svcImport.Name),
		Namespace: svcImport.Namespace,
	}, oldDerivedService)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err = c.Client.Create(context.TODO(), newDerivedService); err != nil {
				klog.Errorf("Create derived service(%s/%s) failed, Error: %v", newDerivedService.Namespace, newDerivedService.Name, err)
				return err
			}

			return c.updateServiceStatus(svcImport, newDerivedService)
		}

		return err
	}

	// retain necessary fields with old service
	retainServiceFields(oldDerivedService, newDerivedService)

	err = c.Client.Update(context.TODO(), newDerivedService)
	if err != nil {
		klog.Errorf("Update derived service(%s/%s) failed, Error: %v", newDerivedService.Namespace, newDerivedService.Name, err)
		return err
	}

	return c.updateServiceStatus(svcImport, newDerivedService)
}

// updateServiceStatus update loadbalanacer status with provided clustersetIPs
func (c *ServiceImportController) updateServiceStatus(svcImport *mcsv1alpha1.ServiceImport, derivedService *corev1.Service) error {
	ingress := make([]corev1.LoadBalancerIngress, 0)
	for _, ip := range svcImport.Spec.IPs {
		ingress = append(ingress, corev1.LoadBalancerIngress{
			IP: ip,
		})
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		derivedService.Status = corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: ingress,
			},
		}
		updateErr := c.Status().Update(context.TODO(), derivedService)
		if updateErr == nil {
			return nil
		}

		updated := &corev1.Service{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: derivedService.Namespace, Name: derivedService.Name}, updated); err == nil {
			derivedService = updated
		} else {
			klog.Errorf("Failed to get updated service %s/%s: %v", derivedService.Namespace, derivedService.Name, err)
		}

		return updateErr
	})

	if err != nil {
		klog.Errorf("Update derived service(%s/%s) status failed, Error: %v", derivedService.Namespace, derivedService.Name, err)
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
