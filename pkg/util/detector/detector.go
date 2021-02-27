package detector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/karmada-io/karmada/pkg/util"
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
	RESTMapper      meta.RESTMapper
	stopCh          <-chan struct{}
}

// Start runs the detector, never stop until stopCh closed.
func (d *ResourceDetector) Start(stopCh <-chan struct{}) error {
	klog.Infof("Starting resource detector.")
	d.stopCh = stopCh

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
	klog.V(2).Infof("Start to propagate object: %s", clusterWideKey)

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

	// for namespace scoped resource, first attempts to match policy in it's namespace.
	propagationPolicy, err := d.LookForMatchedPolicy(object, clusterWideKey)
	if err != nil {
		klog.Errorf("Failed to retrieve policy for object: %s, error: %v", clusterWideKey.String(), err)
		return err
	}
	if propagationPolicy != nil {
		return d.ApplyPolicy(object, clusterWideKey, propagationPolicy)
	}

	klog.V(2).Infof("No appropriate policy found for object: %s", clusterWideKey.String())
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
		clusterWideKey.GVK.Group == policyv1alpha1.GroupName {
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

	klog.V(2).Infof("attempts to match policy in namespace: %s", objectKey.Namespace)
	policyList := &policyv1alpha1.PropagationPolicyList{}
	if err := d.Client.List(context.TODO(), policyList, &client.ListOptions{Namespace: objectKey.Namespace}); err != nil {
		klog.Errorf("Failed to list propagation policy: %v", err)
		return nil, err
	}

	klog.V(2).Infof("found %d policies in namespace: %s", len(policyList.Items), objectKey.Namespace)
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

// ApplyPolicy starts propagate the object referenced by object key.
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

// GetUnstructuredObject retrieves object by key and returned its unstructured.
func (d *ResourceDetector) GetUnstructuredObject(objectKey ClusterWideKey) (*unstructured.Unstructured, error) {
	objectGVR, err := restmapper.GetGroupVersionResource(d.RESTMapper, objectKey.GVK)
	if err != nil {
		klog.Errorf("Failed to get GVK of object: %s, error: %v", objectKey, err)
		return nil, err
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		klog.Errorf("Failed to get object(%s), error: %v", objectKey, err)
		return nil, err
	}

	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object(%s), error: %v", objectKey, err)
		return nil, err
	}

	return &unstructured.Unstructured{Object: uncastObj}, nil
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

// BuildResourceBinding builds a desired PropagationBinding for object.
func (d *ResourceDetector) BuildResourceBinding(object *unstructured.Unstructured, objectKey ClusterWideKey, policy *policyv1alpha1.PropagationPolicy) *policyv1alpha1.PropagationBinding {
	bindingName := names.GenerateBindingName(object.GetNamespace(), object.GetKind(), object.GetName())
	propagationBinding := &policyv1alpha1.PropagationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(object, objectKey.GVK),
			},
			Labels: map[string]string{
				util.PropagationPolicyNamespaceLabel: policy.GetNamespace(),
				util.PropagationPolicyNameLabel:      policy.GetName(),
				// Deprecate util.OwnerLable later
				util.OwnerLabel: names.GenerateOwnerLabelValue(policy.GetNamespace(), policy.GetName()),
			},
		},
		Spec: policyv1alpha1.PropagationBindingSpec{
			Resource: policyv1alpha1.ObjectReference{
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
