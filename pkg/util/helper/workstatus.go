/*
Copyright The Karmada Authors.

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
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// AggregateResourceBindingWorkStatus will collect all work statuses with current ResourceBinding objects,
// then aggregate status info to current ResourceBinding status.
func AggregateResourceBindingWorkStatus(c client.Client, binding *workv1alpha2.ResourceBinding, workload *unstructured.Unstructured) error {
	var targetWorks []workv1alpha1.Work

	// TODO: Delete this logic in the next release to prevent incompatibility when upgrading the current release (v0.10.0).
	workList, err := GetWorksByLabelSelector(c, labels.SelectorFromSet(
		labels.Set{
			workv1alpha2.ResourceBindingNamespaceLabel: binding.Namespace,
			workv1alpha2.ResourceBindingNameLabel:      binding.Name,
		},
	))
	if err != nil {
		klog.Errorf("Failed to get works by ResourceBinding(%s/%s): %v", binding.Namespace, binding.Name, err)
		return err
	}
	targetWorks = append(targetWorks, workList.Items...)

	workList, err = GetWorksByLabelSelector(c, labels.SelectorFromSet(
		labels.Set{
			workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey(binding.Namespace, binding.Name),
		},
	))
	if err != nil {
		klog.Errorf("Failed to get works by ResourceBinding(%s/%s): %v", binding.Namespace, binding.Name, err)
		return err
	}
	targetWorks = append(targetWorks, workList.Items...)

	aggregatedStatuses, err := assembleWorkStatus(targetWorks, workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.V(4).Infof("New aggregatedStatuses are equal with old resourceBinding(%s/%s) AggregatedStatus, no update required.",
			binding.Namespace, binding.Name)
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: binding.Namespace, Name: binding.Name}, binding); err != nil {
			return err
		}
		binding.Status.AggregatedStatus = aggregatedStatuses
		return c.Status().Update(context.TODO(), binding)
	})
}

// AggregateClusterResourceBindingWorkStatus will collect all work statuses with current ClusterResourceBinding objects,
// then aggregate status info to current ClusterResourceBinding status.
func AggregateClusterResourceBindingWorkStatus(c client.Client, binding *workv1alpha2.ClusterResourceBinding, workload *unstructured.Unstructured) error {
	var targetWorks []workv1alpha1.Work

	// TODO: Delete this logic in the next release to prevent incompatibility when upgrading the current release (v0.10.0).
	workList, err := GetWorksByLabelSelector(c, labels.SelectorFromSet(
		labels.Set{
			workv1alpha2.ClusterResourceBindingLabel: binding.Name,
		},
	))
	if err != nil {
		klog.Errorf("Failed to get works by ClusterResourceBinding(%s): %v", binding.Name, err)
		return err
	}
	targetWorks = append(targetWorks, workList.Items...)

	workList, err = GetWorksByLabelSelector(c, labels.SelectorFromSet(
		labels.Set{
			workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", binding.Name),
		},
	))
	if err != nil {
		klog.Errorf("Failed to get works by ClusterResourceBinding(%s): %v", binding.Name, err)
		return err
	}
	targetWorks = append(targetWorks, workList.Items...)

	aggregatedStatuses, err := assembleWorkStatus(targetWorks, workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.Infof("New aggregatedStatuses are equal with old clusterResourceBinding(%s) AggregatedStatus, no update required.", binding.Name)
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err = c.Get(context.TODO(), client.ObjectKey{Name: binding.Name}, binding); err != nil {
			return err
		}
		binding.Status.AggregatedStatus = aggregatedStatuses
		return c.Status().Update(context.TODO(), binding)
	})
}

// assemble workStatuses from workList which list by selector and match with workload.
func assembleWorkStatus(works []workv1alpha1.Work, workload *unstructured.Unstructured) ([]workv1alpha2.AggregatedStatusItem, error) {
	statuses := make([]workv1alpha2.AggregatedStatusItem, 0)
	for _, work := range works {
		identifierIndex, err := GetManifestIndex(work.Spec.Workload.Manifests, workload)
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
			}
			statuses = append(statuses, aggregatedStatus)
			return statuses, nil
		}

		for _, manifestStatus := range work.Status.ManifestStatuses {
			equal, err := equalIdentifier(&manifestStatus.Identifier, identifierIndex, workload)
			if err != nil {
				return nil, err
			}
			if equal {
				aggregatedStatus := workv1alpha2.AggregatedStatusItem{
					ClusterName: clusterName,
					Status:      manifestStatus.Status,
					Applied:     applied,
				}
				statuses = append(statuses, aggregatedStatus)
				break
			}
		}
	}

	return statuses, nil
}

// GetManifestIndex get the index of clusterObj in manifest list, if not exist return -1.
func GetManifestIndex(manifests []workv1alpha1.Manifest, clusterObj *unstructured.Unstructured) (int, error) {
	for index, rawManifest := range manifests {
		manifest := &unstructured.Unstructured{}
		if err := manifest.UnmarshalJSON(rawManifest.Raw); err != nil {
			return -1, err
		}

		if manifest.GetAPIVersion() == clusterObj.GetAPIVersion() &&
			manifest.GetKind() == clusterObj.GetKind() &&
			manifest.GetNamespace() == clusterObj.GetNamespace() &&
			manifest.GetName() == clusterObj.GetName() {
			return index, nil
		}
	}

	return -1, fmt.Errorf("no such manifest exist")
}

func equalIdentifier(targetIdentifier *workv1alpha1.ResourceIdentifier, ordinal int, workload *unstructured.Unstructured) (bool, error) {
	groupVersion, err := schema.ParseGroupVersion(workload.GetAPIVersion())
	if err != nil {
		return false, err
	}

	if targetIdentifier.Ordinal == ordinal &&
		targetIdentifier.Group == groupVersion.Group &&
		targetIdentifier.Version == groupVersion.Version &&
		targetIdentifier.Kind == workload.GetKind() &&
		targetIdentifier.Namespace == workload.GetNamespace() &&
		targetIdentifier.Name == workload.GetName() {
		return true, nil
	}

	return false, nil
}

// IsResourceApplied checks whether resource has been dispatched to member cluster or not
func IsResourceApplied(workStatus *workv1alpha1.WorkStatus) bool {
	return meta.IsStatusConditionTrue(workStatus.Conditions, workv1alpha1.WorkApplied)
}

// IsWorkContains checks if the target resource exists in a work.
// Note: This function checks the Work object's status to detect the target resource, so the Work should be 'Applied',
// otherwise always returns false.
func IsWorkContains(workStatus *workv1alpha1.WorkStatus, targetResource schema.GroupVersionKind) bool {
	for _, manifestStatuses := range workStatus.ManifestStatuses {
		if targetResource.Group == manifestStatuses.Identifier.Group &&
			targetResource.Version == manifestStatuses.Identifier.Version &&
			targetResource.Kind == manifestStatuses.Identifier.Kind {
			return true
		}
	}
	return false
}
