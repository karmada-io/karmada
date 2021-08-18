package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ResourceDetector is a resource watcher which watches all resources and reconcile the events.
type ResourceDetector struct {
	// DiscoveryClientSet is used to resource discovery.
	DiscoveryClientSet *discovery.DiscoveryClient
	// Client is used to retrieve objects, it is often more convenient than lister.
	Client client.Client
	// DynamicClient used to fetch arbitrary resources.
	DynamicClient                dynamic.Interface
	InformerManager              informermanager.SingleClusterInformerManager
	EventHandler                 cache.ResourceEventHandler
	Processor                    util.AsyncWorker
	SkippedResourceConfig        *util.SkippedResourceConfig
	SkippedPropagatingNamespaces map[string]struct{}
	// policyReconcileWorker maintains a rate limited queue which used to store PropagationPolicy's key and
	// a reconcile function to consume the items in queue.
	policyReconcileWorker   util.AsyncWorker
	propagationPolicyLister cache.GenericLister

	// clusterPolicyReconcileWorker maintains a rate limited queue which used to store ClusterPropagationPolicy's key and
	// a reconcile function to consume the items in queue.
	clusterPolicyReconcileWorker   util.AsyncWorker
	clusterPropagationPolicyLister cache.GenericLister

	// bindingReconcileWorker maintains a rate limited queue which used to store ResourceBinding's key and
	// a reconcile function to consume the items in queue.
	bindingReconcileWorker util.AsyncWorker
	resourceBindingLister  cache.GenericLister

	RESTMapper meta.RESTMapper

	// waitingObjects tracks of objects which haven't be propagated yet as lack of appropriate policies.
	waitingObjects map[keys.ClusterWideKey]struct{}
	// waitingLock is the lock for waitingObjects operation.
	waitingLock sync.RWMutex

	stopCh <-chan struct{}

	ManagedGroups []string
}

// Start runs the detector, never stop until stopCh closed.
func (d *ResourceDetector) Start(ctx context.Context) error {
	klog.Infof("Starting resource detector.")
	d.waitingObjects = make(map[keys.ClusterWideKey]struct{})
	d.stopCh = ctx.Done()

	// setup policy reconcile worker
	d.policyReconcileWorker = util.NewAsyncWorker("propagationPolicy reconciler", 1*time.Millisecond, ClusterWideKeyFunc, d.ReconcilePropagationPolicy)
	d.policyReconcileWorker.Run(1, d.stopCh)
	d.clusterPolicyReconcileWorker = util.NewAsyncWorker("clusterPropagationPolicy reconciler", time.Microsecond, ClusterWideKeyFunc, d.ReconcileClusterPropagationPolicy)
	d.clusterPolicyReconcileWorker.Run(1, d.stopCh)

	// watch and enqueue PropagationPolicy changes.
	propagationPolicyGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.GroupVersion.Group,
		Version:  policyv1alpha1.GroupVersion.Version,
		Resource: "propagationpolicies",
	}
	policyHandler := informermanager.NewHandlerOnEvents(d.OnPropagationPolicyAdd, d.OnPropagationPolicyUpdate, d.OnPropagationPolicyDelete)
	d.InformerManager.ForResource(propagationPolicyGVR, policyHandler)
	d.propagationPolicyLister = d.InformerManager.Lister(propagationPolicyGVR)

	// watch and enqueue ClusterPropagationPolicy changes.
	clusterPropagationPolicyGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.GroupVersion.Group,
		Version:  policyv1alpha1.GroupVersion.Version,
		Resource: "clusterpropagationpolicies",
	}
	clusterPolicyHandler := informermanager.NewHandlerOnEvents(d.OnClusterPropagationPolicyAdd, d.OnClusterPropagationPolicyUpdate, d.OnClusterPropagationPolicyDelete)
	d.InformerManager.ForResource(clusterPropagationPolicyGVR, clusterPolicyHandler)
	d.clusterPropagationPolicyLister = d.InformerManager.Lister(clusterPropagationPolicyGVR)

	// setup binding reconcile worker
	d.bindingReconcileWorker = util.NewAsyncWorker("resourceBinding reconciler", time.Microsecond, ClusterWideKeyFunc, d.ReconcileResourceBinding)
	d.bindingReconcileWorker.Run(1, d.stopCh)

	// watch and enqueue ResourceBinding changes.
	resourceBindingGVR := schema.GroupVersionResource{
		Group:    workv1alpha1.GroupVersion.Group,
		Version:  workv1alpha1.GroupVersion.Version,
		Resource: "resourcebindings",
	}
	bindingHandler := informermanager.NewHandlerOnEvents(d.OnResourceBindingAdd, d.OnResourceBindingUpdate, d.OnResourceBindingDelete)
	d.InformerManager.ForResource(resourceBindingGVR, bindingHandler)
	d.resourceBindingLister = d.InformerManager.Lister(resourceBindingGVR)

	// watch and enqueue ClusterResourceBinding changes.
	clusterResourceBindingGVR := schema.GroupVersionResource{
		Group:    workv1alpha1.GroupVersion.Group,
		Version:  workv1alpha1.GroupVersion.Version,
		Resource: "clusterresourcebindings",
	}
	clusterBindingHandler := informermanager.NewHandlerOnEvents(d.OnClusterResourceBindingAdd, d.OnClusterResourceBindingUpdate, d.OnClusterResourceBindingDelete)
	d.InformerManager.ForResource(clusterResourceBindingGVR, clusterBindingHandler)

	d.Processor.Run(1, d.stopCh)
	go d.discoverResources(30 * time.Second)

	<-d.stopCh
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// Check if our ResourceDetector implements necessary interfaces
var _ manager.Runnable = &ResourceDetector{}
var _ manager.LeaderElectionRunnable = &ResourceDetector{}

