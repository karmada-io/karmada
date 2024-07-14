/*
Copyright 2021 The Karmada Authors.

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

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// SetLeaseOwnerFunc helps construct a newLeasePostProcessFunc which sets
// a cluster OwnerReference to the given lease object.
func SetLeaseOwnerFunc(c client.Client, clusterName string) func(lease *coordinationv1.Lease) error {
	return func(lease *coordinationv1.Lease) error {
		// Try to set owner reference every time when renewing the lease if it is not set, until successful.
		if len(lease.OwnerReferences) == 0 {
			clusterObj := &clusterv1alpha1.Cluster{}
			if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err == nil {
				lease.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: clusterObj.APIVersion,
						Kind:       clusterObj.Kind,
						Name:       clusterName,
						UID:        clusterObj.UID,
					},
				}
			} else {
				klog.Errorf("Failed to get cluster %q when trying to set owner ref to the cluster lease: %v", clusterName, err)
				return err
			}
		}
		return nil
	}
}
