package defaultinterpreter

import (
	"encoding/json"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type aggregateStatusInterpreter func(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error)

func getAllDefaultAggregateStatusInterpreter() map[schema.GroupVersionKind]aggregateStatusInterpreter {
	s := make(map[schema.GroupVersionKind]aggregateStatusInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = aggregateDeploymentStatus
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = aggregateServiceStatus
	s[extensionsv1beta1.SchemeGroupVersion.WithKind(util.IngressKind)] = aggregateIngressStatus
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = aggregateJobStatus
	return s
}

func aggregateDeploymentStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	deploy, err := helper.ConvertToDeployment(object)
	if err != nil {
		return nil, err
	}

	oldStatus := &deploy.Status
	newStatus := &appsv1.DeploymentStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &appsv1.DeploymentStatus{}
		if err = json.Unmarshal(item.Status.Raw, temp); err != nil {
			return nil, err
		}
		klog.V(3).Infof("Grab deployment(%s/%s) status from cluster(%s), replicas: %d, ready: %d, updated: %d, available: %d, unavailable: %d",
			deploy.Namespace, deploy.Name, item.ClusterName, temp.Replicas, temp.ReadyReplicas, temp.UpdatedReplicas, temp.AvailableReplicas, temp.UnavailableReplicas)
		newStatus.ObservedGeneration = deploy.Generation
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
		klog.V(3).Infof("ignore update deployment(%s/%s) status as up to date", deploy.Namespace, deploy.Name)
		return object, nil
	}

	oldStatus.ObservedGeneration = newStatus.ObservedGeneration
	oldStatus.Replicas = newStatus.Replicas
	oldStatus.ReadyReplicas = newStatus.ReadyReplicas
	oldStatus.UpdatedReplicas = newStatus.UpdatedReplicas
	oldStatus.AvailableReplicas = newStatus.AvailableReplicas
	oldStatus.UnavailableReplicas = newStatus.UnavailableReplicas

	return helper.ToUnstructured(deploy)
}

func aggregateServiceStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	service, err := helper.ConvertToService(object)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return object, nil
	}

	// If service type is of type LoadBalancer, collect the status.loadBalancer.ingress
	newStatus := &corev1.ServiceStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &corev1.ServiceStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status of service(%s/%s): %v", service.Namespace, service.Name, err)
			return nil, err
		}
		klog.V(3).Infof("Grab service(%s/%s) status from cluster(%s), loadBalancer status: %v",
			service.Namespace, service.Name, item.ClusterName, temp.LoadBalancer)

		// Set cluster name as Hostname by default to indicate the status is collected from which member cluster.
		for i := range temp.LoadBalancer.Ingress {
			if temp.LoadBalancer.Ingress[i].Hostname == "" {
				temp.LoadBalancer.Ingress[i].Hostname = item.ClusterName
			}
		}

		newStatus.LoadBalancer.Ingress = append(newStatus.LoadBalancer.Ingress, temp.LoadBalancer.Ingress...)
	}

	if reflect.DeepEqual(service.Status, *newStatus) {
		klog.V(3).Infof("ignore update service(%s/%s) status as up to date", service.Namespace, service.Name)
		return object, nil
	}

	service.Status = *newStatus
	return helper.ToUnstructured(service)
}

func aggregateIngressStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	ingress, err := helper.ConvertToIngress(object)
	if err != nil {
		return nil, err
	}

	newStatus := &extensionsv1beta1.IngressStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &extensionsv1beta1.IngressStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status ingress(%s/%s): %v", ingress.Namespace, ingress.Name, err)
			return nil, err
		}
		klog.V(3).Infof("Grab ingress(%s/%s) status from cluster(%s), loadBalancer status: %v",
			ingress.Namespace, ingress.Name, item.ClusterName, temp.LoadBalancer)

		// Set cluster name as Hostname by default to indicate the status is collected from which member cluster.
		for i := range temp.LoadBalancer.Ingress {
			if temp.LoadBalancer.Ingress[i].Hostname == "" {
				temp.LoadBalancer.Ingress[i].Hostname = item.ClusterName
			}
		}

		newStatus.LoadBalancer.Ingress = append(newStatus.LoadBalancer.Ingress, temp.LoadBalancer.Ingress...)
	}

	if reflect.DeepEqual(ingress.Status, *newStatus) {
		klog.V(3).Infof("ignore update ingress(%s/%s) status as up to date", ingress.Namespace, ingress.Name)
		return object, nil
	}

	ingress.Status = *newStatus
	return helper.ToUnstructured(ingress)
}

func aggregateJobStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	job, err := helper.ConvertToJob(object)
	if err != nil {
		return nil, err
	}

	newStatus, err := helper.ParsingJobStatus(job, aggregatedStatusItems)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(job.Status, *newStatus) {
		klog.V(3).Infof("ignore update job(%s/%s) status as up to date", job.Namespace, job.Name)
		return object, nil
	}

	job.Status = *newStatus
	return helper.ToUnstructured(job)
}
