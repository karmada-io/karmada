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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

// DeleteClusterObject delete the cluster object in host cluster
func DeleteClusterObject(controlPlaneKarmadaClient *karmadaclientset.Clientset, clusterName string,
	timeout time.Duration, dryRun bool) error {
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

	// make sure the given cluster object has been deleted
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
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", clusterName, err)
		return err
	}

	return nil
}
