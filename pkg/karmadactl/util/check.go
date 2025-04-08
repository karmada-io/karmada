/*
Copyright 2023 The Karmada Authors.

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

package util

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// WaitForStatefulSetRollout wait for StatefulSet reaches the ready state or timeout.
func WaitForStatefulSetRollout(c kubernetes.Interface, sts *appsv1.StatefulSet, timeoutSeconds int) error {
	var lastErr error
	pollError := wait.PollUntilContextTimeout(context.TODO(), time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		s, err := c.AppsV1().StatefulSets(sts.GetNamespace()).Get(ctx, sts.GetName(), metav1.GetOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		if s.Generation != s.Status.ObservedGeneration {
			lastErr = fmt.Errorf("expected generation %d, observed generation: %d",
				s.Generation, s.Status.ObservedGeneration)
			return false, nil
		}
		if (s.Spec.Replicas != nil) && (s.Status.UpdatedReplicas < *s.Spec.Replicas) {
			lastErr = fmt.Errorf("expected %d replicas, got %d updated replicas",
				*s.Spec.Replicas, s.Status.UpdatedReplicas)
			return false, nil
		}
		if s.Status.AvailableReplicas < s.Status.UpdatedReplicas {
			lastErr = fmt.Errorf("expected %d replicas, got %d available replicas",
				s.Status.UpdatedReplicas, s.Status.AvailableReplicas)
			return false, nil
		}
		return true, nil
	})
	if pollError != nil {
		return fmt.Errorf("wait for Statefulset(%s/%s) rollout: %v: %v", sts.GetNamespace(), sts.GetName(), pollError, lastErr)
	}
	return nil
}

// WaitForDeploymentRollout wait for Deployment reaches the ready state or timeout.
func WaitForDeploymentRollout(c kubernetes.Interface, dep *appsv1.Deployment, timeout time.Duration) error {
	var lastErr error
	pollError := wait.PollUntilContextTimeout(context.TODO(), time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		d, err := c.AppsV1().Deployments(dep.GetNamespace()).Get(ctx, dep.GetName(), metav1.GetOptions{})
		if err != nil {
			lastErr = err
			return false, nil
		}
		if d.Generation != d.Status.ObservedGeneration {
			lastErr = fmt.Errorf("current generation %d, observed generation %d",
				d.Generation, d.Status.ObservedGeneration)
			return false, nil
		}
		if (d.Spec.Replicas != nil) && (d.Status.UpdatedReplicas < *d.Spec.Replicas) {
			lastErr = fmt.Errorf("the number of pods targeted by the deployment (%d pods) is different "+
				"from the number of pods targeted by the deployment that have the desired template spec (%d pods)",
				*d.Spec.Replicas, d.Status.UpdatedReplicas)
			return false, nil
		}
		if d.Status.AvailableReplicas < d.Status.UpdatedReplicas {
			lastErr = fmt.Errorf("expected %d replicas, got %d available replicas",
				d.Status.UpdatedReplicas, d.Status.AvailableReplicas)
			return false, nil
		}
		return true, nil
	})
	if pollError != nil {
		return fmt.Errorf("wait for Deployment(%s/%s) rollout: %v: %v", dep.GetNamespace(), dep.GetName(), pollError, lastErr)
	}
	return nil
}