func (d *ResourceDetector) discoverResources(period time.Duration) {
	wait.Until(func() {
		newResources := GetDeletableResources(d.DiscoveryClientSet, d.ManagedGroups)
		for r := range newResources {
			if d.InformerManager.IsHandlerExist(r, d.EventHandler) {
				continue
			}
			klog.Infof("Setup informer for %s", r.String())
			d.InformerManager.ForResource(r, d.EventHandler)
		}
		d.InformerManager.Start(d.stopCh)
	}, period, d.stopCh)
}

// NeedLeaderElection implements LeaderElectionRunnable interface.
// So that the detector could run in the leader election mode.
func (d *ResourceDetector) NeedLeaderElection() bool {
	return true
}

// Reconcile performs a full reconciliation for the object referred to by the key.
// The key will be re-queued if an error is non-nil.
func (d *ResourceDetector) Reconcile(key util.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.Infof("Reconciling object: %s", clusterWideKey)

	object, err := d.GetUnstructuredObject(clusterWideKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// The resource may no longer exist, in which case we try (may not exist in waiting list) remove it from waiting list and stop processing.
			d.RemoveWaiting(clusterWideKey)

			// Once resource be deleted, the derived ResourceBinding or ClusterResourceBinding also need to be cleaned up,
			// currently we do that by setting owner reference to derived objects.
			return nil
		}
		klog.Errorf("Failed to get unstructured object(%s), error: %v", clusterWideKey, err)
		return err
	}

	// first attempts to match policy in it's namespace.
	propagationPolicy, err := d.LookForMatchedPolicy(object, clusterWideKey)
	if err != nil {
		klog.Errorf("Failed to retrieve policy for object: %s, error: %v", clusterWideKey.String(), err)
		return err
	}
	if propagationPolicy != nil {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentOverridesPresent(d.Client, propagationPolicy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", propagationPolicy.Namespace, propagationPolicy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}

		return d.ApplyPolicy(object, clusterWideKey, propagationPolicy)
	}

	// reaching here means there is no appropriate PropagationPolicy, attempts to match a ClusterPropagationPolicy.
	clusterPolicy, err := d.LookForMatchedClusterPolicy(object, clusterWideKey)
	if err != nil {
		klog.Errorf("Failed to retrieve cluster policy for object: %s, error: %v", clusterWideKey.String(), err)
		return err
	}
	if clusterPolicy != nil {
		return d.ApplyClusterPolicy(object, clusterWideKey, clusterPolicy)
	}

	// reaching here mean there is no appropriate policy for the object, put it into waiting list.
	d.AddWaiting(clusterWideKey)

	return nil
}

// EventFilter tells if an object should be take care of.
//
// All objects under Kubernetes reserved namespace should be ignored:
// - kube-system
// - kube-public
// - kube-node-lease
// All objects under Karmada reserved namespace should be ignored:
// - karmada-system
// - karmada-cluster
// - karmada-es-*
// All objects which API group defined by Karmada should be ignored:
// - cluster.karmada.io
// - policy.karmada.io
//
// The api objects listed above will be ignored by default, as we don't want users to manually input the things
// they don't care when trying to skip something else.
//
// If '--skipped-propagating-apis' which used to specific the APIs should be ignored in addition to the defaults, is set,
// the specified apis will be ignored as well.
//
// If '--skipped-propagating-namespaces' is specified, all APIs in the skipped-propagating-namespaces will be ignored.
func (d *ResourceDetector) EventFilter(obj interface{}) bool {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}

	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return false
	}

	if strings.HasPrefix(clusterWideKey.Namespace, names.KubernetesReservedNSPrefix) ||
		strings.HasPrefix(clusterWideKey.Namespace, names.KarmadaReservedNSPrefix) {
		return false
	}

	if d.SkippedResourceConfig != nil {
		if d.SkippedResourceConfig.GroupDisabled(clusterWideKey.Group) {
			klog.V(4).Infof("Skip event for %s", clusterWideKey.Group)
			return false
		}
		if d.SkippedResourceConfig.GroupVersionDisabled(clusterWideKey.GroupVersion()) {
			klog.V(4).Infof("Skip event for %s", clusterWideKey.GroupVersion())
			return false
		}
		if d.SkippedResourceConfig.GroupVersionKindDisabled(clusterWideKey.GroupVersionKind()) {
			klog.V(4).Infof("Skip event for %s", clusterWideKey.GroupVersionKind())
			return false
		}
	}
	// if SkippedPropagatingNamespaces is set, skip object events in these namespaces.
	if _, ok := d.SkippedPropagatingNamespaces[clusterWideKey.Namespace]; ok {
		return false
	}

	return true
}

