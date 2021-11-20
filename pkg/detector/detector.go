package detector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/crdexplorer"
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
	// ResourceExplorer knows the details of resource structure.
	ResourceExplorer crdexplorer.CustomResourceExplorer

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
}

// Start runs the detector, never stop until stopCh closed.
func (d *ResourceDetector) Start(ctx context.Context) error {
	klog.Infof("Starting resource detector.")
	d.waitingObjects = make(map[keys.ClusterWideKey]struct{})
	d.stopCh = ctx.Done()

	// setup policy reconcile worker
	d.policyReconcileWorker = util.NewAsyncWorker("propagationPolicy reconciler", ClusterWideKeyFunc, d.ReconcilePropagationPolicy)
	d.policyReconcileWorker.Run(1, d.stopCh)
	d.clusterPolicyReconcileWorker = util.NewAsyncWorker("clusterPropagationPolicy reconciler", ClusterWideKeyFunc, d.ReconcileClusterPropagationPolicy)
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
	d.bindingReconcileWorker = util.NewAsyncWorker("resourceBinding reconciler", ClusterWideKeyFunc, d.ReconcileResourceBinding)
	d.bindingReconcileWorker.Run(1, d.stopCh)

	// watch and enqueue ResourceBinding changes.
	resourceBindingGVR := schema.GroupVersionResource{
		Group:    workv1alpha2.GroupVersion.Group,
		Version:  workv1alpha2.GroupVersion.Version,
		Resource: "resourcebindings",
	}
	bindingHandler := informermanager.NewHandlerOnEvents(d.OnResourceBindingAdd, d.OnResourceBindingUpdate, nil)
	d.InformerManager.ForResource(resourceBindingGVR, bindingHandler)
	d.resourceBindingLister = d.InformerManager.Lister(resourceBindingGVR)

	// watch and enqueue ClusterResourceBinding changes.
	clusterResourceBindingGVR := schema.GroupVersionResource{
		Group:    workv1alpha2.GroupVersion.Group,
		Version:  workv1alpha2.GroupVersion.Version,
		Resource: "clusterresourcebindings",
	}
	clusterBindingHandler := informermanager.NewHandlerOnEvents(d.OnClusterResourceBindingAdd, d.OnClusterResourceBindingUpdate, nil)
	d.InformerManager.ForResource(clusterResourceBindingGVR, clusterBindingHandler)

	d.EventHandler = informermanager.NewFilteringHandlerOnAllEvents(d.EventFilter, d.OnAdd, d.OnUpdate, d.OnDelete)
	d.Processor = util.NewAsyncWorker("resource detector", ClusterWideKeyFunc, d.Reconcile)
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
		newResources := GetDeletableResources(d.DiscoveryClientSet)
		for r := range newResources {
			if d.InformerManager.IsHandlerExist(r, d.EventHandler) || d.gvrDisabled(r) {
				continue
			}
			klog.Infof("Setup informer for %s", r.String())
			d.InformerManager.ForResource(r, d.EventHandler)
		}
		d.InformerManager.Start()
	}, period, d.stopCh)
}

