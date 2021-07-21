package helper

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// ConvertToPropagationPolicy converts a PropagationPolicy object from unstructured to typed.
func ConvertToPropagationPolicy(obj *unstructured.Unstructured) (*policyv1alpha1.PropagationPolicy, error) {
	typedObj := &policyv1alpha1.PropagationPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToClusterPropagationPolicy converts a ClusterPropagationPolicy object from unstructured to typed.
func ConvertToClusterPropagationPolicy(obj *unstructured.Unstructured) (*policyv1alpha1.ClusterPropagationPolicy, error) {
	typedObj := &policyv1alpha1.ClusterPropagationPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToResourceBinding converts a ResourceBinding object from unstructured to typed.
func ConvertToResourceBinding(obj *unstructured.Unstructured) (*workv1alpha1.ResourceBinding, error) {
	typedObj := &workv1alpha1.ResourceBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}
