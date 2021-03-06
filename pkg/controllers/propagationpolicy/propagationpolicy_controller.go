package propagationpolicy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "propagation-policy-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("PropagationPolicy")

// Controller is to sync PropagationPolicy.
type Controller struct {
	client.Client                   // used to operate PropagationPolicy resources.
	DynamicClient dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationPolicy %s.", req.NamespacedName.String())

	policy := &v1alpha1.PropagationPolicy{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, policy); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to ResourceBinding.
		// ResourceBinding will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	present, err := c.allDependentOverridesPresent(policy)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	if !present {
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("the specific overrides which current PropagationPolicy %v/%v rely on are not all present", policy.GetNamespace(), policy.GetName())
	}

	result, err := c.syncPolicy(policy)
	if err != nil {
		return result, err
	}

	return controllerruntime.Result{}, nil
}

// syncPolicy will fetch matched resource by policy, then transform them to ResourceBinding objects.
func (c *Controller) syncPolicy(policy *v1alpha1.PropagationPolicy) (controllerruntime.Result, error) {
	workloads, err := c.fetchWorkloads(policy)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	// TODO(RainbowMango): Need to report an event for no resource policy that may be a mistake.
	// Ignore the workloads that owns by other policy and can not be shared.
	policyReferenceWorkloads := c.ignoreIrrelevantWorkload(policy, workloads)
	// Claim the rest workloads that should be owned by current policy.
	err = c.claimResources(policy.GetNamespace(), policy.GetName(), policyReferenceWorkloads)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	// TODO: Remove annotation of workloads owned by current policy, but should not be owned now.
	return c.buildResourceBinding(policy, policyReferenceWorkloads)
}

// fetchWorkloads fetches all matched resources via resource selectors.
func (c *Controller) fetchWorkloads(policy *v1alpha1.PropagationPolicy) ([]*unstructured.Unstructured, error) {
	var workloads []*unstructured.Unstructured

	for _, resourceSelector := range policy.Spec.ResourceSelectors {
		if resourceSelector.Namespace == "" {
			resourceSelector.Namespace = policy.Namespace
		}
		tmpWorkloads, err := c.fetchWorkloadsByResourceSelector(resourceSelector)
		if err != nil {
			klog.Errorf("Failed to fetch workloads with labelSelector in namespace %s. Error: %v.", policy.Namespace, err)
			return nil, err
		}
		workloads = append(workloads, tmpWorkloads...)
	}
	return workloads, nil
}

// deleteResourceBinding will delete ResourceBinding.
func (c *Controller) deleteResourceBinding(binding workv1alpha1.ResourceBinding) error {
	err := c.Client.Delete(context.TODO(), &binding)
	if err != nil && errors.IsNotFound(err) {
		klog.Infof("ResourceBinding %s/%s is already not exist.", binding.GetNamespace(), binding.GetName())
		return nil
	} else if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("Failed to delete ResourceBinding %s/%s. Error: %v.", binding.GetNamespace(), binding.GetName(), err)
		return err
	}
	klog.Infof("Delete ResourceBinding %s/%s successfully.", binding.GetNamespace(), binding.GetName())
	return nil
}

// calculateResourceBindings will get orphanBindings and workloads that need to update or create.
func (c *Controller) calculateResourceBindings(policy *v1alpha1.PropagationPolicy,
	workloads []*unstructured.Unstructured) ([]workv1alpha1.ResourceBinding, []*unstructured.Unstructured, error) {
	selector := labels.SelectorFromSet(labels.Set{
		util.PropagationPolicyNamespaceLabel: policy.Namespace,
		util.PropagationPolicyNameLabel:      policy.Name,
	})

	bindingList := &workv1alpha1.ResourceBindingList{}
	if err := c.Client.List(context.TODO(), bindingList, &client.ListOptions{LabelSelector: selector}); err != nil {
		klog.Errorf("Failed to list ResourceBinding in namespace %s", policy.GetNamespace())
		return nil, nil, err
	}
	var orphanBindings []workv1alpha1.ResourceBinding
	for _, binding := range bindingList.Items {
		isFind := false
		for _, workload := range workloads {
			bindingName := names.GenerateBindingName(workload.GetNamespace(), workload.GetKind(), workload.GetName())
			if binding.GetName() == bindingName {
				isFind = true
				break
			}
		}
		if !isFind {
			orphanBindings = append(orphanBindings, binding)
		}
	}
	return orphanBindings, workloads, nil
}

// buildResourceBinding will build ResourceBinding by matched resources.
func (c *Controller) buildResourceBinding(policy *v1alpha1.PropagationPolicy, policyReferenceWorkloads []*unstructured.Unstructured) (controllerruntime.Result, error) {
	orphanBindings, workloads, err := c.calculateResourceBindings(policy, policyReferenceWorkloads)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	// Remove orphan bindings.
	for _, binding := range orphanBindings {
		err := c.deleteResourceBinding(binding)
		if err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
	}

	// If binding already exist, update if changed.
	// If binding not exist, create it.
	for _, workload := range workloads {
		err := c.ensureResourceBinding(policy, workload)
		if err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
	}

	return controllerruntime.Result{}, nil
}

// claimResources will set ownerLabel in resource that associate with policy.
func (c *Controller) claimResources(policyNamespace string, policyName string, workloads []*unstructured.Unstructured) error {
	for _, workload := range workloads {
		dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper,
			schema.FromAPIVersionAndKind(workload.GetAPIVersion(), workload.GetKind()))
		if err != nil {
			klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", workload.GetAPIVersion(), workload.GetKind(), err)
			return err
		}

		util.MergeLabel(workload, util.PropagationPolicyNamespaceLabel, policyNamespace)
		util.MergeLabel(workload, util.PropagationPolicyNameLabel, policyName)

		_, err = c.DynamicClient.Resource(dynamicResource).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to update resource, kind: %s, namespace: %s, name: %s. Error: %v.", workload.GetKind(),
				workload.GetNamespace(), workload.GetName(), err)
			return err
		}
		klog.V(1).Infof("Update resource successfully, kind: %s, namespace: %s, name: %s.", workload.GetKind(),
			workload.GetNamespace(), workload.GetName())
	}
	return nil
}

