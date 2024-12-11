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

// EnsureWorksDeleted ensures that all Work resources in the specified namespace are deleted.
func EnsureWorksDeleted(controlPlaneKarmadaClient karmadaclientset.Interface, namespace string,
	timeout time.Duration, forceDeletion bool) error {
	// make sure the works object under the given namespace has been deleted.
	err := wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, timeout, false, func(context.Context) (done bool, err error) {
		list, err := controlPlaneKarmadaClient.WorkV1alpha1().Works(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to list work in namespace %s", namespace)
		}

		if len(list.Items) == 0 {
			return true, nil
		}
		for i := range list.Items {
			work := &list.Items[i]
			err = controlPlaneKarmadaClient.WorkV1alpha1().Works(namespace).Delete(context.TODO(), work.GetName(), metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("failed to delete the work(%s/%s)", namespace, work.GetName())
			}
		}
		return false, nil
	})

	// If the Works not be deleted within the timeout period, it is likely due to the resources in the member
	// cluster can not be cleaned up. With the option force deletion, we will try to clean up the Works object by
	// removing the finalizers from related resources. This behavior may result in some resources remain in the member
	// clusters.
	if err != nil && forceDeletion {
		klog.Warningf("Deleting the work object timed out. ExecutionSpace: %s, error: %v", namespace, err)
		klog.Infof("Start forced deletion. Deleting finalizer of works in ExecutionSpace: %s", namespace)
		err = RemoveWorkFinalizer(namespace, controlPlaneKarmadaClient)
		if err != nil {
			klog.Errorf("Force deletion. Failed to remove the finalizer of Work, error: %v", err)
		}

		klog.Infof("Work object force deletion is complete.")
		return nil
	}

	return err
}
