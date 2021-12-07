package helper

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exist.
func CreateOrUpdateWork(client client.Client, workMeta metav1.ObjectMeta, resource *unstructured.Unstructured) error {
	workload := resource.DeepCopy()
	workloadJSON, err := workload.MarshalJSON()
	if err != nil {
		klog.Errorf("Failed to marshal workload(%s/%s), Error: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	work := &workv1alpha1.Work{
		ObjectMeta: workMeta,
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: workloadJSON,
						},
					},
				},
			},
		},
	}

	runtimeObject := work.DeepCopy()
	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), client, runtimeObject, func() error {
		runtimeObject.Spec = work.Spec
		runtimeObject.Labels = work.Labels
		runtimeObject.Annotations = work.Annotations
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.V(2).Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.V(2).Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else {
		klog.V(2).Infof("Work %s/%s is up to date.", work.GetNamespace(), work.GetName())
	}

	return nil
}

// GetWorksByLabelSelector get WorkList by matching label selector.
func GetWorksByLabelSelector(c client.Client, selector labels.Selector) (*workv1alpha1.WorkList, error) {
	workList := &workv1alpha1.WorkList{}
	err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	return workList, nil
}
