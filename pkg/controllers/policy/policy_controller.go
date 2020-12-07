package policy

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	karmadaclientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "policy-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("PropagationPolicy")

// PropagationPolicyController is to sync PropagationPolicy.
type PropagationPolicyController struct {
	client.Client                            // used to operate PropagationPolicy resources.
	DynamicClient dynamic.Interface          // used to fetch arbitrary resources.
	KarmadaClient karmadaclientset.Interface // used to create/update PropagationBinding resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *PropagationPolicyController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationPolicy %s", req.NamespacedName.String())

	policy := &v1alpha1.PropagationPolicy{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, policy); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to PropagationBinding.
		// PropagationBinding will be removed automatically by garbage collector.
		return controllerruntime.Result{}, nil
	}

	return c.syncPolicy(policy)
}

// syncPolicy will fetch matched resource by policy, then transform them to propagationBindings
func (c *PropagationPolicyController) syncPolicy(policy *v1alpha1.PropagationPolicy) (controllerruntime.Result, error) {
	workloads, err := c.fetchWorkloads(policy.Spec.ResourceSelectors)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	if len(workloads) == 0 {
		klog.Infof("No resource be selected, ignore sync: %s/%s", policy.Namespace, policy.Name)
		// TODO(RainbowMango): Need to report an event for no resource policy that may be a mistake.
		return controllerruntime.Result{}, nil
	}

	// TODO(RainbowMango): Ignore the workloads that owns by other policy and can not be shared.

	// TODO(RainbowMango): Claim the rest workloads that should be owned by current policy.

	return c.buildPropagationBinding(policy, workloads)
}

// fetchWorkloads fetches all matched resources via resource selectors.
// TODO(RainbowMango): the implementation is old and too complicated, need refactor later.
func (c *PropagationPolicyController) fetchWorkloads(resourceSelectors []v1alpha1.ResourceSelector) ([]*unstructured.Unstructured, error) {
	var workloads []*unstructured.Unstructured
	// todo: if resources repetitive, deduplication.
	// todo: if namespaces, names, labelSelector is nil, need to do something
	for _, resourceSelector := range resourceSelectors {
		names := util.GetUniqueElements(resourceSelector.Names)
		namespaces := util.GetDifferenceSet(resourceSelector.Namespaces, resourceSelector.ExcludeNamespaces)
		for _, namespace := range namespaces {
			if resourceSelector.LabelSelector == nil {
				err := c.fetchWorkloadsWithOutLabelSelector(resourceSelector, namespace, names, &workloads)
				if err != nil {
					klog.Errorf("Failed to fetch workloads by names in namespace %s. Error: %v", namespace, err)
					return nil, err
				}
			} else {
				err := c.fetchWorkloadsWithLabelSelector(resourceSelector, namespace, names, &workloads)
				if err != nil {
					klog.Errorf("Failed to fetch workloads with labelSelector in namespace %s. Error: %v", namespace, err)
					return nil, err
				}
			}
		}
	}
	return workloads, nil
}

// buildPropagationBinding will build propagationBinding by matched resources
func (c *PropagationPolicyController) buildPropagationBinding(policy *v1alpha1.PropagationPolicy, workloads []*unstructured.Unstructured) (controllerruntime.Result, error) {
	targetCluster := c.getTargetClusters(policy.Spec.Placement)

	for _, workload := range workloads {
		err := c.ensurePropagationBinding(policy, workload, targetCluster)
		if err != nil {
			return controllerruntime.Result{Requeue: true}, err
		}
	}

	return controllerruntime.Result{}, nil
}

