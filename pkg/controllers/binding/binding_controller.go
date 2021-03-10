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

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "binding-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("ResourceBinding")

// ResourceBindingController is to sync ResourceBinding.
type ResourceBindingController struct {
	client.Client                     // used to operate ResourceBinding resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	OverrideManager overridemanager.OverrideManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ResourceBindingController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling ResourceBinding %s.", req.NamespacedName.String())

	binding := &workv1alpha1.ResourceBinding{}
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
func (c *ResourceBindingController) isBindingReady(binding *workv1alpha1.ResourceBinding) bool {
	return len(binding.Spec.Clusters) != 0
}

// syncBinding will sync propagationBinding to Works.
func (c *ResourceBindingController) syncBinding(binding *workv1alpha1.ResourceBinding) (controllerruntime.Result, error) {
	clusterNames := c.getBindingClusterNames(binding)
	works, err := c.findOrphanWorks(binding.Namespace, binding.Name, clusterNames)
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
func (c *ResourceBindingController) removeOrphanWorks(works []workv1alpha1.Work) error {
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
func (c *ResourceBindingController) findOrphanWorks(bindingNamespace string, bindingName string, clusterNames []string) ([]workv1alpha1.Work, error) {
	selector := labels.SelectorFromSet(labels.Set{
		util.ResourceBindingNamespaceLabel: bindingNamespace,
		util.ResourceBindingNameLabel:      bindingName,
	})

	workList := &workv1alpha1.WorkList{}
	if err := c.Client.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, err
	}
	var orphanWorks []workv1alpha1.Work
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
func (c *ResourceBindingController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.ResourceBinding{}).Complete(c)
}

// getBindingClusterNames will get clusterName list from bind clusters field
func (c *ResourceBindingController) getBindingClusterNames(binding *workv1alpha1.ResourceBinding) []string {
	var clusterNames []string
	for _, targetCluster := range binding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// transformBindingToWorks will transform propagationBinding to Works
func (c *ResourceBindingController) transformBindingToWorks(binding *workv1alpha1.ResourceBinding, clusterNames []string) error {
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
func (c *ResourceBindingController) ensureWork(workload *unstructured.Unstructured, clusterNames []string,
	binding *workv1alpha1.ResourceBinding) error {

	for _, clusterName := range clusterNames {
		// apply override policies
		clonedWorkload := workload.DeepCopy()
		err := c.OverrideManager.ApplyOverridePolicies(clonedWorkload, clusterName)
		if err != nil {
			klog.Errorf("failed to apply overrides for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
			return err
		}

		workNamespace, err := names.GenerateExecutionSpaceName(clusterName)
		if err != nil {
			klog.Errorf("Failed to ensure Work for cluster: %s. Error: %v.", clusterName, err)
			return err
		}
		workName := binding.Name

		util.MergeLabel(clonedWorkload, util.ResourceBindingNamespaceLabel, binding.Namespace)
		util.MergeLabel(clonedWorkload, util.ResourceBindingNameLabel, binding.Name)
		util.MergeLabel(clonedWorkload, util.WorkNamespaceLabel, workNamespace)
		util.MergeLabel(clonedWorkload, util.WorkNameLabel, workName)

		workloadJSON, err := clonedWorkload.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal workload, kind: %s, namespace: %s, name: %s. Error: %v",
				clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace(), err)
			return err
		}

		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:       workName,
				Namespace:  workNamespace,
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(binding, controllerKind),
				},
				Labels: map[string]string{
					util.ResourceBindingNamespaceLabel: binding.Namespace,
					util.ResourceBindingNameLabel:      binding.Name,
				},
			},
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
