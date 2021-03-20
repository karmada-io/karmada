package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ResourceDetector is a resource watcher which watches all resources and reconcile the events.
type ResourceDetector struct {
	// ClientSet is used to resource discovery.
	ClientSet kubernetes.Interface
	// Client is used to retrieve objects, it is often more convenient than lister.
	Client          client.Client
	InformerManager informermanager.SingleClusterInformerManager
	EventHandler    cache.ResourceEventHandler
	Processor       util.AsyncWorker

	// policyReconcileWorker maintains a rate limited queue which used to store PropagationPolicy's key and
	// a reconcile function to consume the items in queue.
	policyReconcileWorker util.AsyncWorker

	// clusterPolicyReconcileWorker maintains a rate limited queue which used to store ClusterPropagationPolicy's key and
	// a reconcile function to consume the items in queue.
	clusterPolicyReconcileWorker util.AsyncWorker

	// bindingReconcileWorker maintains a rate limited queue which used to store ResourceBinding's key and
	// a reconcile function to consume the items in queue.
	bindingReconcileWorker util.AsyncWorker

	// clusterBindingReconcileWorker maintains a rate limited queue which used to store ClusterResourceBinding's key and
	// a reconcile function to consume the items in queue.
	clusterBindingReconcileWorker util.AsyncWorker

	RESTMapper meta.RESTMapper

	// waitingObjects tracks of objects which haven't be propagated yet as lack of appropriate policies.
	waitingObjects map[ClusterWideKey]struct{}
	// waitingLock is the lock for waitingObjects operation.
	waitingLock sync.RWMutex

	stopCh <-chan struct{}
}

