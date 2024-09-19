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

package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// FullyAppliedSuccessReason defines the success reason for the FullyApplied condition.
	FullyAppliedSuccessReason = "FullyAppliedSuccess"
	// FullyAppliedFailedReason defines the failure reason for the FullyApplied condition.
	FullyAppliedFailedReason = "FullyAppliedFailed"
	// FullyAppliedSuccessMessage defines the success message for the FullyApplied condition.
	FullyAppliedSuccessMessage = "All works have been successfully applied"
	// FullyAppliedFailedMessage defines the failure message for the FullyApplied condition.
	FullyAppliedFailedMessage = "Failed to apply all works, see status.aggregatedStatus for details"
)

// AggregateResourceBindingWorkStatus will collect all work statuses with current ResourceBinding objects,
// then aggregate status info to current ResourceBinding status.
func AggregateResourceBindingWorkStatus(
	ctx context.Context,
	c client.Client,
	binding *workv1alpha2.ResourceBinding,
	eventRecorder record.EventRecorder,
) error {
	workList, err := GetWorksByBindingID(ctx, c, binding.Labels[workv1alpha2.ResourceBindingPermanentIDLabel], true)
	if err != nil {
		return err
	}

	aggregatedStatuses, err := assembleWorkStatus(workList.Items, binding.Spec.Resource)
	if err != nil {
		return err
	}

	fullyAppliedCondition := generateFullyAppliedCondition(binding.Spec, aggregatedStatuses)

	var operationResult controllerutil.OperationResult
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		operationResult, err = UpdateStatus(ctx, c, binding, func() error {
			binding.Status.AggregatedStatus = aggregatedStatuses
			// set binding status with the newest condition
			meta.SetStatusCondition(&binding.Status.Conditions, fullyAppliedCondition)
			return nil
		})
		return err
	}); err != nil {
		eventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonAggregateStatusFailed, err.Error())
		return err
	}

	if operationResult == controllerutil.OperationResultUpdatedStatusOnly {
		msg := fmt.Sprintf("Update ResourceBinding(%s/%s) with AggregatedStatus successfully.", binding.Namespace, binding.Name)
		eventRecorder.Event(binding, corev1.EventTypeNormal, events.EventReasonAggregateStatusSucceed, msg)
	} else {
		klog.Infof("New aggregatedStatuses are equal with old ResourceBinding(%s/%s) AggregatedStatus, no update required.", binding.Namespace, binding.Name)
	}
	return nil
}

// AggregateClusterResourceBindingWorkStatus will collect all work statuses with current ClusterResourceBinding objects,
// then aggregate status info to current ClusterResourceBinding status.
func AggregateClusterResourceBindingWorkStatus(
	ctx context.Context,
	c client.Client,
	binding *workv1alpha2.ClusterResourceBinding,
	eventRecorder record.EventRecorder,
) error {
	workList, err := GetWorksByBindingID(ctx, c, binding.Labels[workv1alpha2.ClusterResourceBindingPermanentIDLabel], false)
	if err != nil {
		return err
	}

	aggregatedStatuses, err := assembleWorkStatus(workList.Items, binding.Spec.Resource)
	if err != nil {
		return err
	}

	fullyAppliedCondition := generateFullyAppliedCondition(binding.Spec, aggregatedStatuses)

	var operationResult controllerutil.OperationResult
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		operationResult, err = UpdateStatus(ctx, c, binding, func() error {
			binding.Status.AggregatedStatus = aggregatedStatuses
			// set binding status with the newest condition
			meta.SetStatusCondition(&binding.Status.Conditions, fullyAppliedCondition)
			return nil
		})
		return err
	}); err != nil {
		eventRecorder.Event(binding, corev1.EventTypeWarning, events.EventReasonAggregateStatusFailed, err.Error())
		return err
	}

	if operationResult == controllerutil.OperationResultUpdatedStatusOnly {
		msg := fmt.Sprintf("Update ClusterResourceBinding(%s) with AggregatedStatus successfully.", binding.Name)
		eventRecorder.Event(binding, corev1.EventTypeNormal, events.EventReasonAggregateStatusSucceed, msg)
	} else {
		klog.Infof("New aggregatedStatuses are equal with old ClusterResourceBinding(%s) AggregatedStatus, no update required.", binding.Name)
	}

	return nil
}

func generateFullyAppliedCondition(spec workv1alpha2.ResourceBindingSpec, aggregatedStatuses []workv1alpha2.AggregatedStatusItem) metav1.Condition {
	clusterNames := ObtainBindingSpecExistingClusters(spec)
	if worksFullyApplied(aggregatedStatuses, clusterNames) {
		return util.NewCondition(workv1alpha2.FullyApplied, FullyAppliedSuccessReason, FullyAppliedSuccessMessage, metav1.ConditionTrue)
	}
	return util.NewCondition(workv1alpha2.FullyApplied, FullyAppliedFailedReason, FullyAppliedFailedMessage, metav1.ConditionFalse)
}