// gvrDisabled returns whether GroupVersionResource is disabled.
func (d *ResourceDetector) gvrDisabled(gvr schema.GroupVersionResource) bool {
	if d.SkippedResourceConfig == nil {
		return false
	}

	if d.SkippedResourceConfig.GroupVersionDisabled(gvr.GroupVersion()) {
		return true
	}
	if d.SkippedResourceConfig.GroupDisabled(gvr.Group) {
		return true
	}

	gvks, err := d.RESTMapper.KindsFor(gvr)
	if err != nil {
		klog.Errorf("gvr(%s) transform failed: %v", gvr.String(), err)
		return false
	}

	for _, gvk := range gvks {
		if d.SkippedResourceConfig.GroupVersionKindDisabled(gvk) {
			return true
		}
	}

	return false
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
	policyObjects, err := d.propagationPolicyLister.ByNamespace(objectKey.Namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list propagation policy: %v", err)
		return nil, err
	}
	if len(policyObjects) == 0 {
		klog.V(2).Infof("no propagationpolicy find in namespace(%s).", objectKey.Namespace)
		return nil, nil
	}

	policyList := make([]*policyv1alpha1.PropagationPolicy, 0)
	for index := range policyObjects {
		policy, err := helper.ConvertToPropagationPolicy(policyObjects[index].(*unstructured.Unstructured))
		if err != nil {
			klog.Errorf("Failed to convert PropagationPolicy from unstructured object: %v", err)
			return nil, err
		}
		policyList = append(policyList, policy)
	}

	matchedPolicies := make([]*policyv1alpha1.PropagationPolicy, 0)
	for _, policy := range policyList {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedPolicies = append(matchedPolicies, policy)
		}
	}

	sort.Slice(matchedPolicies, func(i, j int) bool {
		return matchedPolicies[i].Name < matchedPolicies[j].Name
	})

	if len(matchedPolicies) == 0 {
		klog.V(2).Infof("no propagationpolicy match for resource(%s)", objectKey)
		return nil, nil
	}
	klog.V(2).Infof("Matched policy(%s/%s) for resource(%s)", matchedPolicies[0].Namespace, matchedPolicies[0].Name, objectKey)
	return matchedPolicies[0], nil
}

// LookForMatchedClusterPolicy tries to find a ClusterPropagationPolicy for object referenced by object key.
func (d *ResourceDetector) LookForMatchedClusterPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey) (*policyv1alpha1.ClusterPropagationPolicy, error) {
	klog.V(2).Infof("attempts to match cluster policy for resource(%s)", objectKey)
	policyObjects, err := d.clusterPropagationPolicyLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list cluster propagation policy: %v", err)
		return nil, err
	}
	if len(policyObjects) == 0 {
		klog.V(2).Infof("no propagationpolicy find.")
		return nil, nil
	}

	policyList := make([]*policyv1alpha1.ClusterPropagationPolicy, 0)
	for index := range policyObjects {
		policy, err := helper.ConvertToClusterPropagationPolicy(policyObjects[index].(*unstructured.Unstructured))
		if err != nil {
			klog.Errorf("Failed to convert ClusterPropagationPolicy from unstructured object: %v", err)
			return nil, err
		}
		policyList = append(policyList, policy)
	}

	matchedClusterPolicies := make([]*policyv1alpha1.ClusterPropagationPolicy, 0)
	for _, policy := range policyList {
		if util.ResourceMatchSelectors(object, policy.Spec.ResourceSelectors...) {
			matchedClusterPolicies = append(matchedClusterPolicies, policy)
		}
	}

	sort.Slice(matchedClusterPolicies, func(i, j int) bool {
		return matchedClusterPolicies[i].Name < matchedClusterPolicies[j].Name
	})

	if len(matchedClusterPolicies) == 0 {
		klog.V(2).Infof("no propagationpolicy match for resource(%s)", objectKey)
		return nil, nil
	}
	klog.V(2).Infof("Matched cluster policy(%s) for resource(%s)", matchedClusterPolicies[0].Name, objectKey)
	return matchedClusterPolicies[0], nil
}

