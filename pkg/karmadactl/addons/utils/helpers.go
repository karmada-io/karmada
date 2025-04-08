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

package utils

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	// WaitForDeploymentRollout waits for the specified Deployment to reach its desired state within the given timeout.
	// This blocks until the Deployment's observed generation and ready replicas match the desired state,
	// ensuring it is fully rolled out.
	WaitForDeploymentRollout = func(c clientset.Interface, dep *appsv1.Deployment, timeoutSeconds int) error {
		return cmdutil.WaitForDeploymentRollout(c, dep, time.Duration(timeoutSeconds)*time.Second)
	}
)

// SimulateNetworkErrorOnOp simulates a network error during the specified
// operation on a resource by prepending a reactor to the fake client.
func SimulateNetworkErrorOnOp(c clientset.Interface, operation, resource string) error {
	c.(*fakeclientset.Clientset).Fake.PrependReactor(operation, resource, func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("unexpected error: encountered a network issue while %s the %s", operation, resource)
	})
	return nil
}

// SimulateDeploymentUnready simulates a "not ready" status by incrementing the replicas
// of the specified Deployment, thus marking it as unready. This is useful for testing the handling
// of Deployment readiness in Karmada.
func SimulateDeploymentUnready(c clientset.Interface, name, namespace string) error {
	deployment, err := c.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s in namespace %s, got error: %v", name, namespace, err)
	}

	deployment.Status.Replicas = *deployment.Spec.Replicas + 1
	_, err = c.AppsV1().Deployments(namespace).UpdateStatus(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update replicas status of deployment %s in namespace %s, got error: %v", name, namespace, err)
	}

	return nil
}
