package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
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
func ConvertToResourceBinding(obj *unstructured.Unstructured) (*workv1alpha2.ResourceBinding, error) {
	typedObj := &workv1alpha2.ResourceBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToPod converts a Pod object from unstructured to typed.
func ConvertToPod(obj *unstructured.Unstructured) (*corev1.Pod, error) {
	typedObj := &corev1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToNode converts a Node object from unstructured to typed.
func ConvertToNode(obj *unstructured.Unstructured) (*corev1.Node, error) {
	typedObj := &corev1.Node{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToReplicaSet converts a ReplicaSet object from unstructured to typed.
func ConvertToReplicaSet(obj *unstructured.Unstructured) (*appsv1.ReplicaSet, error) {
	typedObj := &appsv1.ReplicaSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToDeployment converts a Deployment object from unstructured to typed.
func ConvertToDeployment(obj *unstructured.Unstructured) (*appsv1.Deployment, error) {
	typedObj := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToDaemonSet converts a DaemonSet object from unstructured to typed.
func ConvertToDaemonSet(obj *unstructured.Unstructured) (*appsv1.DaemonSet, error) {
	typedObj := &appsv1.DaemonSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToStatefulSet converts a StatefulSet object from unstructured to typed.
func ConvertToStatefulSet(obj *unstructured.Unstructured) (*appsv1.StatefulSet, error) {
	typedObj := &appsv1.StatefulSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToJob converts a Job object from unstructured to typed.
func ConvertToJob(obj *unstructured.Unstructured) (*batchv1.Job, error) {
	typedObj := &batchv1.Job{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToEndpointSlice converts a EndpointSlice object from unstructured to typed.
func ConvertToEndpointSlice(obj *unstructured.Unstructured) (*discoveryv1.EndpointSlice, error) {
	typedObj := &discoveryv1.EndpointSlice{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToResourceExploringWebhookConfiguration converts a ResourceInterpreterWebhookConfiguration object from unstructured to typed.
func ConvertToResourceExploringWebhookConfiguration(obj *unstructured.Unstructured) (*configv1alpha1.ResourceInterpreterWebhookConfiguration, error) {
	typedObj := &configv1alpha1.ResourceInterpreterWebhookConfiguration{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ApplyReplica applies the Replica value for the specific field.
func ApplyReplica(workload *unstructured.Unstructured, desireReplica int64, field string) error {
	_, ok, err := unstructured.NestedInt64(workload.Object, util.SpecField, field)
	if err != nil {
		return err
	}
	if ok {
		err := unstructured.SetNestedField(workload.Object, desireReplica, util.SpecField, field)
		if err != nil {
			return err
		}
	}
	return nil
}