// Start runs the detector, never stop until stopCh closed.
func (d *ResourceDetector) Start(stopCh <-chan struct{}) error {
	klog.Infof("Starting resource detector.")
	d.waitingObjects = make(map[ClusterWideKey]struct{})
	d.stopCh = stopCh

	// setup policy reconcile worker
	d.policyReconcileWorker = util.NewAsyncWorker("propagationpolicy detector", 1*time.Millisecond, ClusterWideKeyFunc, d.ReconcilePropagationPolicy)
	d.policyReconcileWorker.Run(1, d.stopCh)
	d.clusterPolicyReconcileWorker = util.NewAsyncWorker("cluster policy reconciler", time.Microsecond, ClusterWideKeyFunc, d.ReconcileClusterPropagationPolicy)
	d.clusterPolicyReconcileWorker.Run(1, d.stopCh)

	// watch and enqueue policy changes.
	policyHandler := informermanager.NewHandlerOnEvents(d.OnPropagationPolicyAdd, d.OnPropagationPolicyUpdate, d.OnPropagationPolicyDelete)
	d.InformerManager.ForResource(policyv1alpha1.SchemeGroupVersion.WithResource("propagationpolicies"), policyHandler)
	clusterPolicyHandler := informermanager.NewHandlerOnEvents(d.OnClusterPropagationPolicyAdd, d.OnClusterPropagationPolicyUpdate, d.OnClusterPropagationPolicyDelete)
	d.InformerManager.ForResource(policyv1alpha1.SchemeGroupVersion.WithResource("clusterpropagationpolicies"), clusterPolicyHandler)

	// setup binding reconcile worker
	d.bindingReconcileWorker = util.NewAsyncWorker("binding reconciler", time.Microsecond, ClusterWideKeyFunc, d.ReconcileResourceBinding)
	d.bindingReconcileWorker.Run(1, d.stopCh)
	d.clusterBindingReconcileWorker = util.NewAsyncWorker("cluster binding reconciler", time.Microsecond, ClusterWideKeyFunc, d.ReconcileClusterResourceBinding)
	d.clusterBindingReconcileWorker.Run(1, d.stopCh)

	// watch and enqueue binding changes.
	bindingHandler := informermanager.NewHandlerOnEvents(d.OnResourceBindingAdd, d.OnResourceBindingUpdate, d.OnResourceBindingDelete)
	d.InformerManager.ForResource(workv1alpha1.SchemeGroupVersion.WithResource("resourcebindings"), bindingHandler)
	clusterBindingHandler := informermanager.NewHandlerOnEvents(d.OnClusterResourceBindingAdd, d.OnClusterResourceBindingUpdate, d.OnClusterResourceBindingDelete)
	d.InformerManager.ForResource(workv1alpha1.SchemeGroupVersion.WithResource("clusterresourcebindings"), clusterBindingHandler)

	d.Processor.Run(1, stopCh)
	go d.discoverResources(30 * time.Second)

	<-stopCh
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// Check if our ResourceDetector implements necessary interfaces
var _ manager.Runnable = &ResourceDetector{}
var _ manager.LeaderElectionRunnable = &ResourceDetector{}

func (d *ResourceDetector) discoverResources(period time.Duration) {
	wait.Until(func() {
		newResources := GetDeletableResources(d.ClientSet.Discovery())
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
	clusterWideKey, ok := key.(ClusterWideKey)
	if !ok {
		klog.Error("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.Infof("Reconciling object: %s", clusterWideKey)

	object, err := d.GetUnstructuredObject(clusterWideKey)
	if err != nil {
		if errors.IsNotFound(err) {
			// The resource may no longer exist, in which case we stop processing.
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
func (d *ResourceDetector) EventFilter(obj interface{}) bool {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}

	clusterWideKey, ok := key.(ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return false
	}

	if strings.HasPrefix(clusterWideKey.Namespace, names.KubernetesReservedNSPrefix) ||
		strings.HasPrefix(clusterWideKey.Namespace, names.KarmadaReservedNSPrefix) {
		return false
	}

	if clusterWideKey.GVK.Group == clusterv1alpha1.GroupName ||
		clusterWideKey.GVK.Group == policyv1alpha1.GroupName ||
		clusterWideKey.GVK.Group == workv1alpha1.GroupName {
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
func (d *ResourceDetector) LookForMatchedPolicy(object *unstructured.Unstructured, objectKey ClusterWideKey) (*policyv1alpha1.PropagationPolicy, error) {
	if len(objectKey.Namespace) == 0 {
		return nil, nil
	}

	klog.V(2).Infof("attempts to match policy for resource: %s", objectKey)
	policyList := &policyv1alpha1.PropagationPolicyList{}
	if err := d.Client.List(context.TODO(), policyList, &client.ListOptions{Namespace: objectKey.Namespace}); err != nil {
		klog.Errorf("Failed to list propagation policy: %v", err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	for _, policy := range policyList.Items {
		for _, rs := range policy.Spec.ResourceSelectors {
			if util.ResourceMatches(object, rs) {
				return &policy, nil
			}
		}
	}

	return nil, nil
}

// LookForMatchedClusterPolicy tries to find a ClusterPropagationPolicy for object referenced by object key.
func (d *ResourceDetector) LookForMatchedClusterPolicy(object *unstructured.Unstructured, objectKey ClusterWideKey) (*policyv1alpha1.ClusterPropagationPolicy, error) {
	klog.V(2).Infof("attempts to match cluster policy for resource: %s", objectKey)
	policyList := &policyv1alpha1.ClusterPropagationPolicyList{}
	if err := d.Client.List(context.TODO(), policyList); err != nil {
		klog.Errorf("Failed to list cluster propagation policy: %v", err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	for _, policy := range policyList.Items {
		for _, rs := range policy.Spec.ResourceSelectors {
			if util.ResourceMatches(object, rs) {
				klog.V(2).Infof("Matched cluster policy(%s) for object(%s)", policy.Name, objectKey)
				return &policy, nil
			}
		}
	}
	return nil, nil
}

// ApplyPolicy starts propagate the object referenced by object key according to PropagationPolicy.
func (d *ResourceDetector) ApplyPolicy(object *unstructured.Unstructured, objectKey ClusterWideKey, policy *policyv1alpha1.PropagationPolicy) error {
	klog.Infof("Applying policy(%s) for object: %s", policy.Name, objectKey)

	if err := d.ClaimPolicyForObject(object, policy.Namespace, policy.Name); err != nil {
		klog.Errorf("Failed to claim policy(%s) for object: %s", policy.Name, object)
		return err
	}

	binding := d.BuildResourceBinding(object, objectKey, policy)
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
func (d *ResourceDetector) ApplyClusterPolicy(object *unstructured.Unstructured, objectKey ClusterWideKey, policy *policyv1alpha1.ClusterPropagationPolicy) error {
	klog.Infof("Applying cluster policy(%s) for object: %s", policy.Name, objectKey)

	if err := d.ClaimClusterPolicyForObject(object, policy.Name); err != nil {
		klog.Errorf("Failed to claim cluster policy(%s) for object: %s", policy.Name, object)
		return err
	}

	binding := d.BuildClusterResourceBinding(object, objectKey, policy)
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

	return nil
}

// GetUnstructuredObject retrieves object by key and returned its unstructured.
func (d *ResourceDetector) GetUnstructuredObject(objectKey ClusterWideKey) (*unstructured.Unstructured, error) {
	objectGVR, err := restmapper.GetGroupVersionResource(d.RESTMapper, objectKey.GVK)
	if err != nil {
		klog.Errorf("Failed to get GVK of object: %s, error: %v", objectKey, err)
		return nil, err
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		if !errors.IsNotFound(err) {
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
func (d *ResourceDetector) GetObject(objectKey ClusterWideKey) (runtime.Object, error) {
	objectGVR, err := restmapper.GetGroupVersionResource(d.RESTMapper, objectKey.GVK)
	if err != nil {
		klog.Errorf("Failed to get GVK of object: %s, error: %v", objectKey, err)
		return nil, err
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		if !errors.IsNotFound(err) {
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

	return d.Client.Update(context.TODO(), object.DeepCopyObject())
}

// ClaimClusterPolicyForObject set cluster identifier which the object associated with.
func (d *ResourceDetector) ClaimClusterPolicyForObject(object *unstructured.Unstructured, policyName string) error {
	claimedName := util.GetLabelValue(object.GetLabels(), util.ClusterPropagationPolicyLabel)

	// object has been claimed, don't need to claim again
	if claimedName == policyName {
		return nil
	}

	util.MergeLabel(object, util.ClusterPropagationPolicyLabel, policyName)
	return d.Client.Update(context.TODO(), object.DeepCopyObject())
}

// BuildResourceBinding builds a desired ResourceBinding for object.
func (d *ResourceDetector) BuildResourceBinding(object *unstructured.Unstructured, objectKey ClusterWideKey, policy *policyv1alpha1.PropagationPolicy) *workv1alpha1.ResourceBinding {
	bindingName := names.GenerateBindingName(object.GetNamespace(), object.GetKind(), object.GetName())
	propagationBinding := &workv1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GVK),
			},
			Labels: map[string]string{
				util.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
				util.PropagationPolicyNameLabel:      policy.GetName(),
			},
		},
		Spec: workv1alpha1.ResourceBindingSpec{
			Resource: workv1alpha1.ObjectReference{
				APIVersion:      object.GetAPIVersion(),
				Kind:            object.GetKind(),
				Namespace:       object.GetNamespace(),
				Name:            object.GetName(),
				ResourceVersion: object.GetResourceVersion(),
			},
		},
	}

	return propagationBinding
}

// BuildClusterResourceBinding builds a desired ClusterResourceBinding for object.
func (d *ResourceDetector) BuildClusterResourceBinding(object *unstructured.Unstructured, objectKey ClusterWideKey, policy *policyv1alpha1.ClusterPropagationPolicy) *workv1alpha1.ClusterResourceBinding {
	bindingName := names.GenerateClusterResourceBindingName(object.GetKind(), object.GetName())
	binding := &workv1alpha1.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GVK),
			},
			Labels: map[string]string{
				util.ClusterPropagationPolicyLabel: policy.GetName(),
			},
		},
		Spec: workv1alpha1.ResourceBindingSpec{
			Resource: workv1alpha1.ObjectReference{
				APIVersion:      object.GetAPIVersion(),
				Kind:            object.GetKind(),
				Namespace:       object.GetNamespace(),
				Name:            object.GetName(),
				ResourceVersion: object.GetResourceVersion(),
			},
		},
	}

	return binding
}

// AddWaiting adds object's key to waiting list.
func (d *ResourceDetector) AddWaiting(objectKey ClusterWideKey) {
	d.waitingLock.Lock()
	defer d.waitingLock.Unlock()

	d.waitingObjects[objectKey] = struct{}{}
	klog.V(1).Infof("Add object(%s) to waiting list, length of list is: %d", objectKey.String(), len(d.waitingObjects))
}

// RemoveWaiting removes object's key from waiting list.
func (d *ResourceDetector) RemoveWaiting(objectKey ClusterWideKey) {
	d.waitingLock.Lock()
	defer d.waitingLock.Unlock()

	delete(d.waitingObjects, objectKey)
}

// GetMatching gets objects keys in waiting list that matches one of resource selectors.
func (d *ResourceDetector) GetMatching(resourceSelectors []policyv1alpha1.ResourceSelector) []ClusterWideKey {
	d.waitingLock.RLock()
	defer d.waitingLock.RUnlock()

	var matchedResult []ClusterWideKey

	for waitKey := range d.waitingObjects {
		waitObj, err := d.GetUnstructuredObject(waitKey)
		if err != nil {
			// all object in waiting list should exist. Just print a log to trace.
			klog.Errorf("Failed to get object(%s), error: %v", waitKey, err)
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

	d.policyReconcileWorker.AddRateLimited(key)
}

// OnPropagationPolicyUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnPropagationPolicyUpdate(oldObj, newObj interface{}) {
	// currently do nothing, since a policy's resource selector can not be updated.
}

// OnPropagationPolicyDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnPropagationPolicyDelete(obj interface{}) {
	d.OnPropagationPolicyAdd(obj)
}

// ReconcilePropagationPolicy handles PropagationPolicy resource changes.
// When adding a PropagationPolicy, the detector will pick the objects in waitingObjects list that matches the policy and
// put the object to queue.
// When removing a PropagationPolicy, the relevant ResourceBinding will be removed and
// the relevant objects will be put into queue again to try another policy.
func (d *ResourceDetector) ReconcilePropagationPolicy(key util.QueueKey) error {
	ckey, ok := key.(ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling propagation policy.")
		return fmt.Errorf("invalid key")
	}

	policy := &policyv1alpha1.PropagationPolicy{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: ckey.Namespace, Name: ckey.Name}, policy); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Policy(%s) has been removed", ckey.NamespaceKey())
			return d.HandlePropagationPolicyDeletion(ckey.Namespace, ckey.Name)
		}
		return err
	}

	klog.Infof("Policy(%s) has been added", ckey.NamespaceKey())
	return d.HandlePropagationPolicyCreation(policy)
}

// OnClusterPropagationPolicyAdd handles object add event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyAdd(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	d.clusterPolicyReconcileWorker.AddRateLimited(key)
}

// OnClusterPropagationPolicyUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyUpdate(oldObj, newObj interface{}) {
	// currently do nothing, since a policy's resource selector can not be updated.
}

// OnClusterPropagationPolicyDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnClusterPropagationPolicyDelete(obj interface{}) {
	d.OnClusterPropagationPolicyAdd(obj)
}

// ReconcileClusterPropagationPolicy handles ClusterPropagationPolicy resource changes.
// When adding a ClusterPropagationPolicy, the detector will pick the objects in waitingObjects list that matches the policy and
// put the object to queue.
// When removing a ClusterPropagationPolicy, the relevant ClusterResourceBinding will be removed and
// the relevant objects will be put into queue again to try another policy.
func (d *ResourceDetector) ReconcileClusterPropagationPolicy(key util.QueueKey) error {
	ckey, ok := key.(ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling cluster propagation policy.")
		return fmt.Errorf("invalid key")
	}

	policy := &policyv1alpha1.ClusterPropagationPolicy{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Name: ckey.Name}, policy); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Policy(%s) has been removed", ckey.NamespaceKey())
			return d.HandlePropagationPolicyDeletion(ckey.Namespace, ckey.Name)
		}
		return err
	}

	klog.Infof("Policy(%s) has been added", ckey.NamespaceKey())
	return d.HandleClusterPropagationPolicyCreation(policy)
}

// HandlePropagationPolicyDeletion handles PropagationPolicy delete event.
// When policy removing, the associated ResourceBinding objects should be cleaned up.
// In addition, the label added to original resource also need to be cleaned up, this gives a chance for
// original resource to match another policy.
func (d *ResourceDetector) HandlePropagationPolicyDeletion(policyNS string, policyName string) error {
	bindings := &workv1alpha1.ResourceBindingList{}
	selector := labels.SelectorFromSet(labels.Set{
		util.PropagationPolicyNamespaceLabel: policyNS,
		util.PropagationPolicyNameLabel:      policyName,
	})
	listOpt := &client.ListOptions{LabelSelector: selector}

	if err := d.Client.List(context.TODO(), bindings, listOpt); err != nil {
		klog.Errorf("Failed to list propagation bindings: %v", err)
		return err
	}

	for _, binding := range bindings.Items {
		klog.V(2).Infof("Removing binding(%s/%s)", binding.Namespace, binding.Name)
		if err := d.Client.Delete(context.TODO(), &binding); err != nil {
			klog.Errorf("Failed to delete binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
			return err
		}
	}

	// TODO(RainbowMango): cleanup original resource's label.

	return nil
}

// HandleClusterPropagationPolicyDeletion handles ClusterPropagationPolicy delete event.
// When policy removing, the associated ClusterResourceBinding objects should be cleaned up.
// In addition, the label added to original resource also need to be cleaned up, this gives a chance for
// original resource to match another policy.
func (d *ResourceDetector) HandleClusterPropagationPolicyDeletion(policyName string) error {
	bindings := &workv1alpha1.ClusterResourceBindingList{}
	selector := labels.SelectorFromSet(labels.Set{
		util.ClusterPropagationPolicyLabel: policyName,
	})
	listOpt := &client.ListOptions{LabelSelector: selector}

	if err := d.Client.List(context.TODO(), bindings, listOpt); err != nil {
		klog.Errorf("Failed to list cluster propagation bindings: %v", err)
		return err
	}

	for _, binding := range bindings.Items {
		klog.V(2).Infof("Removing cluster resource binding(%s)", binding.Name)
		if err := d.Client.Delete(context.TODO(), &binding); err != nil {
			klog.Errorf("Failed to delete cluster resource binding(%s/%s), error: %v", binding.Namespace, binding.Name, err)
			return err
		}
	}

	// TODO(RainbowMango): cleanup original resource's label.

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
	// TODO(RainbowMango): cleanup status in resource template that current binding object refers to.
}

// ReconcileResourceBinding handles ResourceBinding object changes.
// For each ResourceBinding changes, we will try to calculate the summary status and update to original object
// that the ResourceBinding refer to.
func (d *ResourceDetector) ReconcileResourceBinding(key util.QueueKey) error {
	ckey, ok := key.(ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling resource binding.")
		return fmt.Errorf("invalid key")
	}

	binding := &workv1alpha1.ResourceBinding{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: ckey.Namespace, Name: ckey.Name}, binding); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	klog.Infof("Reconciling resource binding(%s/%s)", binding.Namespace, binding.Name)
	switch binding.Spec.Resource.Kind {
	case "Deployment":
		return d.AggregateDeploymentStatus(binding.Spec.Resource, binding.Status.AggregatedStatus)
	default:
		// Unsupported resource type.
		return nil
	}
}

// OnClusterResourceBindingAdd handles object add event.
func (d *ResourceDetector) OnClusterResourceBindingAdd(obj interface{}) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}

	d.clusterBindingReconcileWorker.AddRateLimited(key)
}

// OnClusterResourceBindingUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnClusterResourceBindingUpdate(oldObj, newObj interface{}) {
	d.OnClusterResourceBindingAdd(newObj)
}

// OnResourceBindingDelete handles object delete event.
func (d *ResourceDetector) OnResourceBindingDelete(obj interface{}) {
	// TODO(RainbowMango): cleanup status in resource template that current binding object refers to.
}

// ReconcileClusterResourceBinding handles ResourceBinding object changes.
// For each ClusterResourceBinding changes, we will try to calculate the summary status and update to original object
// that the ClusterResourceBinding refer to.
func (d *ResourceDetector) ReconcileClusterResourceBinding(key util.QueueKey) error {
	ckey, ok := key.(ClusterWideKey)
	if !ok { // should not happen
		klog.Error("Found invalid key when reconciling cluster resource binding.")
		return fmt.Errorf("invalid key")
	}

	binding := &workv1alpha1.ClusterResourceBinding{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Name: ckey.Name}, binding); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	klog.Infof("Reconciling cluster resource binding(%s)", binding.Name)
	switch binding.Spec.Resource.Kind {
	case "Deployment":
		return d.AggregateDeploymentStatus(binding.Spec.Resource, binding.Status.AggregatedStatus)
	default:
		// Unsupported resource type.
		return nil
	}
}