// OnAdd handles object add event and push the object to queue.
func (d *ResourceDetector) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	d.Processor.EnqueueRateLimited(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnUpdate(oldObj, newObj interface{}) {
	d.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnDelete(obj interface{}) {
	d.OnAdd(obj)
}

// LookForMatchedPolicy tries to find a policy for object referenced by object key.
func (d *ResourceDetector) LookForMatchedPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) (*policyv1alpha1.PropagationPolicy, error) {
	if len(objectKey.Namespace) == 0 {
		return nil, nil
	}

	klog.V(2).Infof("attempts to match policy for resource(%s)", objectKey)
	policyList := &policyv1alpha1.PropagationPolicyList{}
	if err := d.Client.List(context.TODO(), policyList, &client.ListOptions{Namespace: objectKey.Namespace}); err != nil {
		klog.Errorf("Failed to list propagation policy: %v", err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	matchedPolicies := make([]policyv1alpha1.PropagationPolicy, 0)
	for _, policy := range policyList.Items {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedPolicies = append(matchedPolicies, policy)
		}
	}

	sort.Slice(matchedPolicies, func(i, j int) bool {
		return matchedPolicies[i].Name < matchedPolicies[j].Name
	})

	if len(matchedPolicies) == 0 {
		return nil, nil
	}
	klog.V(2).Infof("Matched policy(%s/%s) for resource(%s)", matchedPolicies[0].Namespace, matchedPolicies[0].Name, objectKey)
	return &matchedPolicies[0], nil
}

// LookForMatchedClusterPolicy tries to find a ClusterPropagationPolicy for object referenced by object key.
func (d *ResourceDetector) LookForMatchedClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) (*policyv1alpha1.ClusterPropagationPolicy, error) {
	klog.V(2).Infof("attempts to match cluster policy for resource(%s)", objectKey)
	policyList := &policyv1alpha1.ClusterPropagationPolicyList{}
	if err := d.Client.List(context.TODO(), policyList); err != nil {
		klog.Errorf("Failed to list cluster propagation policy: %v", err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	matchedClusterPolicies := make([]policyv1alpha1.ClusterPropagationPolicy, 0)
	for _, policy := range policyList.Items {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedClusterPolicies = append(matchedClusterPolicies, policy)
		}
	}

	sort.Slice(matchedClusterPolicies, func(i, j int) bool {
		return matchedClusterPolicies[i].Name < matchedClusterPolicies[j].Name
	})

	if len(matchedClusterPolicies) == 0 {
		return nil, nil
	}
	klog.V(2).Infof("Matched cluster policy(%s) for resource(%s)", matchedClusterPolicies[0].Name, objectKey)
	return &matchedClusterPolicies[0], nil
}

// ApplyPolicy starts propagate the object referenced by object key according to PropagationPolicy.
func (d *ResourceDetector) ApplyPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policy *policyv1alpha1.PropagationPolicy) error {
	klog.Infof("Applying policy(%s) for object: %s", policy.Name, objectKey)

	if err := d.ClaimPolicyForObject(object, policy.Namespace, policy.Name); err != nil {
		klog.Errorf("Failed to claim policy(%s) for object: %s", policy.Name, object)
		return err
	}

	policyLabels := map[string]string{
		util.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
		util.PropagationPolicyNameLabel:      policy.GetName(),
	}

	binding, err := d.BuildResourceBinding(object, objectKey, policyLabels)
	if err != nil {
		klog.Errorf("Failed to build resourceBinding for object: %s. error: %v", objectKey, err)
		return err
	}
	bindingCopy := binding.DeepCopy()
	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), d.Client, bindingCopy, func() error {
		// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
		bindingCopy.Labels = binding.Labels
		bindingCopy.OwnerReferences = binding.OwnerReferences
		bindingCopy.Spec.Resource = binding.Spec.Resource
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to apply policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.Infof("Create ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.Infof("Update ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else {
		klog.V(2).Infof("ResourceBinding(%s/%s) is up to date.", binding.GetNamespace(), binding.GetName())
	}

	return nil
}

// ApplyClusterPolicy starts propagate the object referenced by object key according to ClusterPropagationPolicy.
func (d *ResourceDetector) ApplyClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policy *policyv1alpha1.ClusterPropagationPolicy) error {
	klog.Infof("Applying cluster policy(%s) for object: %s", policy.Name, objectKey)

	if err := d.ClaimClusterPolicyForObject(object, policy.Name); err != nil {
		klog.Errorf("Failed to claim cluster policy(%s) for object: %s", policy.Name, object)
		return err
	}

	policyLabels := map[string]string{
		util.ClusterPropagationPolicyLabel: policy.GetName(),
	}

	// Build `ResourceBinding` or `ClusterResourceBinding` according to the resource template's scope.
	// For namespace-scoped resources, which namespace is not empty, building `ResourceBinding`.
	// For cluster-scoped resources, which namespace is empty, building `ClusterResourceBinding`.
	if object.GetNamespace() != "" {
		binding, err := d.BuildResourceBinding(object, objectKey, policyLabels)
		if err != nil {
			klog.Errorf("Failed to build resourceBinding for object: %s. error: %v", objectKey, err)
			return err
		}
		bindingCopy := binding.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), d.Client, bindingCopy, func() error {
			// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
			bindingCopy.Labels = binding.Labels
			bindingCopy.OwnerReferences = binding.OwnerReferences
			bindingCopy.Spec.Resource = binding.Spec.Resource
			return nil
		})

		if err != nil {
			klog.Errorf("Failed to apply cluster policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
			return err
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.Infof("Create ResourceBinding(%s) successfully.", binding.GetName())
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.Infof("Update ResourceBinding(%s) successfully.", binding.GetName())
		} else {
			klog.V(2).Infof("ResourceBinding(%s) is up to date.", binding.GetName())
		}
	} else {
		binding, err := d.BuildClusterResourceBinding(object, objectKey, policyLabels)
		if err != nil {
			klog.Errorf("Failed to build clusterResourceBinding for object: %s. error: %v", objectKey, err)
			return err
		}
		bindingCopy := binding.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), d.Client, bindingCopy, func() error {
			// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
			bindingCopy.Labels = binding.Labels
			bindingCopy.OwnerReferences = binding.OwnerReferences
			bindingCopy.Spec.Resource = binding.Spec.Resource
			return nil
		})

		if err != nil {
			klog.Errorf("Failed to apply cluster policy(%s) for object: %s. error: %v", policy.Name, objectKey, err)
			return err
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.Infof("Create ClusterResourceBinding(%s) successfully.", binding.GetName())
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.Infof("Update ClusterResourceBinding(%s) successfully.", binding.GetName())
		} else {
			klog.V(2).Infof("ClusterResourceBinding(%s) is up to date.", binding.GetName())
		}
	}

	return nil
}

