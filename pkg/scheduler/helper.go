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

package scheduler

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util"
)

func placementChanged(
	placement policyv1alpha1.Placement,
	appliedPlacementStr string,
	schedulerObservingAffinityName string,
) bool {
	if appliedPlacementStr == "" {
		return true
	}

	appliedPlacement := policyv1alpha1.Placement{}
	err := json.Unmarshal([]byte(appliedPlacementStr), &appliedPlacement)
	if err != nil {
		klog.Errorf("Failed to unmarshal applied placement string: %v", err)
		return false
	}

	// first check: entire placement does not change
	if reflect.DeepEqual(placement, appliedPlacement) {
		return false
	}

	// second check: except for ClusterAffinities, the placement has changed
	if !reflect.DeepEqual(placement.ClusterAffinity, appliedPlacement.ClusterAffinity) ||
		!reflect.DeepEqual(placement.ClusterTolerations, appliedPlacement.ClusterTolerations) ||
		!reflect.DeepEqual(placement.SpreadConstraints, appliedPlacement.SpreadConstraints) ||
		!reflect.DeepEqual(placement.ReplicaScheduling, appliedPlacement.ReplicaScheduling) {
		return true
	}

	// third check: check weather ClusterAffinities has changed
	return clusterAffinitiesChanged(placement.ClusterAffinities, appliedPlacement.ClusterAffinities, schedulerObservingAffinityName)
}

func clusterAffinitiesChanged(
	clusterAffinities, appliedClusterAffinities []policyv1alpha1.ClusterAffinityTerm,
	schedulerObservingAffinityName string,
) bool {
	if schedulerObservingAffinityName == "" {
		return true
	}

	var clusterAffinityTerm, appliedClusterAffinityTerm *policyv1alpha1.ClusterAffinityTerm
	for index := range clusterAffinities {
		if clusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			clusterAffinityTerm = &clusterAffinities[index]
			break
		}
	}
	for index := range appliedClusterAffinities {
		if appliedClusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			appliedClusterAffinityTerm = &appliedClusterAffinities[index]
			break
		}
	}
	if clusterAffinityTerm == nil || appliedClusterAffinityTerm == nil {
		return true
	}
	if !reflect.DeepEqual(&clusterAffinityTerm, &appliedClusterAffinityTerm) {
		return true
	}
	return false
}

func getAffinityIndex(affinities []policyv1alpha1.ClusterAffinityTerm, observedName string) int {
	if observedName == "" {
		return 0
	}

	for index, term := range affinities {
		if term.AffinityName == observedName {
			return index
		}
	}
	return 0
}

// getConditionByError returns condition by error type, bool to indicate if ignore this error.
func getConditionByError(err error) (metav1.Condition, bool) {
	if err == nil {
		return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue), true
	}

	var unschedulableErr *framework.UnschedulableError
	if errors.As(err, &unschedulableErr) {
		return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonUnschedulable, err.Error(), metav1.ConditionFalse), false
	}

	fitErrMatcher := func(e error) bool {
		var fitErr *framework.FitError
		return errors.As(e, &fitErr)
	}
	if fitErrMatcher(err) {
		return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, err.Error(), metav1.ConditionFalse), true
	}

	var aggregatedErr utilerrors.Aggregate
	if errors.As(err, &aggregatedErr) {
		for _, ae := range aggregatedErr.Errors() {
			if fitErrMatcher(ae) {
				// if aggregated NoClusterFit error got, we do not ignore error but retry scheduling.
				return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, err.Error(), metav1.ConditionFalse), false
			}
			// ResourceBinding validation webhook will return error with "FederatedResourceQuota" if quota exceeded
			var statusErr *apierrors.StatusError
			if errors.As(ae, &statusErr) && statusErr.Status().Code == http.StatusForbidden && statusErr.Status().Reason == util.QuotaExceededReason {
				return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonQuotaExceeded, ae.Error(), metav1.ConditionFalse), false
			}
		}
	}
	return util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSchedulerError, err.Error(), metav1.ConditionFalse), false
}
