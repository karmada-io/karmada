package resource

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/policy/applyer"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

// Controller is a controller that handle `gvk` specified resource
type Controller struct {
	workqueue                      workqueue.RateLimitingInterface
	client                         client.Client
	gvk                            schema.GroupVersionKind
	gvr                            schema.GroupVersionResource
	apiResource                    *metav1.APIResource
	dynamicClient                  dynamic.Interface
	informerManager                informermanager.SingleClusterInformerManager
	propagationPolicyLister        cache.GenericLister
	clusterPropagationPolicyLister cache.GenericLister
	// resourceInterpreter knows the details of resource structure.
	resourceInterpreter resourceinterpreter.ResourceInterpreter
	eventRecorder       record.EventRecorder
}

// NewController starts a resource controller corrsponding specific GVK
func NewController(apiResource *metav1.APIResource, client client.Client,
	dynamicClient dynamic.Interface, informer informermanager.SingleClusterInformerManager, resourceInterpreter resourceinterpreter.ResourceInterpreter, eventRecorder record.EventRecorder) (*Controller, error) {
	gvk := schema.GroupVersionKind{Group: apiResource.Group,
		Version: apiResource.Version, Kind: apiResource.Kind}
	gvr := schema.GroupVersionResource{Group: apiResource.Group,
		Version: apiResource.Version, Resource: apiResource.Name}
	c := &Controller{
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), gvk.String()),
		client:              client,
		gvk:                 gvk,
		gvr:                 gvr,
		dynamicClient:       dynamicClient,
		informerManager:     informer,
		apiResource:         apiResource,
		resourceInterpreter: resourceInterpreter,
		eventRecorder:       eventRecorder,
	}
	propagationPolicyGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.GroupVersion.Group,
		Version:  policyv1alpha1.GroupVersion.Version,
		Resource: "propagationpolicies",
	}
	clusterPropagationPolicyGVR := schema.GroupVersionResource{
		Group:    policyv1alpha1.GroupVersion.Group,
		Version:  policyv1alpha1.GroupVersion.Version,
		Resource: "clusterpropagationpolicies",
	}
	c.propagationPolicyLister = c.informerManager.Lister(propagationPolicyGVR)
	c.clusterPropagationPolicyLister = c.informerManager.Lister(clusterPropagationPolicyGVR)
	addFunc := func(obj interface{}) {
		resource := obj.(*unstructured.Unstructured)
		klog.V(4).Infof("adding %s %q", c.gvk, klog.KObj(resource))
		c.enqueue(resource)
	}
	updateFunc := func(oldObj interface{}, newObj interface{}) {
		resource := newObj.(*unstructured.Unstructured)
		klog.V(4).Infof("updating %s %q", c.gvk, klog.KObj(resource))
		c.enqueue(resource)
	}
	deleteFunc := func(obj interface{}) {
		resource := obj.(*unstructured.Unstructured)
		klog.V(4).Infof("deleting %s %q", c.gvk, klog.KObj(resource))
		c.enqueue(resource)
	}
	c.informerManager.ForResource(gvr, informermanager.NewHandlerOnEvents(addFunc, updateFunc, deleteFunc))
	c.informerManager.Start()
	return c, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Infof("starting %s controller...", c.gvk)
	defer klog.Infof("shutting down %s controller", c.gvk)
	// Wait for the caches to be synced before starting workers
	synced := c.informerManager.WaitForCacheSync()
	if _, ok := synced[c.gvr]; !ok {
		return
	}

	klog.V(5).Infof("starting %d worker threads", workers)
	// Launch workers to process resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// runWorker process object in workqueue
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(obj)
		klog.Infof("successfully sync resource %s", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncHandler(key string) error {
	klog.V(4).Infof("start processing resource %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}
	object, err := c.dynamicClient.Resource(c.gvr).Namespace(namespace).Get(context.TODO(),
		name, metav1.GetOptions{})
	if err != nil {
		// klog.Errorf("Failed to get workload from api server, kind: %s, namespace: %s, name: %s. Error: %v",
		// 	resource.Kind, resource.Namespace, resource.Name, err)
		return err
	}
	clusterWideKey, err := keys.ClusterWideKeyFunc(object)
	if err != nil {
		return err
	}
	propagationPolicy, err := c.lookForMatchedPolicy(object, clusterWideKey)
	if err != nil {
		klog.Errorf("Failed to retrieve policy for object: %s, error: %v", clusterWideKey.String(), err)
		return err
	}
	if propagationPolicy != nil {
		// return err when dependents not present, that we can retry at next reconcile.
		if present, err := helper.IsDependentOverridesPresent(c.client, propagationPolicy); err != nil || !present {
			klog.Infof("Waiting for dependent overrides present for policy(%s/%s)", propagationPolicy.Namespace, propagationPolicy.Name)
			return fmt.Errorf("waiting for dependent overrides")
		}
		up, err := helper.ToUnstructured(propagationPolicy)
		if err != nil {
			return err
		}
		apply := applyer.NewPropagationPolicyApplyer(c.client, c.resourceInterpreter)
		apply.Apply(object, clusterWideKey, up)
	}

	// reaching here means there is no appropriate PropagationPolicy, attempts to match a ClusterPropagationPolicy.
	clusterPolicy, err := c.lookForMatchedClusterPolicy(object, clusterWideKey)
	if err != nil {
		klog.Errorf("Failed to retrieve cluster policy for object: %s, error: %v", clusterWideKey.String(), err)
		return err
	}
	if clusterPolicy != nil {
		up, err := helper.ToUnstructured(propagationPolicy)
		if err != nil {
			return err
		}
		apply := applyer.NewClusterPropagationPolicyApplyer(c.client, c.resourceInterpreter)
		apply.Apply(object, clusterWideKey, up)
	}
	return nil
}

// enqueue puts key onto the work queue.
func (c *Controller) enqueue(obj *unstructured.Unstructured) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}
	c.workqueue.Add(key)
}