// ApplyPolicy starts propagate the object referenced by object key according to PropagationPolicy.
func (d *ResourceDetector) ApplyPolicy(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policy *policyv1alpha1.PropagationPolicy) error {
	klog.Infof("Applying policy(%s) for object: %s", policy.Name, objectKey)

	if err := d.ClaimPolicyForObject(object, policy.Namespace, policy.Name); err != nil {
		klog.Errorf("Failed to claim policy(%s) for object: %s", policy.Name, object)
		return err
	}

	policyLabels := map[string]string{
		policyv1alpha1.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
		policyv1alpha1.PropagationPolicyNameLabel:      policy.GetName(),
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
		bindingCopy.Finalizers = binding.Finalizers
		bindingCopy.Spec.Resource = binding.Spec.Resource
		bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
		bindingCopy.Spec.Replicas = binding.Spec.Replicas
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
		policyv1alpha1.ClusterPropagationPolicyLabel: policy.GetName(),
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
			bindingCopy.Finalizers = binding.Finalizers
			bindingCopy.Spec.Resource = binding.Spec.Resource
			bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
			bindingCopy.Spec.Replicas = binding.Spec.Replicas
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
			bindingCopy.Finalizers = binding.Finalizers
			bindingCopy.Spec.Resource = binding.Spec.Resource
			bindingCopy.Spec.ReplicaRequirements = binding.Spec.ReplicaRequirements
			bindingCopy.Spec.Replicas = binding.Spec.Replicas
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
		klog.Errorf("Failed to get GVR of object: %s, error: %v", objectKey, err)
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
	claimedNS := util.GetLabelValue(object.GetLabels(), policyv1alpha1.PropagationPolicyNamespaceLabel)
	claimedName := util.GetLabelValue(object.GetLabels(), policyv1alpha1.PropagationPolicyNameLabel)

	// object has been claimed, don't need to claim again
	if claimedNS == policyNamespace && claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, policyv1alpha1.PropagationPolicyNamespaceLabel, policyNamespace)
	util.MergeLabel(object, policyv1alpha1.PropagationPolicyNameLabel, policyName)

	return d.Client.Update(context.TODO(), object)
}

// ClaimClusterPolicyForObject set cluster identifier which the object associated with.
func (d *ResourceDetector) ClaimClusterPolicyForObject(object *unstructured.Unstructured, policyName string) error {
	claimedName := util.GetLabelValue(object.GetLabels(), policyv1alpha1.ClusterPropagationPolicyLabel)

	// object has been claimed, don't need to claim again
	if claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, policyv1alpha1.ClusterPropagationPolicyLabel, policyName)
	return d.Client.Update(context.TODO(), object)
}

// BuildResourceBinding builds a desired ResourceBinding for object.
func (d *ResourceDetector) BuildResourceBinding(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, labels map[string]string) (*workv1alpha2.ResourceBinding, error) {
	bindingName := names.GenerateBindingName(object.GetKind(), object.GetName())
	propagationBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GroupVersionKind()),
			},
			Labels:     labels,
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      object.GetAPIVersion(),
				Kind:            object.GetKind(),
				Namespace:       object.GetNamespace(),
				Name:            object.GetName(),
				ResourceVersion: object.GetResourceVersion(),
			},
		},
	}

	if d.ResourceExplorer.HookEnabled(object, configv1alpha1.InterpreterOperationInterpretReplica) {
		replicas, replicaRequirements, err := d.ResourceExplorer.GetReplicas(object)
		if err != nil {
			klog.Errorf("Failed to customize replicas for %s(%s), %v", object.GroupVersionKind(), object.GetName(), err)
			return nil, err
		}
		propagationBinding.Spec.Replicas = replicas
		propagationBinding.Spec.ReplicaRequirements = replicaRequirements
	}

	return propagationBinding, nil
}