// GetUnstructuredObject retrieves object by key and returned its unstructured.
func (d *ResourceDetector) GetUnstructuredObject(objectKey keys.ClusterWideKey) (*unstructured.Unstructured, error) {
	objectGVR, err := restmapper.GetGroupVersionResource(d.RESTMapper, objectKey.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to get GVK of object: %s, error: %v", objectKey, err)
		return nil, err
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get object(%s), error: %v", objectKey, err)
		}
		return nil, err
	}

	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object(%s), error: %v", objectKey, err)
		return nil, err
	}

	return &unstructured.Unstructured{Object: uncastObj}, nil
}

// GetObject retrieves object from local cache.
func (d *ResourceDetector) GetObject(objectKey keys.ClusterWideKey) (runtime.Object, error) {
	objectGVR, err := restmapper.GetGroupVersionResource(d.RESTMapper, objectKey.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to get GVK of object: %s, error: %v", objectKey, err)
		return nil, err
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get object(%s), error: %v", objectKey, err)
		}
		return nil, err
	}

	return object, nil
}

// ClaimPolicyForObject set policy identifier which the object associated with.
func (d *ResourceDetector) ClaimPolicyForObject(object *unstructured.Unstructured, policyNamespace string, policyName string) error {
	claimedNS := util.GetLabelValue(object.GetLabels(), util.PropagationPolicyNamespaceLabel)
	claimedName := util.GetLabelValue(object.GetLabels(), util.PropagationPolicyNameLabel)

	// object has been claimed, don't need to claim again
	if claimedNS == policyNamespace && claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, util.PropagationPolicyNamespaceLabel, policyNamespace)
	util.MergeLabel(object, util.PropagationPolicyNameLabel, policyName)

	return d.Client.Update(context.TODO(), object)
}

// ClaimClusterPolicyForObject set cluster identifier which the object associated with.
func (d *ResourceDetector) ClaimClusterPolicyForObject(object *unstructured.Unstructured, policyName string) error {
	claimedName := util.GetLabelValue(object.GetLabels(), util.ClusterPropagationPolicyLabel)

	// object has been claimed, don't need to claim again
	if claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, util.ClusterPropagationPolicyLabel, policyName)
	return d.Client.Update(context.TODO(), object)
}

// BuildResourceBinding builds a desired ResourceBinding for object.
func (d *ResourceDetector) BuildResourceBinding(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, labels map[string]string) (*workv1alpha1.ResourceBinding, error) {
	bindingName := names.GenerateBindingName(object.GetKind(), object.GetName())
	replicaResourceRequirements, replicas, err := d.GetReplicaDeclaration(object)
	if err != nil {
		return nil, err
	}
	propagationBinding := &workv1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GroupVersionKind()),
			},
			Labels: labels,
		},
		Spec: workv1alpha1.ResourceBindingSpec{
			Resource: workv1alpha1.ObjectReference{
				APIVersion:                  object.GetAPIVersion(),
				Kind:                        object.GetKind(),
				Namespace:                   object.GetNamespace(),
				Name:                        object.GetName(),
				ResourceVersion:             object.GetResourceVersion(),
				ReplicaResourceRequirements: replicaResourceRequirements,
				Replicas:                    replicas,
			},
		},
	}

	return propagationBinding, nil
}

// BuildClusterResourceBinding builds a desired ClusterResourceBinding for object.
func (d *ResourceDetector) BuildClusterResourceBinding(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, labels map[string]string) (*workv1alpha1.ClusterResourceBinding, error) {
	bindingName := names.GenerateBindingName(object.GetKind(), object.GetName())
	replicaResourceRequirements, replicas, err := d.GetReplicaDeclaration(object)
	if err != nil {
		return nil, err
	}
	binding := &workv1alpha1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GroupVersionKind()),
			},
			Labels: labels,
		},
		Spec: workv1alpha1.ResourceBindingSpec{
			Resource: workv1alpha1.ObjectReference{
				APIVersion:                  object.GetAPIVersion(),
				Kind:                        object.GetKind(),
				Name:                        object.GetName(),
				ResourceVersion:             object.GetResourceVersion(),
				ReplicaResourceRequirements: replicaResourceRequirements,
				Replicas:                    replicas,
			},
		},
	}

	return binding, nil
}

