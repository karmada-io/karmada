package binding

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	karmadaclientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
	"github.com/huawei-cloudnative/karmada/pkg/util/names"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "binding-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("PropagationBinding")

// PropagationBindingController is to sync PropagationBinding.
type PropagationBindingController struct {
	client.Client                            // used to operate PropagationBinding resources.
	DynamicClient dynamic.Interface          // used to fetch arbitrary resources.
	KarmadaClient karmadaclientset.Interface // used to create/update PropagationWork resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *PropagationBindingController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationBinding %s", req.NamespacedName.String())

	binding := &v1alpha1.PropagationBinding{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, binding); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !binding.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to PropagationWork.
		// PropagationWork will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	return c.syncBinding(binding)
}

// syncBinding will sync propagationBinding to propagationWorks
func (c *PropagationBindingController) syncBinding(binding *v1alpha1.PropagationBinding) (controllerruntime.Result, error) {
	err := c.transformBindingToWorks(binding)
	if err != nil {
		klog.Errorf("Failed to transform propagationBinding %s/%s to propagationWorks. Error: %+v",
			binding.Namespace, binding.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationBinding{}).Complete(c)
}

// getBindingClusterNames will get clusterName list from bind clusters field
func (c *PropagationBindingController) getBindingClusterNames(binding *v1alpha1.PropagationBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// removeIrrelevantField will delete irrelevant field from workload. such as uid, timestamp, status
func (c *PropagationBindingController) removeIrrelevantField(workload *unstructured.Unstructured) {
	unstructured.RemoveNestedField(workload.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(workload.Object, "metadata", "generation")
	unstructured.RemoveNestedField(workload.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(workload.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(workload.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(workload.Object, "metadata", "uid")
	unstructured.RemoveNestedField(workload.Object, "status")
}

// transformBindingToWorks will transform propagationBinding to propagationWorks
func (c *PropagationBindingController) transformBindingToWorks(binding *v1alpha1.PropagationBinding) error {
	workload, err := util.GetUnstructured(c.DynamicClient, binding.Spec.Resource.APIVersion,
		binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name)
	if err != nil {
		klog.Errorf("Failed to get resource, kind: %s, namespace: %s, name: %s. Error: %v",
			binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, err)
		return err
	}

	clusterNames := c.getBindingClusterNames(binding)

	err = c.ensurePropagationWork(workload, clusterNames, binding)
	if err != nil {
		return err
	}
	return nil
}

// ensurePropagationWork ensure PropagationWork to be created or updated
func (c *PropagationBindingController) ensurePropagationWork(workload *unstructured.Unstructured, clusterNames []string,
	binding *v1alpha1.PropagationBinding) error {
	c.removeIrrelevantField(workload)
	formatWorkload, err := workload.MarshalJSON()
	if err != nil {
		klog.Errorf("Failed to marshal workload, kind: %s, namespace: %s, name: %s. Error: %v",
			workload.GetKind(), workload.GetName(), workload.GetNamespace(), err)
		return err
	}
	rawExtension := runtime.RawExtension{
		Raw: formatWorkload,
	}
	propagationWork := v1alpha1.PropagationWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: binding.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(binding, controllerKind),
			},
		},
		Spec: v1alpha1.PropagationWorkSpec{
			Workload: v1alpha1.WorkloadTemplate{
				Manifests: []v1alpha1.Manifest{
					{
						RawExtension: rawExtension,
					},
				},
			},
		},
	}

	for _, clusterName := range clusterNames {
		executionSpace, err := names.GenerateExecutionSpaceName(clusterName)
		if err != nil {
			klog.Errorf("Failed to generate execution space name for propagationWork %s/%s. Error: %v", executionSpace, propagationWork.Name, err)
			return err
		}
		workGetResult, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(executionSpace).Get(context.TODO(), propagationWork.Name, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			_, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(executionSpace).Create(context.TODO(), &propagationWork, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create propagationWork %s/%s. Error: %v", executionSpace, propagationWork.Name, err)
				return err
			}
			klog.Infof("Create propagationWork %s/%s successfully", executionSpace, propagationWork.Name)
			continue
		} else if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get propagationWork %s/%s. Error: %v", executionSpace, propagationWork.Name, err)
			return err
		}
		workGetResult.Spec = propagationWork.Spec
		workGetResult.ObjectMeta.OwnerReferences = propagationWork.ObjectMeta.OwnerReferences
		_, err = c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(executionSpace).Update(context.TODO(), workGetResult, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to update propagationWork %s/%s. Error: %v", executionSpace, propagationWork.Name, err)
			return err
		}
		klog.Infof("Update propagationWork %s/%s successfully", executionSpace, propagationWork.Name)
	}
	return nil
}
