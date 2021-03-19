package helper

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// AggregateResourceBindingWorkStatus will collect all work statuses with current ResourceBinding objects,
// then aggregate status info to current ResourceBinding status.
func AggregateResourceBindingWorkStatus(c client.Client, binding *workv1alpha1.ResourceBinding, workload *unstructured.Unstructured) error {
	aggregatedStatuses, err := assembleWorkStatus(c, labels.SelectorFromSet(labels.Set{
		util.ResourceBindingNamespaceLabel: binding.Namespace,
		util.ResourceBindingNameLabel:      binding.Name,
	}), workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.V(4).Infof("New aggregatedStatuses are equal with old resourceBinding(%s/%s) AggregatedStatus, no update required.",
			binding.Namespace, binding.Name)
		return nil
	}

	binding.Status.AggregatedStatus = aggregatedStatuses
	err = c.Status().Update(context.TODO(), binding)
	if err != nil {
		klog.Errorf("Failed update resourceBinding(%s/%s). Error: %v.", binding.Namespace, binding.Name, err)
		return err
	}
	klog.Infof("Update resourceBinding(%s/%s) with AggregatedStatus successfully.", binding.Namespace, binding.Name)

	return nil
}

// AggregateClusterResourceBindingWorkStatus will collect all work statuses with current ClusterResourceBinding objects,
// then aggregate status info to current ClusterResourceBinding status.
func AggregateClusterResourceBindingWorkStatus(c client.Client, binding *workv1alpha1.ClusterResourceBinding, workload *unstructured.Unstructured) error {
	aggregatedStatuses, err := assembleWorkStatus(c, labels.SelectorFromSet(labels.Set{
		util.ClusterResourceBindingLabel: binding.Name,
	}), workload)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(binding.Status.AggregatedStatus, aggregatedStatuses) {
		klog.Infof("New aggregatedStatuses are equal with old clusterResourceBinding(%s) AggregatedStatus, no update required.", binding.Name)
		return nil
	}

	binding.Status.AggregatedStatus = aggregatedStatuses
	err = c.Status().Update(context.TODO(), binding)
	if err != nil {
		klog.Errorf("Failed update clusterResourceBinding(%s). Error: %v.", binding.Name, err)
		return err
	}
	klog.Infof("Update clusterResourceBinding(%s) with AggregatedStatus successfully.", binding.Name)

	return nil
}

// assemble workStatuses from workList which list by selector and match with workload.
func assembleWorkStatus(c client.Client, selector labels.Selector, workload *unstructured.Unstructured) ([]workv1alpha1.AggregatedStatusItem, error) {
	workList := &workv1alpha1.WorkList{}
	if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}

	statuses := make([]workv1alpha1.AggregatedStatusItem, 0)
	for _, work := range workList.Items {
		identifierIndex, err := GetManifestIndex(work.Spec.Workload.Manifests, workload)
		if err != nil {
			klog.Errorf("Failed to get manifestIndex of workload in work.Spec.Workload.Manifests. Error: %v.", err)
			return nil, err
		}

		for _, manifestStatus := range work.Status.ManifestStatuses {
			equal, err := equalIdentifier(&manifestStatus.Identifier, identifierIndex, workload)
			if err != nil {
				return nil, err
			}
			if equal {
				clusterName, err := names.GetClusterName(work.Namespace)
				if err != nil {
					klog.Errorf("Failed to get clusterName from work namespace %s. Error: %v.", work.Namespace, err)
					return nil, err
				}

				aggregatedStatus := workv1alpha1.AggregatedStatusItem{
					ClusterName: clusterName,
					Status:      manifestStatus.Status,
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