// ignoreIrrelevantWorkload will ignore the workloads that owns by other policy and can not be shared.
func (c *Controller) ignoreIrrelevantWorkload(policy *v1alpha1.PropagationPolicy, workloads []*unstructured.Unstructured) []*unstructured.Unstructured {
	var result []*unstructured.Unstructured

	for _, workload := range workloads {
		workloadLabels := workload.GetLabels()

		// ignore objects that already associated with a PropagationPolicy.
		if policyName, exist := workloadLabels[util.PropagationPolicyNameLabel]; exist {
			// check if associated PropagationPolicy is current policy.
			// Don't need to check namespace here.
			if policyName != policy.Name {
				continue
			}
		}

		// ignore objects that already associated with a ClusterPropagationPolicy.
		if _, exist := workloadLabels[util.ClusterPropagationPolicyLabel]; exist {
			continue
		}

		result = append(result, workload)
	}
	return result
}

// fetchWorkloadsByResourceSelector query workloads by labelSelector and names
func (c *Controller) fetchWorkloadsByResourceSelector(resourceSelector v1alpha1.ResourceSelector) ([]*unstructured.Unstructured, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper,
		schema.FromAPIVersionAndKind(resourceSelector.APIVersion, resourceSelector.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resourceSelector.APIVersion, resourceSelector.Kind, err)
		return nil, err
	}

	if resourceSelector.Name != "" {
		workload, err := c.fetchWorkload(resourceSelector)
		if err != nil || workload == nil {
			return nil, err
		}

		return []*unstructured.Unstructured{workload}, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(resourceSelector.LabelSelector)
	if err != nil {
		return nil, err
	}
	unstructuredWorkLoadList, err := c.DynamicClient.Resource(dynamicResource).Namespace(resourceSelector.Namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	var workloads []*unstructured.Unstructured
	for _, unstructuredWorkLoad := range unstructuredWorkLoadList.Items {
		if unstructuredWorkLoad.GetDeletionTimestamp() == nil {
			workloads = append(workloads, &unstructuredWorkLoad)
		}
	}
	return workloads, nil
}

func (c *Controller) fetchWorkload(resourceSelector v1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper,
		schema.FromAPIVersionAndKind(resourceSelector.APIVersion, resourceSelector.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resourceSelector.APIVersion,
			resourceSelector.Kind, err)
		return nil, err
	}
	workload, err := c.DynamicClient.Resource(dynamicResource).Namespace(resourceSelector.Namespace).Get(context.TODO(), resourceSelector.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warningf("Workload does not exist, kind: %s, namespace: %s, name: %s",
				resourceSelector.Kind, resourceSelector.Namespace, resourceSelector.Name)
			return nil, nil
		}

		klog.Errorf("Failed to get workload, kind: %s, namespace: %s, name: %s. Error: %v",
			resourceSelector.Kind, resourceSelector.Namespace, resourceSelector.Name, err)
		return nil, err
	}
	if workload.GetDeletionTimestamp() != nil {
		return nil, nil
	}
	return workload, nil
}

// ensureResourceBinding will ensure ResourceBindings are created or updated.
func (c *Controller) ensureResourceBinding(policy *v1alpha1.PropagationPolicy, workload *unstructured.Unstructured) error {
	bindingName := names.GenerateBindingName(workload.GetNamespace(), workload.GetKind(), workload.GetName())
	binding := &workv1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: policy.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(policy, controllerKind),
			},
			Labels: map[string]string{
				util.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
				util.PropagationPolicyNameLabel:      policy.GetName(),
			},
		},
		Spec: workv1alpha1.ResourceBindingSpec{
			Resource: workv1alpha1.ObjectReference{
				APIVersion:      workload.GetAPIVersion(),
				Kind:            workload.GetKind(),
				Namespace:       workload.GetNamespace(),
				Name:            workload.GetName(),
				ResourceVersion: workload.GetResourceVersion(),
			},
		},
	}
	err := c.Client.Create(context.TODO(), binding)
	if err == nil {
		klog.Infof("Create binding %s/%s successfully.", binding.GetNamespace(), binding.GetName())
		return nil
	}
	if errors.IsAlreadyExists(err) {
		klog.V(2).Infof("ResourceBinding %s/%s is up to date.", binding.GetNamespace(), binding.GetName())
		return nil
	}
	klog.Errorf("Failed to create binding %s/%s. Error: %v", binding.GetNamespace(), binding.GetName(), err)
	return err
}

// allDependentOverridesPresent will ensure the specify overrides which current PropagationPolicy rely on are present
func (c *Controller) allDependentOverridesPresent(policy *v1alpha1.PropagationPolicy) (bool, error) {
	for _, override := range policy.Spec.DependentOverrides {
		overrideObj := &v1alpha1.OverridePolicy{}
		if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: policy.Namespace, Name: override}, overrideObj); err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("The specific override policy %v/%v which current PropagationPolicy %v/%v rely on is not present", policy.Namespace, override, policy.Namespace, policy.Name)
				return false, nil
			}
			klog.Errorf("Failed to get override policy %v/%v, Error: %v", policy.Namespace, override, err)
			return false, err
		}
	}

	return true, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationPolicy{}).Complete(c)
}
