package applyer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// Builder is the builder for ResourceBinding and ClusterResourceBinding
type Builder struct {
	object              *unstructured.Unstructured
	objectKey           keys.ClusterWideKey
	labels              map[string]string
	propagateDeps       bool
	resourceInterpreter resourceinterpreter.ResourceInterpreter
}

func newBuilder(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, labels map[string]string, propagateDeps bool, resourceInterpreter resourceinterpreter.ResourceInterpreter) *Builder {
	return &Builder{
		object:              object,
		objectKey:           objectKey,
		labels:              labels,
		propagateDeps:       propagateDeps,
		resourceInterpreter: resourceInterpreter,
	}
}

// buildResourceBinding builds a desired ResourceBinding for object.
func (b *Builder) buildResourceBinding() (*workv1alpha2.ResourceBinding, error) {
	bindingName := names.GenerateBindingName(b.object.GetKind(), b.object.GetName())
	propagationBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: b.object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b.object, b.objectKey.GroupVersionKind()),
			},
			Labels:     b.labels,
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			PropagateDeps: b.propagateDeps,
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      b.object.GetAPIVersion(),
				Kind:            b.object.GetKind(),
				Namespace:       b.object.GetNamespace(),
				Name:            b.object.GetName(),
				UID:             b.object.GetUID(),
				ResourceVersion: b.object.GetResourceVersion(),
			},
		},
	}

	if b.resourceInterpreter.HookEnabled(b.object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretReplica) {
		replicas, replicaRequirements, err := b.resourceInterpreter.GetReplicas(b.object)
		if err != nil {
			klog.Errorf("Failed to customize replicas for %s(%s), %v", b.object.GroupVersionKind(), b.object.GetName(), err)
			return nil, err
		}
		propagationBinding.Spec.Replicas = replicas
		propagationBinding.Spec.ReplicaRequirements = replicaRequirements
	}

	return propagationBinding, nil
}

// buildClusterResourceBinding builds a desired ClusterResourceBinding for object.
func (b *Builder) buildClusterResourceBinding() (*workv1alpha2.ClusterResourceBinding, error) {
	bindingName := names.GenerateBindingName(b.object.GetKind(), b.object.GetName())
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b.object, b.objectKey.GroupVersionKind()),
			},
			Labels:     b.labels,
			Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			PropagateDeps: b.propagateDeps,
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      b.object.GetAPIVersion(),
				Kind:            b.object.GetKind(),
				Name:            b.object.GetName(),
				UID:             b.object.GetUID(),
				ResourceVersion: b.object.GetResourceVersion(),
			},
		},
	}

	if b.resourceInterpreter.HookEnabled(b.object.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretReplica) {
		replicas, replicaRequirements, err := b.resourceInterpreter.GetReplicas(b.object)
		if err != nil {
			klog.Errorf("Failed to customize replicas for %s(%s), %v", b.object.GroupVersionKind(), b.object.GetName(), err)
			return nil, err
		}
		binding.Spec.Replicas = replicas
		binding.Spec.ReplicaRequirements = replicaRequirements
	}

	return binding, nil
}