// GetReplicaDeclaration get the replicas and resource requirements of a Deployment object
func (d *ResourceDetector) GetReplicaDeclaration(object *unstructured.Unstructured) (corev1.ResourceList, int32, error) {
	if object.GetKind() == util.DeploymentKind {
		replicas, ok, err := unstructured.NestedInt64(object.Object, util.SpecField, util.ReplicasField)
		if !ok || err != nil {
			return nil, 0, err
		}
		podTemplate, ok, err := unstructured.NestedMap(object.Object, util.SpecField, util.TemplateField)
		if !ok || err != nil {
			return nil, 0, err
		}
		replicaResourceRequirements, err := d.getReplicaResourceRequirements(podTemplate)
		if err != nil {
			return nil, 0, err
		}
		return replicaResourceRequirements, int32(replicas), nil
	}
	return nil, 0, nil
}

func (d *ResourceDetector) getReplicaResourceRequirements(object map[string]interface{}) (corev1.ResourceList, error) {
	var podTemplateSpec *corev1.PodTemplateSpec
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(object, &podTemplateSpec)
	if err != nil {
		return nil, err
	}
	resPtr := d.calculateResource(&podTemplateSpec.Spec)
	replicaResourceRequirements := resPtr.ResourceList()
	return replicaResourceRequirements, nil
}

func (d *ResourceDetector) calculateResource(podSpec *corev1.PodSpec) (res util.Resource) {
	resPtr := &res
	for _, c := range podSpec.Containers {
		resPtr.Add(c.Resources.Requests)
	}
	for _, c := range podSpec.InitContainers {
		resPtr.SetMaxResource(c.Resources.Requests)
	}
	return
}

// AddWaiting adds object's key to waiting list.
func (d *ResourceDetector) AddWaiting(objectKey keys.ClusterWideKey) {
	d.waitingLock.Lock()
	defer d.waitingLock.Unlock()

	d.waitingObjects[objectKey] = struct{}{}
	klog.V(1).Infof("Add object(%s) to waiting list, length of list is: %d", objectKey.String(), len(d.waitingObjects))
}

// RemoveWaiting removes object's key from waiting list.
func (d *ResourceDetector) RemoveWaiting(objectKey keys.ClusterWideKey) {
	d.waitingLock.Lock()
	defer d.waitingLock.Unlock()

	delete(d.waitingObjects, objectKey)
}

// GetMatching gets objects keys in waiting list that matches one of resource selectors.
func (d *ResourceDetector) GetMatching(resourceSelectors []policyv1alpha1.ResourceSelector) []keys.ClusterWideKey {
	d.waitingLock.RLock()
	defer d.waitingLock.RUnlock()

	var matchedResult []keys.ClusterWideKey

	for waitKey := range d.waitingObjects {
		waitObj, err := d.GetUnstructuredObject(waitKey)
		if err != nil {
			// all object in waiting list should exist. Just print a log to trace.
			klog.Errorf("Failed to get object(%s), error: %v", waitKey.String(), err)
			continue
		}

		for _, rs := range resourceSelectors {
			if util.ResourceMatches(waitObj, rs) {
				matchedResult = append(matchedResult, waitKey)
				break
			}
		}
	}

	return matchedResult
}

// OnPropagationPolicyAdd handles object add event and push the object to queue.
func (d *ResourceDetector) OnPropagationPolicyAdd(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(2).Infof("Create PropagationPolicy(%s)", key)
	d.policyReconcileWorker.AddRateLimited(key)
}

// OnPropagationPolicyUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnPropagationPolicyUpdate(oldObj, newObj interface{}) {
	// currently do nothing, since a policy's resource selector can not be updated.
}

// OnPropagationPolicyDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnPropagationPolicyDelete(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(2).Infof("Delete PropagationPolicy(%s)", key)
	d.policyReconcileWorker.AddRateLimited(key)
}

// ReconcilePropagationPolicy handles PropagationPolicy resource changes.
// When adding a PropagationPolicy, the detector will pick the objects in waitingObjects list that matches the policy and
// put the object to queue.
// When removing a PropagationPolicy, the relevant ResourceBinding will be removed and
// the relevant objects will be put into queue again to try another policy.
func (d *ResourceDetector) ReconcilePropagationPolicy(key util.QueueKey) error {
	ckey, ok := key.(keys.ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling propagation policy.")
		return fmt.Errorf("invalid key")
	}

	unstructuredObj, err := d.propagationPolicyLister.Get(ckey.NamespaceKey())
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("PropagationPolicy(%s) has been removed.", ckey.NamespaceKey())
			return d.HandlePropagationPolicyDeletion(ckey.Namespace, ckey.Name)
		}
		klog.Errorf("Failed to get PropagationPolicy(%s): %v", ckey.NamespaceKey(), err)
		return err
	}

	klog.Infof("PropagationPolicy(%s) has been added.", ckey.NamespaceKey())
	propagationObject, err := helper.ConvertToPropagationPolicy(unstructuredObj.(*unstructured.Unstructured))
	if err != nil {
		klog.Errorf("Failed to convert PropagationPolicy(%s) from unstructured object: %v", ckey.NamespaceKey(), err)
		return err
	}
	return d.HandlePropagationPolicyCreation(propagationObject)
}

