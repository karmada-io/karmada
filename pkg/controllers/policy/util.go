package policy

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

func resourceSelectorToGVK(resourceSelector policyv1alpha1.ResourceSelector) (schema.GroupVersionKind, error) {
	apiVersion := resourceSelector.APIVersion
	parts := strings.Split(apiVersion, "/")
	if len(parts) != 2 {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid apiVersion %s", apiVersion)
	}
	return schema.GroupVersionKind{
		Group:   parts[0],
		Version: parts[1],
		Kind:    resourceSelector.Kind,
	}, nil
}

func cleanupLabels(dynamicClient dynamic.Interface, informerManager informermanager.SingleClusterInformerManager,
	restMapper meta.RESTMapper, objRef workv1alpha2.ObjectReference, labels ...string) error {
	workload, err := helper.FetchWorkload(dynamicClient, informerManager, restMapper, objRef)
	if err != nil {
		// do nothing if resource template not exist, it might has been removed.
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to fetch resource(kind=%s, %s/%s): %v", objRef.Kind, objRef.Namespace, objRef.Name, err)
		return err
	}

	workloadLabels := workload.GetLabels()
	for _, l := range labels {
		delete(workloadLabels, l)
	}
	workload.SetLabels(workloadLabels)

	gvr, err := restmapper.GetGroupVersionResource(restMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to delete resource(%s/%s) labels as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	newWorkload, err := dynamicClient.Resource(gvr).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update resource %v/%v, err is %v ", workload.GetNamespace(), workload.GetName(), err)
		return err
	}
	klog.V(2).Infof("Updated resource template(kind=%s, %s/%s) successfully", newWorkload.GetKind(), newWorkload.GetNamespace(), newWorkload.GetName())
	return nil
}