// AggregateDeploymentStatus summarize deployment status and update to original objects.
func (d *ResourceDetector) AggregateDeploymentStatus(objRef workv1alpha1.ObjectReference, status []workv1alpha1.AggregatedStatusItem) error {
	if objRef.APIVersion != "apps/v1" {
		return nil
	}

	obj := &appsv1.Deployment{}
	if err := d.Client.Get(context.TODO(), client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get deployment(%s/%s): %v", objRef.Namespace, objRef.Name, err)
		return err
	}

	oldStatus := &obj.Status
	newStatus := &appsv1.DeploymentStatus{}
	for _, item := range status {
		temp := &appsv1.DeploymentStatus{}
		if err := json.Unmarshal(item.Status.Raw, temp); err != nil {
			klog.Errorf("Failed to unmarshal status")
			return err
		}
		klog.V(3).Infof("Scrub deployment(%s/%s) status from cluster(%s), replicas: %d, ready: %d, updated: %d, available: %d, unavailable: %d",
			obj.Namespace, obj.Name, item.ClusterName, temp.Replicas, temp.ReadyReplicas, temp.UpdatedReplicas, temp.AvailableReplicas, temp.UnavailableReplicas)
		newStatus.Replicas += temp.Replicas
		newStatus.ReadyReplicas += temp.ReadyReplicas
		newStatus.UpdatedReplicas += temp.UpdatedReplicas
		newStatus.AvailableReplicas += temp.AvailableReplicas
		newStatus.UnavailableReplicas += temp.UnavailableReplicas
	}

	if oldStatus.Replicas == newStatus.Replicas &&
		oldStatus.ReadyReplicas == newStatus.ReadyReplicas &&
		oldStatus.UpdatedReplicas == newStatus.UpdatedReplicas &&
		oldStatus.AvailableReplicas == newStatus.AvailableReplicas &&
		oldStatus.UnavailableReplicas == newStatus.UnavailableReplicas {
		klog.V(3).Infof("ignore update deployment(%s/%s) status as up to date", obj.Namespace, obj.Name)
		return nil
	}

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
