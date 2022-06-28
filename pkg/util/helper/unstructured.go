package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

// ConvertToPersistentVolumeClaim converts a pvc object from unstructured to typed.
func ConvertToPersistentVolumeClaim(obj *unstructured.Unstructured) (*corev1.PersistentVolumeClaim, error) {
	typedObj := &corev1.PersistentVolumeClaim{}
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

// ConvertToDeploymentStatus converts a DeploymentStatus object from unstructured to typed.
func ConvertToDeploymentStatus(obj map[string]interface{}) (*appsv1.DeploymentStatus, error) {
	typedObj := &appsv1.DeploymentStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, typedObj); err != nil {
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

// ConvertToDaemonSetStatus converts a DaemonSetStatus object from unstructured to typed.
func ConvertToDaemonSetStatus(obj map[string]interface{}) (*appsv1.DaemonSetStatus, error) {
	typedObj := &appsv1.DaemonSetStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, typedObj); err != nil {
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

// ConvertToStatefulSetStatus converts a StatefulSetStatus object from unstructured to typed.
func ConvertToStatefulSetStatus(obj map[string]interface{}) (*appsv1.StatefulSetStatus, error) {
	typedObj := &appsv1.StatefulSetStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, typedObj); err != nil {
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

// ConvertToCronJob converts a CronJob object from unstructured to typed.
func ConvertToCronJob(obj *unstructured.Unstructured) (*batchv1.CronJob, error) {
	typedObj := &batchv1.CronJob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToJobStatus converts a JobStatus from unstructured to typed.
func ConvertToJobStatus(obj map[string]interface{}) (*batchv1.JobStatus, error) {
	typedObj := &batchv1.JobStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, typedObj); err != nil {
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

// ConvertToService converts a Service object from unstructured to typed.
func ConvertToService(obj *unstructured.Unstructured) (*corev1.Service, error) {
	typedObj := &corev1.Service{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToServiceStatus converts a ServiceStatus object from unstructured to typed.
func ConvertToServiceStatus(obj map[string]interface{}) (*corev1.ServiceStatus, error) {
	typedObj := &corev1.ServiceStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, typedObj); err != nil {
		return nil, err
	}

	return typedObj, nil
}

// ConvertToIngress converts an Ingress object from unstructured to typed.
func ConvertToIngress(obj *unstructured.Unstructured) (*networkingv1.Ingress, error) {
	typedObj := &networkingv1.Ingress{}
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

// ToUnstructured converts a typed object to an unstructured object.
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uncastObj}, nil
}
