package detector

import (
	"context"
	"encoding/json"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// AggregateDeploymentStatus summarize deployment status and update to original objects.
func (d *ResourceDetector) AggregateDeploymentStatus(objRef workv1alpha2.ObjectReference, status []workv1alpha2.AggregatedStatusItem) error {
	if objRef.APIVersion != "apps/v1" {
		return nil
	}

	obj := &appsv1.Deployment{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get deployment(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	oldStatus := &obj.Status
	newStatus := &appsv1.DeploymentStatus{}
	for _, item := range status {
		if item.Status == nil {
			continue
		}
		temp := &appsv1.DeploymentStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status")
			return err
		}
		klog.V(3).Infof("Grab deployment(%s/%s) status from cluster(%s), replicas: %d, ready: %d, updated: %d, available: %d, unavailable: %d",
			obj.Namespace, obj.Name, item.ClusterName, temp.Replicas, temp.ReadyReplicas, temp.UpdatedReplicas, temp.AvailableReplicas, temp.UnavailableReplicas)
		newStatus.ObservedGeneration = obj.Generation
		newStatus.Replicas += temp.Replicas
		newStatus.ReadyReplicas += temp.ReadyReplicas
		newStatus.UpdatedReplicas += temp.UpdatedReplicas
		newStatus.AvailableReplicas += temp.AvailableReplicas
		newStatus.UnavailableReplicas += temp.UnavailableReplicas
	}

	if oldStatus.ObservedGeneration == newStatus.ObservedGeneration &&
		oldStatus.Replicas == newStatus.Replicas &&
		oldStatus.ReadyReplicas == newStatus.ReadyReplicas &&
		oldStatus.UpdatedReplicas == newStatus.UpdatedReplicas &&
		oldStatus.AvailableReplicas == newStatus.AvailableReplicas &&
		oldStatus.UnavailableReplicas == newStatus.UnavailableReplicas {
		klog.V(3).Infof("ignore update deployment(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

	oldStatus.ObservedGeneration = newStatus.ObservedGeneration
	oldStatus.Replicas = newStatus.Replicas
	oldStatus.ReadyReplicas = newStatus.ReadyReplicas
	oldStatus.UpdatedReplicas = newStatus.UpdatedReplicas
	oldStatus.AvailableReplicas = newStatus.AvailableReplicas
	oldStatus.UnavailableReplicas = newStatus.UnavailableReplicas

	if err := d.Client.Status().Update(context.TODO(), obj); err != nil {
		klog.Errorf("Failed to update deployment(%s/%s) status: %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	return nil
}

// AggregateServiceStatus summarize service status and update to original objects.
func (d *ResourceDetector) AggregateServiceStatus(objRef workv1alpha2.ObjectReference, status []workv1alpha2.AggregatedStatusItem) error {
	if objRef.APIVersion != "v1" {
		return nil
	}

	obj := &corev1.Service{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get service(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	if obj.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}

	// If service type is of type LoadBalancer, collect the status.loadBalancer.ingress
	newStatus := &corev1.ServiceStatus{}
	for _, item := range status {
		if item.Status == nil {
			continue
		}
		temp := &corev1.ServiceStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status of service(%s/%s): %v", objRef.Namespace, objRef.Name, err)
			return err
		}
		klog.V(3).Infof("Grab service(%s/%s) status from cluster(%s), loadBalancer status: %v",
			obj.Namespace, obj.Name, item.ClusterName, temp.LoadBalancer)

		// Set cluster name as Hostname by default to indicate the status is collected from which member cluster.
		for i := range temp.LoadBalancer.Ingress {
			if temp.LoadBalancer.Ingress[i].Hostname == "" {
				temp.LoadBalancer.Ingress[i].Hostname = item.ClusterName
			}
		}

		newStatus.LoadBalancer.Ingress = append(newStatus.LoadBalancer.Ingress, temp.LoadBalancer.Ingress...)
	}

	if reflect.DeepEqual(obj.Status, *newStatus) {
		klog.V(3).Infof("ignore update service(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

	obj.Status = *newStatus
	if err := d.Client.Status().Update(context.TODO(), obj); err != nil {
		klog.Errorf("Failed to update service(%s/%s) status: %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	return nil
}

// AggregateIngressStatus summarize ingress status and update to original objects.
func (d *ResourceDetector) AggregateIngressStatus(objRef workv1alpha2.ObjectReference, status []workv1alpha2.AggregatedStatusItem) error {
	if objRef.APIVersion != "extensions/v1beta1" {
		return nil
	}

	obj := &extensionsv1beta1.Ingress{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get ingress(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	newStatus := &extensionsv1beta1.IngressStatus{}
	for _, item := range status {
		if item.Status == nil {
			continue
		}
		temp := &extensionsv1beta1.IngressStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status ingress(%s/%s): %v", obj.Namespace, obj.Name, err)
			return err
		}
		klog.V(3).Infof("Grab ingress(%s/%s) status from cluster(%s), loadBalancer status: %v",
			obj.Namespace, obj.Name, item.ClusterName, temp.LoadBalancer)

		// Set cluster name as Hostname by default to indicate the status is collected from which member cluster.
		for i := range temp.LoadBalancer.Ingress {
			if temp.LoadBalancer.Ingress[i].Hostname == "" {
				temp.LoadBalancer.Ingress[i].Hostname = item.ClusterName
			}
		}

		newStatus.LoadBalancer.Ingress = append(newStatus.LoadBalancer.Ingress, temp.LoadBalancer.Ingress...)
	}

	if reflect.DeepEqual(obj.Status, *newStatus) {
		klog.V(3).Infof("ignore update ingress(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

	obj.Status = *newStatus
	if err := d.Client.Status().Update(context.TODO(), obj); err != nil {
		klog.Errorf("Failed to update ingress(%s/%s) status: %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	return nil
}

// AggregateJobStatus summarize job status and update to original objects.
func (d *ResourceDetector) AggregateJobStatus(objRef workv1alpha2.ObjectReference, status []workv1alpha2.AggregatedStatusItem, clusters []workv1alpha2.TargetCluster) error {
	if objRef.APIVersion != "batch/v1" {
		return nil
	}

	obj := &batchv1.Job{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get service(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	newStatus, err := helper.ParsingJobStatus(obj, status, clusters)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(obj.Status, *newStatus) {
		klog.V(3).Infof("ignore update job(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		if err = d.Client.Get(context.TODO(), client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, obj); err != nil {
			return err
		}
		obj.Status = *newStatus
		return d.Client.Status().Update(context.TODO(), obj)
	})
}
