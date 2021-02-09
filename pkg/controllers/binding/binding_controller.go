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
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
	client.Client                     // used to operate PropagationBinding resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
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
		// Do nothing, just return as we have added owner reference to Work.
		// Work will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	isReady := c.isBindingReady(binding)
	if !isReady {
		klog.Infof("propagationBinding %s/%s is not ready to sync", binding.GetNamespace(), binding.GetName())
		return controllerruntime.Result{Requeue: true}, nil
	}

	return c.syncBinding(binding)
}

// isBindingReady will check if propagationBinding is ready to build Work.
func (c *PropagationBindingController) isBindingReady(binding *v1alpha1.PropagationBinding) bool {
	return len(binding.Spec.Clusters) != 0
}

// syncBinding will sync propagationBinding to Works.
func (c *PropagationBindingController) syncBinding(binding *v1alpha1.PropagationBinding) (controllerruntime.Result, error) {
	clusterNames := c.getBindingClusterNames(binding)
	ownerLabel := names.GenerateOwnerLabelValue(binding.GetNamespace(), binding.GetName())
	works, err := c.findOrphanWorks(ownerLabel, clusterNames)
	if err != nil {
		klog.Errorf("Failed to find orphan works by propagationBinding %s/%s. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = c.removeOrphanWorks(works)
	if err != nil {
		klog.Errorf("Failed to remove orphan works by propagationBinding %s/%s. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	err = c.transformBindingToWorks(binding, clusterNames)
	if err != nil {
		klog.Errorf("Failed to transform propagationBinding %s/%s to works. Error: %v.",
			binding.GetNamespace(), binding.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// removeOrphanBindings will remove orphan works.
func (c *PropagationBindingController) removeOrphanWorks(works []v1alpha1.Work) error {
	for _, work := range works {
		err := c.Client.Delete(context.TODO(), &work)
		if err != nil {
			return err
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// findOrphanWorks will find orphan works that don't match current propagationBinding clusters.
func (c *PropagationBindingController) findOrphanWorks(ownerLabel string, clusterNames []string) ([]v1alpha1.Work, error) {
	labelRequirement, err := labels.NewRequirement(util.OwnerLabel, selection.Equals, []string{ownerLabel})
	if err != nil {
		klog.Errorf("Failed to new a requirement. Error: %v", err)
		return nil, err
	}
	selector := labels.NewSelector().Add(*labelRequirement)
	workList := &v1alpha1.WorkList{}
	if err := c.Client.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}
	var orphanWorks []v1alpha1.Work
	expectClusters := sets.NewString(clusterNames...)
	for _, work := range workList.Items {
		workTargetCluster, err := names.GetClusterName(work.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get cluster name which Work %s/%s belongs to. Error: %v.",
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

// transformBindingToWorks will transform propagationBinding to Works
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

	err = c.ensureWork(workload, clusterNames, binding)
	if err != nil {
		return err
	}
	return nil
}

// ensureWork ensure Work to be created or updated
func (c *PropagationBindingController) ensureWork(workload *unstructured.Unstructured, clusterNames []string,
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
			klog.Errorf("Failed to ensure Work for cluster: %s. Error: %v.", clusterName, err)
			return err
		}

		work := &v1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:       binding.Name,
				Namespace:  executionSpace,
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(binding, controllerKind),
				},
				Labels: map[string]string{util.OwnerLabel: names.GenerateOwnerLabelValue(binding.GetNamespace(), binding.GetName())},
			},
			Spec: v1alpha1.WorkSpec{
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

		runtimeObject := work.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), c.Client, runtimeObject, func() error {
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
	}
	return nil
}
