package defaultinterpreter

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type reflectStatusInterpreter func(object *unstructured.Unstructured) (*runtime.RawExtension, error)

func getAllDefaultReflectStatusInterpreter() map[schema.GroupVersionKind]reflectStatusInterpreter {
	s := make(map[schema.GroupVersionKind]reflectStatusInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = reflectDeploymentStatus
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = reflectServiceStatus
	s[extensionsv1beta1.SchemeGroupVersion.WithKind(util.IngressKind)] = reflectIngressStatus
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = reflectJobStatus
	s[appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind)] = reflectDaemonSetStatus
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = reflectStatefulSetStatus
	return s
}

func reflectDeploymentStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	deploymentStatus, err := helper.ConvertToDeploymentStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DeploymentStatus from map[string]interface{}: %v", err)
	}

	grabStatus := appsv1.DeploymentStatus{
		Replicas:            deploymentStatus.Replicas,
		UpdatedReplicas:     deploymentStatus.UpdatedReplicas,
		ReadyReplicas:       deploymentStatus.ReadyReplicas,
		AvailableReplicas:   deploymentStatus.AvailableReplicas,
		UnavailableReplicas: deploymentStatus.UnavailableReplicas,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func reflectServiceStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	serviceType, exist, err := unstructured.NestedString(object.Object, "spec", "type")
	if err != nil {
		klog.Errorf("Failed to get spec.type field from %s(%s/%s)")
	}
	if !exist {
		klog.Errorf("Failed to get spec.type from %s(%s/%s) which should have spec.type field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}
	if serviceType != string(corev1.ServiceTypeLoadBalancer) {
		return nil, nil
	}

	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	serviceStatus, err := helper.ConvertToServiceStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ServiceStatus from map[string]interface{}: %v", err)
	}

	grabStatus := corev1.ServiceStatus{
		LoadBalancer: serviceStatus.LoadBalancer,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func reflectIngressStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	return reflectWholeStatus(object)
}

func reflectJobStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	jobStatus, err := helper.ConvertToJobStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JobStatus from map[string]interface{}: %v", err)
	}

	grabStatus := batchv1.JobStatus{
		Active:         jobStatus.Active,
		Succeeded:      jobStatus.Succeeded,
		Failed:         jobStatus.Failed,
		Conditions:     jobStatus.Conditions,
		StartTime:      jobStatus.StartTime,
		CompletionTime: jobStatus.CompletionTime,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func reflectDaemonSetStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	daemonSetStatus, err := helper.ConvertToDaemonSetStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DaemonSetStatus from map[string]interface{}: %v", err)
	}

	grabStatus := appsv1.DaemonSetStatus{
		CurrentNumberScheduled: daemonSetStatus.CurrentNumberScheduled,
		DesiredNumberScheduled: daemonSetStatus.DesiredNumberScheduled,
		NumberAvailable:        daemonSetStatus.NumberAvailable,
		NumberMisscheduled:     daemonSetStatus.NumberMisscheduled,
		NumberReady:            daemonSetStatus.NumberReady,
		UpdatedNumberScheduled: daemonSetStatus.UpdatedNumberScheduled,
		NumberUnavailable:      daemonSetStatus.NumberUnavailable,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func reflectStatefulSetStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if !exist {
		klog.Errorf("Failed to grab status from %s(%s/%s) which should have status field.",
			object.GetKind(), object.GetNamespace(), object.GetName())
		return nil, nil
	}

	statefulSetStatus, err := helper.ConvertToStatefulSetStatus(statusMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert StatefulSetStatus from map[string]interface{}: %v", err)
	}

	grabStatus := appsv1.StatefulSetStatus{
		AvailableReplicas: statefulSetStatus.AvailableReplicas,
		CurrentReplicas:   statefulSetStatus.CurrentReplicas,
		ReadyReplicas:     statefulSetStatus.ReadyReplicas,
		Replicas:          statefulSetStatus.Replicas,
		UpdatedReplicas:   statefulSetStatus.UpdatedReplicas,
	}
	return helper.BuildStatusRawExtension(grabStatus)
}

func reflectWholeStatus(object *unstructured.Unstructured) (*runtime.RawExtension, error) {
	statusMap, exist, err := unstructured.NestedMap(object.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status field from %s(%s/%s), error: %v",
			object.GetKind(), object.GetNamespace(), object.GetName(), err)
		return nil, err
	}
	if exist {
		return helper.BuildStatusRawExtension(statusMap)
	}
	return nil, nil
}