// assemble workStatuses from workList which list by selector and match with workload.
func assembleWorkStatus(works []workv1alpha1.Work, objRef workv1alpha2.ObjectReference) ([]workv1alpha2.AggregatedStatusItem, error) {
	statuses := make([]workv1alpha2.AggregatedStatusItem, 0)
	for _, work := range works {
		if !work.DeletionTimestamp.IsZero() {
			continue
		}

		manifestRef := ManifestReference{APIVersion: objRef.APIVersion, Kind: objRef.Kind, Namespace: objRef.Namespace, Name: objRef.Name}
		identifierIndex, err := GetManifestIndex(work.Spec.Workload.Manifests, &manifestRef)
		if err != nil {
			klog.Errorf("Failed to get manifestIndex of workload in work.Spec.Workload.Manifests. Error: %v.", err)
			return nil, err
		}
		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.Errorf("Failed to get clusterName from work namespace %s. Error: %v.", work.Namespace, err)
			return nil, err
		}

		// if sync work to member cluster failed, then set status back to resource binding.
		var applied bool
		var appliedMsg string
		if cond := meta.FindStatusCondition(work.Status.Conditions, workv1alpha1.WorkApplied); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				applied = true
			case metav1.ConditionUnknown:
				fallthrough
			case metav1.ConditionFalse:
				applied = false
				appliedMsg = cond.Message
			default: // should not happen unless the condition api changed.
				panic("unexpected status")
			}
		}
		if !applied {
			aggregatedStatus := workv1alpha2.AggregatedStatusItem{
				ClusterName:    clusterName,
				Applied:        applied,
				AppliedMessage: appliedMsg,
				Health:         workv1alpha2.ResourceUnknown,
			}
			statuses = append(statuses, aggregatedStatus)
			continue
		}

		// resources with no status,only record whether the propagation is successful in work
		aggregatedStatus := workv1alpha2.AggregatedStatusItem{
			ClusterName: clusterName,
			Applied:     applied,
			Health:      workv1alpha2.ResourceUnknown,
		}

		for i := range work.Status.ManifestStatuses {
			equal, err := equalIdentifier(&work.Status.ManifestStatuses[i].Identifier, identifierIndex, &manifestRef)
			if err != nil {
				return nil, err
			}
			if equal {
				aggregatedStatus.Status = work.Status.ManifestStatuses[i].Status
				aggregatedStatus.Health = workv1alpha2.ResourceHealth(work.Status.ManifestStatuses[i].Health)
				break
			}
		}
		statuses = append(statuses, aggregatedStatus)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ClusterName < statuses[j].ClusterName
	})
	return statuses, nil
}

// ManifestReference identifies an object in manifest list
type ManifestReference struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// GetManifestIndex gets the index of clusterObj in manifest list, if not exist return -1.
func GetManifestIndex(manifests []workv1alpha1.Manifest, manifestRef *ManifestReference) (int, error) {
	for index, rawManifest := range manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return -1, err
		}
		if manifest.GetAPIVersion() == manifestRef.APIVersion &&
			manifest.GetKind() == manifestRef.Kind &&
			manifest.GetNamespace() == manifestRef.Namespace &&
			manifest.GetName() == manifestRef.Name {
			return index, nil
		}
	}

	return -1, fmt.Errorf("no such manifest exist")
}

func equalIdentifier(targetIdentifier *workv1alpha1.ResourceIdentifier, ordinal int, manifestRef *ManifestReference) (bool, error) {
	groupVersion, err := schema.ParseGroupVersion(manifestRef.APIVersion)
	if err != nil {
		return false, err
	}

	if targetIdentifier.Ordinal == ordinal &&
		targetIdentifier.Group == groupVersion.Group &&
		targetIdentifier.Version == groupVersion.Version &&
		targetIdentifier.Kind == manifestRef.Kind &&
		targetIdentifier.Namespace == manifestRef.Namespace &&
		targetIdentifier.Name == manifestRef.Name {
		return true, nil
	}

	return false, nil
}

// worksFullyApplied checks if all works are applied according the scheduled result and collected status.
func worksFullyApplied(aggregatedStatuses []workv1alpha2.AggregatedStatusItem, targetClusters sets.Set[string]) bool {
	// short path: not scheduled
	if len(targetClusters) == 0 {
		return false
	}

	// short path: lack of status
	if len(targetClusters) != len(aggregatedStatuses) {
		return false
	}

	for _, aggregatedStatusItem := range aggregatedStatuses {
		if !aggregatedStatusItem.Applied {
			return false
		}

		if !targetClusters.Has(aggregatedStatusItem.ClusterName) {
			return false
		}
	}

	return true
}

// IsResourceApplied checks whether resource has been dispatched to member cluster or not
func IsResourceApplied(workStatus *workv1alpha1.WorkStatus) bool {
	return meta.IsStatusConditionTrue(workStatus.Conditions, workv1alpha1.WorkApplied)
}

// BuildStatusRawExtension builds raw JSON by a status map.
func BuildStatusRawExtension(status interface{}) (*runtime.RawExtension, error) {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		klog.Errorf("Failed to marshal status. Error: %v.", statusJSON)
		return nil, err
	}

	return &runtime.RawExtension{
		Raw: statusJSON,
	}, nil
}
