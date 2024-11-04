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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// DeleteClusterObject deletes the cluster object from the Karmada control plane.
func DeleteClusterObject(controlPlaneKubeClient kubeclient.Interface, controlPlaneKarmadaClient karmadaclientset.Interface, clusterName string,
	timeout time.Duration, dryRun bool, forceDeletion bool) error {
	if dryRun {
		return nil
	}

	err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("no cluster object %s found in karmada control Plane", clusterName)
	}
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", clusterName, err)
		return err
	}

	// make sure the given cluster object has been deleted.
	// If the operation times out and `forceDeletion` is true, then force deletion begins, which involves sequentially deleting the `work`, `executionSpace`, and `cluster` finalizers.
	err = wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, timeout, false, func(context.Context) (done bool, err error) {
		_, err = controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			klog.Errorf("Failed to get cluster %s. err: %v", clusterName, err)
			return false, err
		}
		klog.Infof("Waiting for the cluster object %s to be deleted", clusterName)
		return false, nil
	})

	// If the Cluster object not be deleted within the timeout period, it is likely due to the resources in the member
	// cluster can not be cleaned up. With the option force deletion, we will try to clean up the Cluster object by
	// removing the finalizers from related resources. This behavior may result in some resources remain in the member
	// clusters.
	if err != nil && forceDeletion {
		klog.Warningf("Deleting the cluster object timed out. cluster name: %s, error: %v", clusterName, err)
		klog.Infof("Start forced deletion. cluster name: %s", clusterName)
		executionSpaceName := names.GenerateExecutionSpaceName(clusterName)
		err = removeWorkFinalizer(executionSpaceName, controlPlaneKarmadaClient)
		if err != nil {
			klog.Errorf("Force deletion. Failed to remove the finalizer of Work, error: %v", err)
		}

		err = removeExecutionSpaceFinalizer(executionSpaceName, controlPlaneKubeClient)
		if err != nil {
			klog.Errorf("Force deletion. Failed to remove the finalizer of Namespace(%s), error: %v", executionSpaceName, err)
		}

		err = removeClusterFinalizer(clusterName, controlPlaneKarmadaClient)
		if err != nil {
			klog.Errorf("Force deletion. Failed to remove the finalizer of Cluster(%s), error: %v", clusterName, err)
		}

		klog.Infof("Forced deletion is complete.")
		return nil
	}

	return err
}

// removeWorkFinalizer removes the finalizer of works in the executionSpace.
func removeWorkFinalizer(executionSpaceName string, controlPlaneKarmadaClient karmadaclientset.Interface) error {
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

// removeExecutionSpaceFinalizer removes the finalizer of executionSpace.
func removeExecutionSpaceFinalizer(executionSpaceName string, controlPlaneKubeClient kubeclient.Interface) error {
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

// removeClusterFinalizer removes the finalizer of cluster object.
func removeClusterFinalizer(clusterName string, controlPlaneKarmadaClient karmadaclientset.Interface) error {
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
