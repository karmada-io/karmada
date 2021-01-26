package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(binding *v1alpha1.PropagationBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// CreateOrUpdatePropagationWork creates or updates propagationWork by controllerutil.CreateOrUpdate
func CreateOrUpdatePropagationWork(client client.Client, objectMeta metav1.ObjectMeta, rawExtension []byte) error {
	propagationWork := &v1alpha1.PropagationWork{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.PropagationWorkSpec{
			Workload: v1alpha1.WorkloadTemplate{
				Manifests: []v1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: rawExtension,
						},
					},
				},
			},
		},
	}

	runtimeObject := propagationWork.DeepCopy()
	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), client, runtimeObject, func() error {
		runtimeObject.Spec = propagationWork.Spec
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update propagationWork %s/%s. Error: %v", propagationWork.GetNamespace(), propagationWork.GetName(), err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.Infof("Create propagationWork %s/%s successfully.", propagationWork.GetNamespace(), propagationWork.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.Infof("Update propagationWork %s/%s successfully.", propagationWork.GetNamespace(), propagationWork.GetName())
	} else {
		klog.V(2).Infof("PropagationWork %s/%s is up to date.", propagationWork.GetNamespace(), propagationWork.GetName())
	}
	return nil
}