// BuildClusterResourceBinding builds a desired ClusterResourceBinding for object.
func (d *ResourceDetector) BuildClusterResourceBinding(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, labels map[string]string) (*workv1alpha2.ClusterResourceBinding, error) {
	bindingName := names.GenerateBindingName(object.GetKind(), object.GetName())
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GroupVersionKind()),
			},
			Labels:     labels,
			Finalizers: []string{util.ClusterResourceBindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      object.GetAPIVersion(),
				Kind:            object.GetKind(),
				Name:            object.GetName(),
				ResourceVersion: object.GetResourceVersion(),
			},
		},
	}

	if d.ResourceExplorer.HookEnabled(object, configv1alpha1.InterpreterOperationInterpretReplica) {
		replicas, replicaRequirements, err := d.ResourceExplorer.GetReplicas(object)
		if err != nil {
			klog.Errorf("Failed to customize replicas for %s(%s), %v", object.GroupVersionKind(), object.GetName(), err)
			return nil, err
		}
		binding.Spec.Replicas = replicas
		binding.Spec.ReplicaRequirements = replicaRequirements
	}

	return binding, nil
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
// After a policy is removed, the label marked on relevant resource template will be removed(which gives
// the resource template a change to match another policy).
//
// Note: The relevant ResourceBinding will continue to exist until the resource template is gone.
func (d *ResourceDetector) HandlePropagationPolicyDeletion(policyNS string, policyName string) error {
	labelSet := labels.Set{
		policyv1alpha1.PropagationPolicyNamespaceLabel: policyNS,
		policyv1alpha1.PropagationPolicyNameLabel:      policyName,
	}

	rbs, err := helper.GetResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to list propagation bindings: %v", err)
		return err
	}

	for _, binding := range rbs.Items {
		// Cleanup the labels from the object referencing by binding.
		// In addition, this will give the object a chance to match another policy.
		if err := d.CleanupLabels(binding.Spec.Resource, policyv1alpha1.PropagationPolicyNamespaceLabel, policyv1alpha1.PropagationPolicyNameLabel); err != nil {
			klog.Errorf("Failed to cleanup label from resource(%s-%s/%s) when resource binding(%s/%s) removing, error: %v",
				binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, binding.Namespace, binding.Name, err)
			return err
		}
	}
	return nil
}

// HandleClusterPropagationPolicyDeletion handles ClusterPropagationPolicy delete event.
// After a policy is removed, the label marked on relevant resource template will be removed(which gives
// the resource template a change to match another policy).
//
// Note: The relevant ClusterResourceBinding or ResourceBinding will continue to exist until the resource template is gone.
func (d *ResourceDetector) HandleClusterPropagationPolicyDeletion(policyName string) error {
	var errs []error
	labelSet := labels.Set{
		policyv1alpha1.ClusterPropagationPolicyLabel: policyName,
	}

	// load the ClusterResourceBindings which labeled with current policy
	crbs, err := helper.GetClusterResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to load cluster resource binding by policy(%s), error: %v", policyName, err)
		errs = append(errs, err)
	} else if len(crbs.Items) > 0 {
		for _, binding := range crbs.Items {
			// Cleanup the labels from the object referencing by binding.
			// In addition, this will give the object a chance to match another policy.
			if err := d.CleanupLabels(binding.Spec.Resource, policyv1alpha1.ClusterPropagationPolicyLabel); err != nil {
				klog.Errorf("Failed to cleanup label from resource(%s-%s/%s) when cluster resource binding(%s) removing, error: %v",
					binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, binding.Name, err)
				errs = append(errs, err)
			}
		}
	}

	// load the ResourceBindings which labeled with current policy
	rbs, err := helper.GetResourceBindings(d.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to load resource binding by policy(%s), error: %v", policyName, err)
		errs = append(errs, err)
	} else if len(rbs.Items) > 0 {
		for _, binding := range rbs.Items {
			// Cleanup the labels from the object referencing by binding.
			// In addition, this will give the object a chance to match another policy.
			if err := d.CleanupLabels(binding.Spec.Resource, policyv1alpha1.ClusterPropagationPolicyLabel); err != nil {
				klog.Errorf("Failed to cleanup label from resource binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
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
	case util.ServiceKind:
		return d.AggregateServiceStatus(binding.Spec.Resource, binding.Status.AggregatedStatus)
	case util.IngressKind:
		return d.AggregateIngressStatus(binding.Spec.Resource, binding.Status.AggregatedStatus)
	case util.JobKind:
		return d.AggregateJobStatus(binding.Spec.Resource, binding.Status.AggregatedStatus, binding.Spec.Clusters)
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

// CleanupLabels removes labels from object referencing by objRef.
func (d *ResourceDetector) CleanupLabels(objRef workv1alpha2.ObjectReference, labels ...string) error {
	workload, err := helper.FetchWorkload(d.DynamicClient, d.InformerManager, d.RESTMapper, objRef)
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
