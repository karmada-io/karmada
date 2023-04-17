package util

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Constants defining labels
const (
	StatusReady      = "Ready"
	StatusInProgress = "InProgress"
	StatusUnknown    = "Unknown"
)

// WaitForStatefulSetRollout wait for StatefulSet reaches the ready state or timeout.
func WaitForStatefulSetRollout(c kubernetes.Interface, sts *appsv1.StatefulSet, timeoutSeconds time.Duration) error {
	stsWatcher, err := c.AppsV1().StatefulSets(sts.GetNamespace()).Watch(context.Background(), metav1.SingleObject(sts.ObjectMeta))
	if err != nil {
		return err
	}
	defer stsWatcher.Stop()

	for {
		select {
		case <-time.After(timeoutSeconds * time.Second):
			klog.Errorf("wait for StatefulSets(%s/%s) rollout: timeout", sts.GetNamespace(), sts.GetName())
			return nil
		case event, ok := <-stsWatcher.ResultChan():
			if !ok {
				return fmt.Errorf("wait for Statefulset(%s/%s) rollout: channel broken", sts.GetNamespace(), sts.GetName())
			}

			status, err := stsStatus(event.Object)
			if err != nil {
				return err
			} else if status == StatusReady {
				return nil
			}
		}
	}
}

// WaitForDeploymentRollout  wait for Deployment reaches the ready state or timeout.
func WaitForDeploymentRollout(c kubernetes.Interface, dep *appsv1.Deployment, timeoutSeconds time.Duration) error {
	depWatcher, err := c.AppsV1().Deployments(dep.GetNamespace()).Watch(context.Background(), metav1.SingleObject(dep.ObjectMeta))
	if err != nil {
		return err
	}
	defer depWatcher.Stop()

	for {
		select {
		case <-time.After(timeoutSeconds * time.Second):
			klog.Errorf("wait for Deployment(%s/%s) rollout: timeout", dep.GetNamespace(), dep.GetName())
			return nil
		case event, ok := <-depWatcher.ResultChan():
			if !ok {
				return fmt.Errorf("wait for Deployment(%s/%s) rollout: channel broken", dep.GetNamespace(), dep.GetName())
			}

			status, err := deploymentStatus(event.Object)
			if err != nil {
				return err
			} else if status == StatusReady {
				return nil
			}
		}
	}
}

// Reference https://github.com/kubernetes-sigs/application/blob/e5329b1b083ece8abb4b4efaf738ab9277fbb2ce/controllers/status.go#L78-#L92
func stsStatus(o runtime.Object) (string, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return StatusUnknown, err
	}

	sts := &appsv1.StatefulSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, sts); err != nil {
		return StatusUnknown, err
	}

	if sts.Status.ObservedGeneration == sts.Generation &&
		sts.Status.Replicas == *sts.Spec.Replicas &&
		sts.Status.ReadyReplicas == *sts.Spec.Replicas &&
		sts.Status.CurrentReplicas == *sts.Spec.Replicas {
		return StatusReady, nil
	}
	return StatusInProgress, nil
}

// Reference https://github.com/kubernetes-sigs/application/blob/e5329b1b083ece8abb4b4efaf738ab9277fbb2ce/controllers/status.go#L94-#L132
//
//nolint:gocyclo
func deploymentStatus(o runtime.Object) (string, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return StatusUnknown, err
	}

	deployment := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u, deployment); err != nil {
		return StatusUnknown, err
	}

	replicaFailure := false
	progressing := false
	available := false

	for _, condition := range deployment.Status.Conditions {
		switch condition.Type {
		case appsv1.DeploymentProgressing:
			if condition.Status == corev1.ConditionTrue && condition.Reason == "NewReplicaSetAvailable" {
				progressing = true
			}
		case appsv1.DeploymentAvailable:
			if condition.Status == corev1.ConditionTrue {
				available = true
			}
		case appsv1.DeploymentReplicaFailure:
			if condition.Status == corev1.ConditionTrue {
				replicaFailure = true
				break
			}
		}
	}

	if deployment.Status.ObservedGeneration == deployment.Generation &&
		deployment.Status.Replicas == *deployment.Spec.Replicas &&
		deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
		deployment.Status.AvailableReplicas == *deployment.Spec.Replicas &&
		deployment.Status.Conditions != nil && len(deployment.Status.Conditions) > 0 &&
		(progressing || available) && !replicaFailure {
		return StatusReady, nil
	}
	return StatusInProgress, nil
}
