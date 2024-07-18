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

// FailoverHistoryInfo is currently not supported for the following scheduling types
//  1. Duplicated (as these resources do not failover)
//  2. Divided resources that can be scheduled across multiple clusters. In this case, state is harder to conserve since
//     the application's replicas will not be migrating together.
func restrictFailoverHistoryInfo(binding *workv1alpha2.ResourceBinding) bool {
	placement := binding.Spec.Placement
	// Check if replica scheduling type is Duplicated
	if placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return true
	}

	// Check if replica scheduling type is Divided with no spread constraints or invalid spread constraints
	if placement.ReplicaScheduling.ReplicaSchedulingType == policyv1alpha1.ReplicaSchedulingTypeDivided {
		if len(placement.SpreadConstraints) == 0 {
			return true
		}

		for _, spreadConstraint := range placement.SpreadConstraints {
			if spreadConstraint.SpreadByLabel != "" {
				return true
			}
			if spreadConstraint.SpreadByField == "cluster" && (spreadConstraint.MaxGroups > 1 || spreadConstraint.MinGroups > 1) {
				return true
			}
		}
	}

	return false
}

// UpdateFailoverStatus adds a failoverHistoryItem to the failoverHistory field in the ResourceBinding.
func UpdateFailoverStatus(client client.Client, binding *workv1alpha2.ResourceBinding, cluster string, failoverType workv1alpha2.FailoverReason) (err error) {
	if restrictFailoverHistoryInfo(binding) {
		return nil
	}
	klog.V(4).Infof("Failover triggered for replica on cluster %s", cluster)
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(context.Background(), client, binding, func() error {
			failoverHistoryItem := workv1alpha2.FailoverHistoryItem{
				StartTime:     metav1.Time{Time: time.Now()},
				OriginCluster: cluster,
				Reason:        failoverType,
				// TODO: Add remaining attributes here
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
	return nil
}