// fetchWorkloadsWithLabelSelector query workloads by labelSelector and names
func (c *PropagationPolicyController) fetchWorkloadsWithLabelSelector(resourceSelector v1alpha1.ResourceSelector, namespace string, names []string, workloads *[]*unstructured.Unstructured) error {
	unstructuredWorkLoadList, err := util.ListUnstructuredByFilter(c.DynamicClient, resourceSelector.APIVersion,
		resourceSelector.Kind, namespace, resourceSelector.LabelSelector)
	if err != nil {
		return err
	}
	if resourceSelector.Names == nil {
		for _, unstructuredWorkLoad := range unstructuredWorkLoadList.Items {
			*workloads = append(*workloads, &unstructuredWorkLoad)
		}
	} else {
		for _, unstructuredWorkLoad := range unstructuredWorkLoadList.Items {
			for _, name := range names {
				if unstructuredWorkLoad.GetName() == name {
					*workloads = append(*workloads, &unstructuredWorkLoad)
					break
				}
			}
		}
	}
	return nil
}

// fetchWorkloadsWithOutLabelSelector query workloads by names
func (c *PropagationPolicyController) fetchWorkloadsWithOutLabelSelector(resourceSelector v1alpha1.ResourceSelector, namespace string, names []string, workloads *[]*unstructured.Unstructured) error {
	for _, name := range names {
		workload, err := util.GetUnstructured(c.DynamicClient, resourceSelector.APIVersion,
			resourceSelector.Kind, namespace, name)
		if err != nil {
			return err
		}
		*workloads = append(*workloads, workload)
	}
	return nil
}

// getTargetClusters get targetClusters by placement
// TODO(RainbowMango): This is a dummy function and will be removed once scheduler on board.
func (c *PropagationPolicyController) getTargetClusters(placement v1alpha1.Placement) []v1alpha1.TargetCluster {
	matchClusterNames := util.GetDifferenceSet(placement.ClusterAffinity.ClusterNames, placement.ClusterAffinity.ExcludeClusters)

	// todo: cluster labelSelector, fieldSelector, clusterTolerations
	// todo: calc spread contraints. such as maximum, minimum
	var targetClusters []v1alpha1.TargetCluster
	for _, matchClusterName := range matchClusterNames {
		targetClusters = append(targetClusters, v1alpha1.TargetCluster{Name: matchClusterName})
	}
	return targetClusters
}

// ensurePropagationBinding will ensure propagationBinding are created or updated.
func (c *PropagationPolicyController) ensurePropagationBinding(propagationPolicy *v1alpha1.PropagationPolicy, workload *unstructured.Unstructured, clusterNames []v1alpha1.TargetCluster) error {
	bindingName := strings.ToLower(workload.GetNamespace() + "-" + workload.GetKind() + "-" + workload.GetName())
	propagationBinding := v1alpha1.PropagationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: propagationPolicy.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(propagationPolicy, controllerKind),
			},
		},
		Spec: v1alpha1.PropagationBindingSpec{
			Resource: v1alpha1.ObjectReference{
				APIVersion:      workload.GetAPIVersion(),
				Kind:            workload.GetKind(),
				Namespace:       workload.GetNamespace(),
				Name:            workload.GetName(),
				ResourceVersion: workload.GetResourceVersion(),
			},
			Clusters: clusterNames,
		},
	}

	bindingGetResult, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Get(context.TODO(), propagationBinding.Name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		_, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Create(context.TODO(), &propagationBinding, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("Failed to create propagationBinding %s/%s. Error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
			return err
		}
		klog.Infof("Create propagationBinding %s/%s successfully", propagationBinding.Namespace, propagationBinding.Name)
		return nil
	} else if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Failed to get propagationBinding %s/%s. Error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
		return err
	}
	bindingGetResult.Spec = propagationBinding.Spec
	bindingGetResult.ObjectMeta.OwnerReferences = propagationBinding.ObjectMeta.OwnerReferences
	_, err = c.KarmadaClient.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Update(context.TODO(), bindingGetResult, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update propagationBinding %s/%s. Error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
		return err
	}
	klog.Infof("Update propagationBinding %s/%s successfully", propagationBinding.Namespace, propagationBinding.Name)

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationPolicyController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationPolicy{}).Complete(c)
}
