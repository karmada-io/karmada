package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(binding *workv1alpha1.ResourceBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// CreateOrUpdateWork creates a Work object if not exist, or updates if it already exist.
func CreateOrUpdateWork(client client.Client, objectMeta metav1.ObjectMeta, rawExtension []byte) error {
	work := &workv1alpha1.Work{
		ObjectMeta: objectMeta,
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: rawExtension,
						},
					},
				},
			},
		},
	}

	runtimeObject := work.DeepCopy()
	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), client, runtimeObject, func() error {
		runtimeObject.Spec = work.Spec
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
	} else {
		klog.V(2).Infof("Work %s/%s is up to date.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// IsBindingReplicasChanges will check if the sum of replicas is different from the replicas of object
func IsBindingReplicasChanges(bindingSpec *workv1alpha1.ResourceBindingSpec) bool {
	replicasSum := int32(0)
	for _, targetCluster := range bindingSpec.Clusters {
		replicasSum += targetCluster.Replicas
	}
	return replicasSum != bindingSpec.Resource.Replicas
}
