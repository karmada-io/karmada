/*
Copyright 2020 The Karmada Authors.

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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// FailoverHistoryInfoIsSupported verifies that the resource's placement supports FailoverHistory feature
func FailoverHistoryInfoIsSupported(binding *workv1alpha2.ResourceBinding) bool {
	placementPtr := binding.Spec.Placement
	if placementPtr == nil {
		return false
	}
	return verifySchedulingTypeSupported(placementPtr.ReplicaScheduling, placementPtr.SpreadConstraints)
}

// FailoverHistoryInfo is currently not supported for the following scheduling types
//  1. Duplicated (as these resources do not failover)
//  2. Divided resources that can be scheduled across multiple clusters. In this case, state is harder to conserve since
//     the application's replicas will not be migrating together.
func verifySchedulingTypeSupported(schedulingPtr *policyv1alpha1.ReplicaSchedulingStrategy, spreadConstraints []policyv1alpha1.SpreadConstraint) bool {
	if schedulingPtr == nil {
		return false
	}
	switch schedulingType := schedulingPtr.ReplicaSchedulingType; schedulingType {
	case policyv1alpha1.ReplicaSchedulingTypeDuplicated:
		return false
	// Handles divided and nil case
	default:
		if len(spreadConstraints) == 0 {
			return false
		}
		for _, spreadConstraint := range spreadConstraints {
			if spreadConstraint.SpreadByLabel != "" {
				return false
			}
			if spreadConstraint.SpreadByField == "cluster" && (spreadConstraint.MaxGroups > 1 || spreadConstraint.MinGroups > 1) {
				return false
			}
		}
	}
	return true
}

// UpdateFailoverStatus adds a failoverHistoryItem to the failoverHistory field in the ResourceBinding.
func UpdateFailoverStatus(client client.Client, binding *workv1alpha2.ResourceBinding, cluster string, failoverType workv1alpha2.FailoverReason) (err error) {
	if FailoverHistoryInfoIsSupported(binding) {
		klog.V(4).Infof("Failover triggered for replica on cluster %s", cluster)
		err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
			_, err = helper.UpdateStatus(context.Background(), client, binding, func() error {
				failoverHistoryItem := workv1alpha2.FailoverHistoryItem{
					FromCluster:            cluster,
					StartTime:              metav1.Time{Time: time.Now()},
					Reason:                 failoverType,
					ClustersBeforeFailover: binding.Spec.Clusters,
				}
				binding.Status.FailoverHistory = append(binding.Status.FailoverHistory, failoverHistoryItem)
				return nil
			})
			return err
		})

		if err != nil {
			klog.Errorf("Failed to update FailoverHistoryInfo to ResourceBinding %s/%s. Error: %v", binding.Namespace, binding.Name, err)
			return err
		}
	}
	return nil
}