// OnClusterPropagationPolicyAdd handles object add event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyAdd(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(2).Infof("Create ClusterPropagationPolicy(%s)", key)
	d.clusterPolicyReconcileWorker.AddRateLimited(key)
}

// OnClusterPropagationPolicyUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyUpdate(oldObj, newObj interface{}) {
	// currently do nothing, since a policy's resource selector can not be updated.
}

// OnClusterPropagationPolicyDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyDelete(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(2).Infof("Delete ClusterPropagationPolicy(%s)", key)
	d.clusterPolicyReconcileWorker.AddRateLimited(key)
}

// ReconcileClusterPropagationPolicy handles ClusterPropagationPolicy resource changes.
// When adding a ClusterPropagationPolicy, the detector will pick the objects in waitingObjects list that matches the policy and
// put the object to queue.
// When removing a ClusterPropagationPolicy, the relevant ClusterResourceBinding will be removed and
// the relevant objects will be put into queue again to try another policy.
func (d *ResourceDetector) ReconcileClusterPropagationPolicy(key util.QueueKey) error {
	ckey, ok := key.(keys.ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling cluster propagation policy.")
		return fmt.Errorf("invalid key")
	}

	unstructuredObj, err := d.clusterPropagationPolicyLister.Get(ckey.NamespaceKey())
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("ClusterPropagationPolicy(%s) has been removed.", ckey.NamespaceKey())
			return d.HandleClusterPropagationPolicyDeletion(ckey.Name)
		}

		klog.Errorf("Failed to get ClusterPropagationPolicy(%s): %v", ckey.NamespaceKey(), err)
		return err
	}

	klog.Infof("Policy(%s) has been added", ckey.NamespaceKey())
	propagationObject, err := helper.ConvertToClusterPropagationPolicy(unstructuredObj.(*unstructured.Unstructured))
	if err != nil {
		klog.Errorf("Failed to convert ClusterPropagationPolicy(%s) from unstructured object: %v", ckey.NamespaceKey(), err)
		return err
	}
	return d.HandleClusterPropagationPolicyCreation(propagationObject)
}

// HandlePropagationPolicyDeletion handles PropagationPolicy delete event.
// When policy removing, the associated ResourceBinding objects should be cleaned up.
// In addition, the label added to original resource also need to be cleaned up, this gives a chance for
// original resource to match another policy.
func (d *ResourceDetector) HandlePropagationPolicyDeletion(policyNS string, policyName string) error {
	labelSet := labels.Set{
		util.PropagationPolicyNamespaceLabel: policyNS,
		util.PropagationPolicyNameLabel:      policyName,
	}

	rbs, err := helper.GetResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to list propagation bindings: %v", err)
		return err
	}

	for itemIndex, binding := range rbs.Items {
		// Cleanup the labels from the object referencing by binding.
		// In addition, this will give the object a chance to match another policy.
		if err := d.CleanupLabels(binding.Spec.Resource, util.PropagationPolicyNameLabel, util.PropagationPolicyNameLabel); err != nil {
			klog.Errorf("Failed to cleanup label from resource(%s-%s/%s) when resource binding(%s/%s) removing, error: %v",
				binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, binding.Namespace, binding.Name, err)
			return err
		}

		klog.V(2).Infof("Removing binding(%s/%s)", binding.Namespace, binding.Name)
		if err := d.Client.Delete(context.TODO(), &rbs.Items[itemIndex]); err != nil {
			klog.Errorf("Failed to delete binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
			return err
		}
	}
	return nil
}

