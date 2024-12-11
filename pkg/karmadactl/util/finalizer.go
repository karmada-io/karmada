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

package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
)

// RemoveWorkFinalizer removes the finalizer of works in the executionSpace.
func RemoveWorkFinalizer(executionSpaceName string, controlPlaneKarmadaClient karmadaclientset.Interface) error {
	list, err := controlPlaneKarmadaClient.WorkV1alpha1().Works(executionSpaceName).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list work in executionSpace %s", executionSpaceName)
	}

	for i := range list.Items {
		work := &list.Items[i]
		if !controllerutil.ContainsFinalizer(work, util.ExecutionControllerFinalizer) {
			continue
		}
		controllerutil.RemoveFinalizer(work, util.ExecutionControllerFinalizer)
		_, err = controlPlaneKarmadaClient.WorkV1alpha1().Works(executionSpaceName).Update(context.TODO(), work, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to remove the finalizer of work(%s/%s)", executionSpaceName, work.GetName())
		}
	}
	return nil
}

// RemoveExecutionSpaceFinalizer removes the finalizer of executionSpace.
func RemoveExecutionSpaceFinalizer(executionSpaceName string, controlPlaneKubeClient kubeclient.Interface) error {
	executionSpace, err := controlPlaneKubeClient.CoreV1().Namespaces().Get(context.TODO(), executionSpaceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Namespace(%s)", executionSpaceName)
	}

	if !controllerutil.ContainsFinalizer(executionSpace, string(corev1.FinalizerKubernetes)) {
		return nil
	}

	controllerutil.RemoveFinalizer(executionSpace, "kubernetes")
	_, err = controlPlaneKubeClient.CoreV1().Namespaces().Update(context.TODO(), executionSpace, metav1.UpdateOptions{})

	return err
}

// RemoveClusterFinalizer removes the finalizer of cluster object.
func RemoveClusterFinalizer(clusterName string, controlPlaneKarmadaClient karmadaclientset.Interface) error {
	cluster, err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Cluster(%s)", clusterName)
	}

	if !controllerutil.ContainsFinalizer(cluster, util.ClusterControllerFinalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(cluster, util.ClusterControllerFinalizer)
	_, err = controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})

	return err
}
