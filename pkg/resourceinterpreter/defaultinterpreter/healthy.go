package defaultinterpreter

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type healthInterpreter func(object *unstructured.Unstructured) (bool, error)

func getAllDefaultHealthInterpreter() map[schema.GroupVersionKind]healthInterpreter {
	s := make(map[schema.GroupVersionKind]healthInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = interpretDeploymentHealth
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = interpretStatefulSetHealth
	s[appsv1.SchemeGroupVersion.WithKind(util.ReplicaSetKind)] = interpretReplicaSetHealth
	s[appsv1.SchemeGroupVersion.WithKind(util.DaemonSetKind)] = interpretDaemonSetHealth
	s[corev1.SchemeGroupVersion.WithKind(util.ServiceKind)] = interpretServiceHealth
	s[networkingv1.SchemeGroupVersion.WithKind(util.IngressKind)] = interpretIngressHealth
	s[corev1.SchemeGroupVersion.WithKind(util.PersistentVolumeClaimKind)] = interpretPersistentVolumeClaimHealth
	return s
}

func interpretDeploymentHealth(object *unstructured.Unstructured) (bool, error) {
	deploy := &appsv1.Deployment{}
	err := helper.ConvertToTypedObject(object, deploy)
	if err != nil {
		return false, err
	}

	healthy := (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) && (deploy.Generation == deploy.Status.ObservedGeneration)
	return healthy, nil
}

func interpretStatefulSetHealth(object *unstructured.Unstructured) (bool, error) {
	statefulSet := &appsv1.StatefulSet{}
	err := helper.ConvertToTypedObject(object, statefulSet)
	if err != nil {
		return false, err
	}

	healthy := (statefulSet.Status.UpdatedReplicas == *statefulSet.Spec.Replicas) && (statefulSet.Generation == statefulSet.Status.ObservedGeneration)
	return healthy, nil
}

func interpretReplicaSetHealth(object *unstructured.Unstructured) (bool, error) {
	replicaSet := &appsv1.ReplicaSet{}
	err := helper.ConvertToTypedObject(object, replicaSet)
	if err != nil {
		return false, err
	}

	if replicaSet.Generation != replicaSet.Status.ObservedGeneration {
		return false, nil
	}
	if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
		return false, nil
	}
	return true, nil
}

func interpretDaemonSetHealth(object *unstructured.Unstructured) (bool, error) {
	daemonSet := &appsv1.DaemonSet{}
	err := helper.ConvertToTypedObject(object, daemonSet)
	if err != nil {
		return false, err
	}

	if daemonSet.Generation != daemonSet.Status.ObservedGeneration {
		return false, nil
	}
	if daemonSet.Status.UpdatedNumberScheduled < daemonSet.Status.DesiredNumberScheduled {
		return false, nil
	}
	if daemonSet.Status.NumberAvailable < daemonSet.Status.DesiredNumberScheduled {
		return false, nil
	}

	return true, nil
}

func interpretServiceHealth(object *unstructured.Unstructured) (bool, error) {
	service := &corev1.Service{}
	err := helper.ConvertToTypedObject(object, service)
	if err != nil {
		return false, err
	}

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return true, nil
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" || ingress.IP != "" {
			return true, nil
		}
	}

	return false, nil
}

func interpretIngressHealth(object *unstructured.Unstructured) (bool, error) {
	ingress := &networkingv1.Ingress{}
	err := helper.ConvertToTypedObject(object, ingress)
	if err != nil {
		return false, err
	}

	for _, ing := range ingress.Status.LoadBalancer.Ingress {
		if ing.Hostname != "" || ing.IP != "" {
			return true, nil
		}
	}

	return false, nil
}

func interpretPersistentVolumeClaimHealth(object *unstructured.Unstructured) (bool, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := helper.ConvertToTypedObject(object, pvc)
	if err != nil {
		return false, err
	}

	return pvc.Status.Phase == corev1.ClaimBound, nil
}