// HandleClusterPropagationPolicyDeletion handles ClusterPropagationPolicy delete event.
// When policy removing, the associated ClusterResourceBinding or ResourceBinding objects will be cleaned up.
// In addition, the label added to original resource also should be cleaned up, this gives a chance for
// original resource to match another policy.
func (d *ResourceDetector) HandleClusterPropagationPolicyDeletion(policyName string) error {
	var errs []error
	labelSet := labels.Set{
		util.ClusterPropagationPolicyLabel: policyName,
	}

	// load and remove the ClusterResourceBindings which labeled with current policy
	crbs, err := helper.GetClusterResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to load cluster resource binding by policy(%s), error: %v", policyName, err)
		errs = append(errs, err)
	} else if len(crbs.Items) > 0 {
		for itemIndex, binding := range crbs.Items {
			// Cleanup the labels from the object referencing by binding.
			// In addition, this will give the object a chance to match another policy.
			if err := d.CleanupLabels(binding.Spec.Resource, util.ClusterPropagationPolicyLabel); err != nil {
				klog.Errorf("Failed to cleanup label from resource(%s-%s/%s) when cluster resource binding(%s) removing, error: %v",
					binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, binding.Name, err)
				errs = append(errs, err)
			}

			klog.V(2).Infof("Removing cluster resource binding(%s)", binding.Name)
			if err := d.Client.Delete(context.TODO(), &crbs.Items[itemIndex]); err != nil {
				klog.Errorf("Failed to delete cluster resource binding(%s), error: %v", binding.Name, err)
				errs = append(errs, err)
			}
		}
	}

	// load and remove the ResourceBindings which labeled with current policy
	rbs, err := helper.GetResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to load resource binding by policy(%s), error: %v", policyName, err)
		errs = append(errs, err)
	} else if len(rbs.Items) > 0 {
		for itemIndex, binding := range rbs.Items {
			// Cleanup the labels from the object referencing by binding.
			// In addition, this will give the object a chance to match another policy.
			if err := d.CleanupLabels(binding.Spec.Resource, util.ClusterPropagationPolicyLabel); err != nil {
				klog.Errorf("Failed to cleanup label from resource binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
				errs = append(errs, err)
			}

			klog.V(2).Infof("Removing resource binding(%s)", binding.Name)
			if err := d.Client.Delete(context.TODO(), &rbs.Items[itemIndex]); err != nil {
				klog.Errorf("Failed to delete resource binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}

	return nil
}

// HandlePropagationPolicyCreation handles PropagationPolicy add event.
// When a new policy arrives, should check if object in waiting list matches the policy, if yes remove the object
// from waiting list and throw the object to it's reconcile queue. If not, do nothing.
func (d *ResourceDetector) HandlePropagationPolicyCreation(policy *policyv1alpha1.PropagationPolicy) error {
	matchedKeys := d.GetMatching(policy.Spec.ResourceSelectors)
	klog.Infof("Matched %d resources by policy(%s/%s)", len(matchedKeys), policy.Namespace, policy.Name)

	// check dependents only when there at least a real match.
	if len(matchedKeys) > 0 {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentOverridesPresent(d.Client, policy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", policy.Namespace, policy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}
	}

	for _, key := range matchedKeys {
		d.RemoveWaiting(key)
		d.Processor.AddRateLimited(key)
	}

	return nil
}

// HandleClusterPropagationPolicyCreation handles ClusterPropagationPolicy add event.
// When a new policy arrives, should check if object in waiting list matches the policy, if yes remove the object
// from waiting list and throw the object to it's reconcile queue. If not, do nothing.
func (d *ResourceDetector) HandleClusterPropagationPolicyCreation(policy *policyv1alpha1.ClusterPropagationPolicy) error {
	matchedKeys := d.GetMatching(policy.Spec.ResourceSelectors)
	klog.Infof("Matched %d resources by policy(%s)", len(matchedKeys), policy.Name)

	// check dependents only when there at least a real match.
	if len(matchedKeys) > 0 {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentClusterOverridesPresent(d.Client, policy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s)", policy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}
	}

	for _, key := range matchedKeys {
		d.RemoveWaiting(key)
		d.Processor.AddRateLimited(key)
	}

	return nil
}

// OnResourceBindingAdd handles object add event.
func (d *ResourceDetector) OnResourceBindingAdd(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	d.bindingReconcileWorker.AddRateLimited(key)
}

// OnResourceBindingUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnResourceBindingUpdate(_, newObj interface{}) {
	d.OnResourceBindingAdd(newObj)
}

// OnClusterResourceBindingDelete handles object delete event.
func (d *ResourceDetector) OnClusterResourceBindingDelete(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("Invalid object type: %v", reflect.TypeOf(obj))
		return
	}

	binding := &workv1alpha1.ClusterResourceBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), binding); err != nil {
		klog.Errorf("Failed to convert unstructured to typed object: %v", err)
		return
	}

	objRef := binding.Spec.Resource
	err := d.CleanupResourceTemplateStatus(binding.Spec.Resource)
	if err != nil {
		// Just log when error happened as there is no queue for delete event.
		klog.Warningf("Failed to cleanup resource(kind=%s, %s/%s) status: %v", objRef.Kind, objRef.Namespace, objRef.Name, err)
	}
}

// ReconcileResourceBinding handles ResourceBinding object changes.
// For each ResourceBinding changes, we will try to calculate the summary status and update to original object
// that the ResourceBinding refer to.
func (d *ResourceDetector) ReconcileResourceBinding(key util.QueueKey) error {
	ckey, ok := key.(keys.ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling resource binding.")
		return fmt.Errorf("invalid key")
	}

	unstructuredObj, err := d.resourceBindingLister.Get(ckey.NamespaceKey())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	binding, err := helper.ConvertToResourceBinding(unstructuredObj.(*unstructured.Unstructured))
	if err != nil {
		klog.Errorf("Failed to convert ResourceBinding(%s) from unstructured object: %v", ckey.NamespaceKey(), err)
		return err
	}

	klog.Infof("Reconciling resource binding(%s/%s)", binding.Namespace, binding.Name)
	switch binding.Spec.Resource.Kind {
	case util.DeploymentKind:
		return d.AggregateDeploymentStatus(binding.Spec.Resource, binding.Status.AggregatedStatus)
	default:
		// Unsupported resource type.
		return nil
	}
}

// OnClusterResourceBindingAdd handles object add event.
func (d *ResourceDetector) OnClusterResourceBindingAdd(obj interface{}) {

}

// OnClusterResourceBindingUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnClusterResourceBindingUpdate(oldObj, newObj interface{}) {
	d.OnClusterResourceBindingAdd(newObj)
}

