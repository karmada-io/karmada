package defaultinterpreter

import (
	"encoding/json"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	s[networkingv1.SchemeGroupVersion.WithKind(util.IngressKind)] = aggregateIngressStatus
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = aggregateJobStatus
	s[appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind)] = aggregateDaemonSetStatus
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = aggregateStatefulSetStatus
	s[corev1.SchemeGroupVersion.WithKind(util.PodKind)] = aggregatePodStatus
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeClaimKind)] = aggregatePersistentVolumeClaimStatus
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
		newStatus.Replicas += temp.Replicas
		newStatus.ReadyReplicas += temp.ReadyReplicas
		newStatus.UpdatedReplicas += temp.UpdatedReplicas
		newStatus.AvailableReplicas += temp.AvailableReplicas
		newStatus.UnavailableReplicas += temp.UnavailableReplicas
	}

	if oldStatus.Replicas == newStatus.Replicas &&
		oldStatus.ReadyReplicas == newStatus.ReadyReplicas &&
		oldStatus.UpdatedReplicas == newStatus.UpdatedReplicas &&
		oldStatus.AvailableReplicas == newStatus.AvailableReplicas &&
		oldStatus.UnavailableReplicas == newStatus.UnavailableReplicas {
		klog.V(3).Infof("ignore update deployment(%s/%s) status as up to date", deploy.Namespace, deploy.Name)
		return object, nil
	}

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

	newStatus := &networkingv1.IngressStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &networkingv1.IngressStatus{}
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

	// If a job is finished, we should never update status again.
	if finished, _ := helper.GetJobFinishedStatus(&job.Status); finished {
		return object, nil
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

func aggregateDaemonSetStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	daemonSet, err := helper.ConvertToDaemonSet(object)
	if err != nil {
		return nil, err
	}

	oldStatus := &daemonSet.Status
	newStatus := &appsv1.DaemonSetStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &appsv1.DaemonSetStatus{}
		if err = json.Unmarshal(item.Status.Raw, temp); err != nil {
			return nil, err
		}
		klog.V(3).Infof("Grab daemonSet(%s/%s) status from cluster(%s), currentNumberScheduled: %d, desiredNumberScheduled: %d, numberAvailable: %d, numberMisscheduled: %d, numberReady: %d, updatedNumberScheduled: %d, numberUnavailable: %d",
			daemonSet.Namespace, daemonSet.Name, item.ClusterName, temp.CurrentNumberScheduled, temp.DesiredNumberScheduled, temp.NumberAvailable, temp.NumberMisscheduled, temp.NumberReady, temp.UpdatedNumberScheduled, temp.NumberUnavailable)
		newStatus.CurrentNumberScheduled += temp.CurrentNumberScheduled
		newStatus.DesiredNumberScheduled += temp.DesiredNumberScheduled
		newStatus.NumberAvailable += temp.NumberAvailable
		newStatus.NumberMisscheduled += temp.NumberMisscheduled
		newStatus.NumberReady += temp.NumberReady
		newStatus.UpdatedNumberScheduled += temp.UpdatedNumberScheduled
		newStatus.NumberUnavailable += temp.NumberUnavailable
	}

	if oldStatus.CurrentNumberScheduled == newStatus.CurrentNumberScheduled &&
		oldStatus.DesiredNumberScheduled == newStatus.DesiredNumberScheduled &&
		oldStatus.NumberAvailable == newStatus.NumberAvailable &&
		oldStatus.NumberMisscheduled == newStatus.NumberMisscheduled &&
		oldStatus.NumberReady == newStatus.NumberReady &&
		oldStatus.UpdatedNumberScheduled == newStatus.UpdatedNumberScheduled &&
		oldStatus.NumberUnavailable == newStatus.NumberUnavailable {
		klog.V(3).Infof("ignore update daemonSet(%s/%s) status as up to date", daemonSet.Namespace, daemonSet.Name)
		return object, nil
	}

	oldStatus.CurrentNumberScheduled = newStatus.CurrentNumberScheduled
	oldStatus.DesiredNumberScheduled = newStatus.DesiredNumberScheduled
	oldStatus.NumberAvailable = newStatus.NumberAvailable
	oldStatus.NumberMisscheduled = newStatus.NumberMisscheduled
	oldStatus.NumberReady = newStatus.NumberReady
	oldStatus.UpdatedNumberScheduled = newStatus.UpdatedNumberScheduled
	oldStatus.NumberUnavailable = newStatus.NumberUnavailable

	return helper.ToUnstructured(daemonSet)
}

func aggregateStatefulSetStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	statefulSet, err := helper.ConvertToStatefulSet(object)
	if err != nil {
		return nil, err
	}
	oldStatus := &statefulSet.Status
	newStatus := &appsv1.StatefulSetStatus{}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}
		temp := &appsv1.StatefulSetStatus{}
		if err = json.Unmarshal(item.Status.Raw, temp); err != nil {
			return nil, err
		}
		klog.V(3).Infof("Grab statefulSet(%s/%s) status from cluster(%s), availableReplicas: %d, currentReplicas: %d, readyReplicas: %d, replicas: %d, updatedReplicas: %d",
			statefulSet.Namespace, statefulSet.Name, item.ClusterName, temp.AvailableReplicas, temp.CurrentReplicas, temp.ReadyReplicas, temp.Replicas, temp.UpdatedReplicas)
		newStatus.AvailableReplicas += temp.AvailableReplicas
		newStatus.CurrentReplicas += temp.CurrentReplicas
		newStatus.ReadyReplicas += temp.ReadyReplicas
		newStatus.Replicas += temp.Replicas
		newStatus.UpdatedReplicas += temp.UpdatedReplicas
	}

	if oldStatus.AvailableReplicas == newStatus.AvailableReplicas &&
		oldStatus.CurrentReplicas == newStatus.CurrentReplicas &&
		oldStatus.ReadyReplicas == newStatus.ReadyReplicas &&
		oldStatus.Replicas == newStatus.Replicas &&
		oldStatus.UpdatedReplicas == newStatus.UpdatedReplicas {
		klog.V(3).Infof("ignore update statefulSet(%s/%s) status as up to date", statefulSet.Namespace, statefulSet.Name)
		return object, nil
	}

	oldStatus.AvailableReplicas = newStatus.AvailableReplicas
	oldStatus.CurrentReplicas = newStatus.CurrentReplicas
	oldStatus.ReadyReplicas = newStatus.ReadyReplicas
	oldStatus.Replicas = newStatus.Replicas
	oldStatus.UpdatedReplicas = newStatus.UpdatedReplicas

	return helper.ToUnstructured(statefulSet)
}

func aggregatePodStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	pod, err := helper.ConvertToPod(object)
	if err != nil {
		return nil, err
	}

	newStatus := &corev1.PodStatus{}
	newStatus.ContainerStatuses = make([]corev1.ContainerStatus, 0)
	runningFlag := true
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			runningFlag = false
			continue
		}

		temp := &corev1.PodStatus{}
		if err = json.Unmarshal(item.Status.Raw, temp); err != nil {
			return nil, err
		}

		if temp.Phase != corev1.PodRunning {
			runningFlag = false
		}

		for _, containerStatus := range temp.ContainerStatuses {
			tempStatus := corev1.ContainerStatus{
				Ready: containerStatus.Ready,
				State: containerStatus.State,
			}
			newStatus.ContainerStatuses = append(newStatus.ContainerStatuses, tempStatus)
		}
		klog.V(3).Infof("Grab pod(%s/%s) status from cluster(%s), phase: %s", pod.Namespace,
			pod.Name, item.ClusterName, temp.Phase)
	}

	if runningFlag {
		newStatus.Phase = corev1.PodRunning
	} else {
		newStatus.Phase = corev1.PodPending
	}

	if reflect.DeepEqual(pod.Status, *newStatus) {
		klog.V(3).Infof("ignore update pod(%s/%s) status as up to date", pod.Namespace, pod.Name)
		return object, nil
	}

	pod.Status = *newStatus
	return helper.ToUnstructured(pod)
}

func aggregatePersistentVolumeClaimStatus(object *unstructured.Unstructured, aggregatedStatusItems []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	pvc, err := helper.ConvertToPersistentVolumeClaim(object)
	if err != nil {
		return nil, err
	}

	newStatus := &corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}
	for _, item := range aggregatedStatusItems {
		if item.Status == nil {
			continue
		}

		temp := &corev1.PersistentVolumeClaimStatus{}
		if err = json.Unmarshal(item.Status.Raw, temp); err != nil {
			return nil, err
		}
		klog.V(3).Infof("Grab pvc(%s/%s) status from cluster(%s), phase: %s", pvc.Namespace,
			pvc.Name, item.ClusterName, temp.Phase)
		if temp.Phase == corev1.ClaimLost {
			newStatus.Phase = corev1.ClaimLost
			break
		}

		if temp.Phase != corev1.ClaimBound {
			newStatus.Phase = temp.Phase
		}
	}

	if reflect.DeepEqual(pvc.Status, *newStatus) {
		klog.V(3).Infof("ignore update pvc(%s/%s) status as up to date", pvc.Namespace, pvc.Name)
		return object, nil
	}

	pvc.Status = *newStatus
	return helper.ToUnstructured(pvc)
}
