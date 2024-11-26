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

// UpdateFailoverStatus adds a failoverHistoryItem to the failoverHistory field in the ResourceBinding.
func UpdateFailoverStatus(client client.Client, binding *workv1alpha2.ResourceBinding, clusters []string, failoverType workv1alpha2.FailoverReason) (err error) {
	// If the resource is Duplicated, then it does not have a concept of failover. We skip attaching that status here.
	placement := binding.Spec.Placement
	if placement != nil && placement.ReplicaSchedulingType() == policyv1alpha1.ReplicaSchedulingTypeDuplicated {
		return nil
	}
	klog.V(4).Infof("Updating failover status for ResourceBinding(%s/%s)", binding.Name, binding.Namespace)
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(context.Background(), client, binding, func() error {
			failoverHistoryItem := workv1alpha2.FailoverHistoryItem{
				StartTime:              metav1.Time{Time: time.Now()},
				Reason:                 failoverType,
				ClustersBeforeFailover: clusters,
				ClustersAfterFailover:  []string{},
			}
			binding.Status.FailoverHistory = append(binding.Status.FailoverHistory, failoverHistoryItem)
			// ToDo: Consider parametrizing failover history length
			historyLength := len(binding.Status.FailoverHistory)
			if historyLength >= 3 {
				binding.Status.FailoverHistory = binding.Status.FailoverHistory[historyLength-3:]
			}
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