// OnResourceBindingDelete handles object delete event.
func (d *ResourceDetector) OnResourceBindingDelete(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("Invalid object type: %v", reflect.TypeOf(obj))
		return
	}

	binding := &workv1alpha1.ResourceBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), binding); err != nil {
		klog.Errorf("Failed to convert unstructured to typed object: %v", err)
		return
	}

	objRef := binding.Spec.Resource
	err := d.CleanupResourceTemplateStatus(binding.Spec.Resource)
	if err != nil {
		// Just log when error happened as there is no queue for delete event.
		klog.Warningf("Failed to cleanup resource(kind=%s, %s/%s) status: %v", objRef.Kind, objRef.Namespace, objRef.Name, err)
	}
}

// AggregateDeploymentStatus summarize deployment status and update to original objects.
func (d *ResourceDetector) AggregateDeploymentStatus(objRef workv1alpha1.ObjectReference, status []workv1alpha1.AggregatedStatusItem) error {
	if objRef.APIVersion != "apps/v1" {
		return nil
	}

	obj := &appsv1.Deployment{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get deployment(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	oldStatus := &obj.Status
	newStatus := &appsv1.DeploymentStatus{}
	for _, item := range status {
		if item.Status == nil {
			continue
		}
		temp := &appsv1.DeploymentStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status")
			return err
		}
		klog.V(3).Infof("Scrub deployment(%s/%s) status from cluster(%s), replicas: %d, ready: %d, updated: %d, available: %d, unavailable: %d",
			obj.Namespace, obj.Name, item.ClusterName, temp.Replicas, temp.ReadyReplicas, temp.UpdatedReplicas, temp.AvailableReplicas, temp.UnavailableReplicas)
		newStatus.ObservedGeneration = obj.Generation
		newStatus.Replicas += temp.Replicas
		newStatus.ReadyReplicas += temp.ReadyReplicas
		newStatus.UpdatedReplicas += temp.UpdatedReplicas
		newStatus.AvailableReplicas += temp.AvailableReplicas
		newStatus.UnavailableReplicas += temp.UnavailableReplicas
	}

	if oldStatus.ObservedGeneration == newStatus.ObservedGeneration &&
		oldStatus.Replicas == newStatus.Replicas &&
		oldStatus.ReadyReplicas == newStatus.ReadyReplicas &&
		oldStatus.UpdatedReplicas == newStatus.UpdatedReplicas &&
		oldStatus.AvailableReplicas == newStatus.AvailableReplicas &&
		oldStatus.UnavailableReplicas == newStatus.UnavailableReplicas {
		klog.V(3).Infof("ignore update deployment(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

	oldStatus.ObservedGeneration = newStatus.ObservedGeneration
	oldStatus.Replicas = newStatus.Replicas
	oldStatus.ReadyReplicas = newStatus.ReadyReplicas
	oldStatus.UpdatedReplicas = newStatus.UpdatedReplicas
	oldStatus.AvailableReplicas = newStatus.AvailableReplicas
	oldStatus.UnavailableReplicas = newStatus.UnavailableReplicas

	if err := d.Client.Status().Update(context.TODO(), obj); err != nil {
		klog.Errorf("Failed to update deployment(%s/%s) status: %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	return nil
}

// CleanupResourceTemplateStatus cleanup the status from resource template.
// Note: Only limited resource type supported.
func (d *ResourceDetector) CleanupResourceTemplateStatus(objRef workv1alpha1.ObjectReference) error {
	switch objRef.Kind {
	case util.DeploymentKind:
		return d.CleanupDeploymentStatus(objRef)
	}

	// Unsupported resource type.
	return nil
}

// CleanupDeploymentStatus reinitialize Deployment status.
func (d *ResourceDetector) CleanupDeploymentStatus(objRef workv1alpha1.ObjectReference) error {
	if objRef.APIVersion != "apps/v1" {
		return nil
	}

	obj := &appsv1.Deployment{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get deployment(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	obj.Status = appsv1.DeploymentStatus{}

	if err := d.Client.Status().Update(context.TODO(), obj); err != nil {
		klog.Errorf("Failed to update deployment(%s/%s) status: %v", objRef.Namespace, objRef.Name, err)
		return err
	}
	klog.V(2).Infof("Reinitialized deployment(%s/%s) status.", objRef.Namespace, objRef.Name)

	return nil
}

// CleanupLabels removes labels from object referencing by objRef.
func (d *ResourceDetector) CleanupLabels(objRef workv1alpha1.ObjectReference, labels ...string) error {
	workload, err := helper.FetchWorkload(d.DynamicClient, d.RESTMapper, objRef)
	if err != nil {
		// do nothing if resource template not exist, it might has been removed.
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to fetch resource(kind=%s, %s/%s): %v", objRef.Kind, objRef.Namespace, objRef.Name, err)
		return err
	}

	workloadLabels := workload.GetLabels()
	for _, l := range labels {
		delete(workloadLabels, l)
	}
	workload.SetLabels(workloadLabels)

	gvr, err := restmapper.GetGroupVersionResource(d.RESTMapper, workload.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to delete resource(%s/%s) labels as mapping GVK to GVR failed: %v", workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	newWorkload, err := d.DynamicClient.Resource(gvr).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update resource %v/%v, err is %v ", workload.GetNamespace(), workload.GetName(), err)
		return err
	}
	klog.V(2).Infof("Updated resource template(kind=%s, %s/%s) successfully", newWorkload.GetKind(), newWorkload.GetNamespace(), newWorkload.GetName())
	return nil
}
