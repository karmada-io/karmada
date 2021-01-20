package binding

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "binding-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("PropagationBinding")

// PropagationBindingController is to sync PropagationBinding.
type PropagationBindingController struct {
	client.Client                              // used to operate PropagationBinding resources.
	DynamicClient   dynamic.Interface          // used to fetch arbitrary resources.
	KarmadaClient   karmadaclientset.Interface // used to create/update PropagationWork resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	OverrideManager overridemanager.OverrideManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *PropagationBindingController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationBinding %s.", req.NamespacedName.String())

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

	isReady := c.isBindingReady(binding)
	if !isReady {
		klog.Infof("propagationBinding %s/%s is not ready to sync", binding.GetNamespace(), binding.GetName())
		return controllerruntime.Result{Requeue: true}, nil
	}

	return c.syncBinding(binding)
}

// isBindingReady will check if propagationBinding is ready to build propagationWork.
func (c *PropagationBindingController) isBindingReady(binding *v1alpha1.PropagationBinding) bool {
	return len(binding.Spec.Clusters) != 0
}

// syncBinding will sync propagationBinding to propagationWorks.
func (c *PropagationBindingController) syncBinding(binding *v1alpha1.PropagationBinding) (controllerruntime.Result, error) {
	clusterNames := c.getBindingClusterNames(binding)
	ownerLabel := names.GenerateOwnerLabelValue(binding.GetNamespace(), binding.GetName())
	works, err := c.findOrphanWorks(ownerLabel, clusterNames)
	if err != nil {
		klog.Errorf("Failed to find orphan propagationWorks by propagationBinding %s/%s. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = c.removeOrphanWorks(works)
	if err != nil {
		klog.Errorf("Failed to remove orphan propagationWorks by propagationBinding %s/%s. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = c.transformBindingToWorks(binding, clusterNames)
	if err != nil {
		klog.Errorf("Failed to transform propagationBinding %s/%s to propagationWorks. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// removeOrphanBindings will remove orphan propagationWorks.
func (c *PropagationBindingController) removeOrphanWorks(works []v1alpha1.PropagationWork) error {
	for _, work := range works {
		err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(work.GetNamespace()).Delete(context.TODO(), work.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Delete orphan propagationWork %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// findOrphanWorks will find orphan propagationWorks that don't match current propagationBinding clusters.
func (c *PropagationBindingController) findOrphanWorks(ownerLabel string, clusterNames []string) ([]v1alpha1.PropagationWork, error) {
	ownerLabelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{util.OwnerLabel: ownerLabel},
	}
	propagationWorkList, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(metav1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(ownerLabelSelector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	var orphanWorks []v1alpha1.PropagationWork
	expectClusters := sets.NewString(clusterNames...)
	for _, work := range propagationWorkList.Items {
		workTargetCluster, err := names.GetMemberClusterName(work.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get cluster name which PropagationWork %s/%s belongs to. Error: %v.",
				work.GetNamespace(), work.GetName(), err)
			return nil, err
		}
		if !expectClusters.Has(workTargetCluster) {
			orphanWorks = append(orphanWorks, work)
		}
	}
	return orphanWorks, nil
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
func (c *PropagationBindingController) transformBindingToWorks(binding *v1alpha1.PropagationBinding, clusterNames []string) error {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper,
		schema.FromAPIVersionAndKind(binding.Spec.Resource.APIVersion, binding.Spec.Resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", binding.Spec.Resource.APIVersion,
			binding.Spec.Resource.Kind, err)
		return err
	}
	workload, err := c.DynamicClient.Resource(dynamicResource).Namespace(binding.Spec.Resource.Namespace).Get(context.TODO(),
		binding.Spec.Resource.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get workload, kind: %s, namespace: %s, name: %s. Error: %v",
			binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, err)
		return err
	}

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

	for _, clusterName := range clusterNames {
		// apply override policies
		clonedWorkload := workload.DeepCopy()
		err := c.OverrideManager.ApplyOverridePolicies(clonedWorkload, clusterName)
		if err != nil {
			klog.Errorf("failed to apply overrides for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
			return err
		}

		workloadJSON, err := clonedWorkload.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal workload, kind: %s, namespace: %s, name: %s. Error: %v",
				clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace(), err)
			return err
		}

		executionSpace, err := names.GenerateExecutionSpaceName(clusterName)
		if err != nil {
			klog.Errorf("Failed to ensure PropagationWork for cluster: %s. Error: %v.", clusterName, err)
			return err
		}

		propagationWork := &v1alpha1.PropagationWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:       binding.Name,
				Namespace:  executionSpace,
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(binding, controllerKind),
				},
				Labels: map[string]string{util.OwnerLabel: names.GenerateOwnerLabelValue(binding.GetNamespace(), binding.GetName())},
			},
			Spec: v1alpha1.PropagationWorkSpec{
				Workload: v1alpha1.WorkloadTemplate{
					Manifests: []v1alpha1.Manifest{
						{
							RawExtension: runtime.RawExtension{
								Raw: workloadJSON,
							},
						},
					},
				},
			},
		}

		runtimeObject := propagationWork.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), c.Client, runtimeObject, func() error {
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
	}
	return nil
}
