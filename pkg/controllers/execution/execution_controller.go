package execution

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
	pagationstrategy "github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	clientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
	karmadakubecheme "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned/scheme"
	informers "github.com/huawei-cloudnative/karmada/pkg/generated/informers/externalversions"
	listers "github.com/huawei-cloudnative/karmada/pkg/generated/listers/propagationstrategy/v1alpha1"
)

const (
	controllerAgentName = "execution-controller"
	finalizer           = "karmada.io/execution-controller"
	memberClusterNS     = "karmada-cluster"
)

// Controller is the controller implementation for PropagationWork resources
type Controller struct {
	// karmadaClientSet is the clientset for our own API group.
	karmadaClientSet clientset.Interface

	// kubeClientSet is a standard kubernetes clientset.
	kubeClientSet kubernetes.Interface

	karmadaInformerFactory informers.SharedInformerFactory
	propagationWorkLister  listers.PropagationWorkLister
	propagationWorkSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	eventRecorder record.EventRecorder
}

// StartExecutionController starts a new execution controller.
func StartExecutionController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	controller, err := newExecutionController(config)
	if err != nil {
		return err
	}
	klog.Infof("Starting execution controller")

	go wait.Until(func() {
		if err := controller.Run(2, stopChan); err != nil {
			klog.Errorf("controller exit unexpected! will restart later, controller: %s, error: %v", controllerAgentName, err)
		}
	}, 1*time.Second, stopChan)

	return nil
}

// newExecutionController returns a new controller.
func newExecutionController(config *util.ControllerConfig) (*Controller, error) {
	headClusterConfig := rest.CopyConfig(config.HeadClusterConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(headClusterConfig)

	karmadaClientSet := clientset.NewForConfigOrDie(headClusterConfig)
	karmadaInformerFactory := informers.NewSharedInformerFactory(karmadaClientSet, 0)
	PropagationWorkInformer := karmadaInformerFactory.Propagationstrategy().V1alpha1().PropagationWorks()

	// Add karmada types to the default Kubernetes Scheme so Events can be logged for karmada types.
	utilruntime.Must(karmadakubecheme.AddToScheme(scheme.Scheme))

	// Create event broadcaster
	klog.V(1).Infof("Creating event broadcaster for %s", controllerAgentName)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})

	controller := &Controller{
		karmadaClientSet:       karmadaClientSet,
		kubeClientSet:          kubeClientSet,
		karmadaInformerFactory: karmadaInformerFactory,
		propagationWorkLister:  PropagationWorkInformer.Lister(),
		propagationWorkSynced:  PropagationWorkInformer.Informer().HasSynced,
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		eventRecorder:          eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}

	klog.Info("Setting up event handlers")
	PropagationWorkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("Received add event. just add to queue.")
			controller.enqueueEventResource(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			klog.Infof("Received update event. just add to queue.")
			controller.enqueueEventResource(new)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("Received delete event. just add to queue.")
			controller.enqueueEventResource(obj)
		},
	})

	return controller, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workerNumber int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Infof("Run controller: %s", controllerAgentName)
	c.karmadaInformerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.propagationWorkSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Infof("Starting workers for controller. worker number: %d, controller: %s", workerNumber, controllerAgentName)
	for i := 0; i < workerNumber; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	// Controller will block here until stopCh is closed.
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
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

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// PropagateStrategy resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PropagateStrategy resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the resource with this namespace/name
	propagationWork, err := c.propagationWorkLister.PropagationWorks(namespace).Get(name)
	if err != nil {
		// The PropagationWork resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("PropagationWork '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	if propagationWork.GetDeletionTimestamp() != nil {
		applied := c.isResourceApplied(&propagationWork.Status)
		if applied {
			err = c.deletePropagationWork(propagationWork)
			if err != nil {
				klog.Infof("Failed to delete propagationwork %v, err is %v", propagationWork.Name, err)
				return err
			}
		}
		return c.removeFinalizer(propagationWork)
	}

	// ensure finalizer
	updated, err := c.ensureFinalizer(propagationWork)
	if err != nil {
		klog.Infof("Failed to ensure finalizer for propagationwork %q", propagationWork.Name)
		return err
	} else if updated {
		return nil
	}

	err = c.dispatchPropagationWork(propagationWork)
	if err != nil {
		return err
	}

	klog.Infof("Sync propagationWork: %s/%s", propagationWork.Namespace, propagationWork.Name)
	return nil
}

// enqueueFoo takes a resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than PropagationWork.
func (c *Controller) enqueueEventResource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// IsMemberClusterReady checking readiness for the given member cluster
func (c *Controller) isMemberClusterReady(clusterStatus *v1alpha1.MemberClusterStatus) bool {
	for _, condition := range clusterStatus.Conditions {
		if condition.Type == "ClusterReady" {
			if condition.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// IsResourceExist checking weather resource exist in host cluster
func (c *Controller) isResourceApplied(propagationWorkStatus *pagationstrategy.PropagationWorkStatus) bool {
	for _, condition := range propagationWorkStatus.Conditions {
		if condition.Type == "Applied" {
			if condition.Status == v1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func (c *Controller) deletePropagationWork(propagationWork *pagationstrategy.PropagationWork) error {
	// TODO(RainbowMango): retrieve member cluster from the local cache instead of a real request to API server.
	membercluster, err := c.karmadaClientSet.MemberclusterV1alpha1().MemberClusters(memberClusterNS).Get(context.TODO(), propagationWork.Namespace, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get status of the given member cluster")
		return err
	}

	if !c.isMemberClusterReady(&membercluster.Status) {
		klog.Errorf("The status of the given member cluster is unready")
		return fmt.Errorf("cluster %s not ready, requeuing operation until cluster state is ready", membercluster.Name)
	}

	memberclusterDynamicClient, err := util.NewClusterDynamicClientSet(membercluster, c.kubeClientSet, membercluster.Spec.SecretRef.Namespace)
	if err != nil {
		c.eventRecorder.Eventf(membercluster, corev1.EventTypeWarning, "MalformedClusterConfig", err.Error())
		return err
	}

	for _, manifest := range propagationWork.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal workload, error is: %v", err)
			return err
		}

		err = c.deleteResource(memberclusterDynamicClient, workload)
		if err != nil {
			klog.Errorf("Failed to delete resource in the given member cluster, err is %v", err)
			return err
		}
	}

	return nil
}

func (c *Controller) dispatchPropagationWork(propagationWork *pagationstrategy.PropagationWork) error {
	// TODO(RainbowMango): retrieve member cluster from the local cache instead of a real request to API server.
	membercluster, err := c.karmadaClientSet.MemberclusterV1alpha1().MemberClusters(memberClusterNS).Get(context.TODO(), propagationWork.Namespace, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get status of the given member cluster")
		return err
	}

	if !c.isMemberClusterReady(&membercluster.Status) {
		klog.Errorf("The status of the given member cluster is unready")
		return fmt.Errorf("cluster %s not ready, requeuing operation until cluster state is ready", membercluster.Name)
	}

	err = c.syncToMemberClusters(membercluster, propagationWork)
	if err != nil {
		klog.Infof("Failed to delete propagationwork %v, err is %v", propagationWork.Name, err)
		return err
	}

	return nil
}

// syncToMemberClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToMemberClusters(membercluster *v1alpha1.MemberCluster, propagationWork *pagationstrategy.PropagationWork) error {
	memberclusterDynamicClient, err := util.NewClusterDynamicClientSet(membercluster, c.kubeClientSet, membercluster.Spec.SecretRef.Namespace)
	if err != nil {
		c.eventRecorder.Eventf(membercluster, corev1.EventTypeWarning, "MalformedClusterConfig", err.Error())
		return err
	}

	for _, manifest := range propagationWork.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("failed to unmarshal workload, error is: %v", err)
			return err
		}

		applied := c.isResourceApplied(&propagationWork.Status)
		if applied {
			err = c.updateResource(memberclusterDynamicClient, workload)
			if err != nil {
				klog.Errorf("Failed to update resource in the given member cluster, err is %v", err)
				return err
			}
		} else {
			err = c.createResource(memberclusterDynamicClient, workload)
			if err != nil {
				klog.Errorf("Failed to create resource in the given member cluster,err is %v", err)
				return err
			}

			err := c.updateAppliedCondition(propagationWork)
			if err != nil {
				klog.Errorf("Failed to update applied status for given propagationwork %v, err is %v", propagationWork.Name, err)
				return err
			}
		}
	}
	return nil
}

// deleteResource delete resource in member cluster
func (c *Controller) deleteResource(memberclusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	// start delete resource in member cluster
	groupVersion, err := schema.ParseGroupVersion(workload.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("can't get parse groupVersion[namespace: %s name: %s kind: %s]. error: %v", workload.GetNamespace(),
			workload.GetName(), util.ResourceKindMap[workload.GetKind()], err)
	}
	dynamicResource := schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: util.ResourceKindMap[workload.GetKind()]}
	err = memberclusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Delete(context.TODO(), workload.GetName(), v1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		klog.Infof("Failed to delete resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// createResource create resource in member cluster
func (c *Controller) createResource(memberclusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	// start create resource in member cluster
	groupVersion, err := schema.ParseGroupVersion(workload.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("can't get parse groupVersion[namespace: %s name: %s kind: %s]. error: %v", workload.GetNamespace(),
			workload.GetName(), util.ResourceKindMap[workload.GetKind()], err)
	}
	dynamicResource := schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: util.ResourceKindMap[workload.GetKind()]}
	_, err = memberclusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Create(context.TODO(), workload, v1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		klog.Infof("Failed to create resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// updateResource update resource in member cluster
func (c *Controller) updateResource(memberclusterDynamicClient *util.DynamicClusterClient, workload *unstructured.Unstructured) error {
	// start update resource in member cluster
	groupVersion, err := schema.ParseGroupVersion(workload.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("can't get parse groupVersion[namespace: %s name: %s kind: %s]. error: %v", workload.GetNamespace(),
			workload.GetName(), util.ResourceKindMap[workload.GetKind()], err)
	}
	dynamicResource := schema.GroupVersionResource{Group: groupVersion.Group, Version: groupVersion.Version, Resource: util.ResourceKindMap[workload.GetKind()]}
	_, err = memberclusterDynamicClient.DynamicClientSet.Resource(dynamicResource).Namespace(workload.GetNamespace()).Update(context.TODO(), workload, v1.UpdateOptions{})
	if err != nil {
		klog.Infof("Failed to update resource %v, err is %v ", workload.GetName(), err)
		return err
	}
	return nil
}

// removeFinalizer remove finalizer from the given propagationWork
func (c *Controller) removeFinalizer(propagationWork *pagationstrategy.PropagationWork) error {
	accessor, err := meta.Accessor(propagationWork)
	if err != nil {
		return err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if !finalizers.Has(finalizer) {
		return nil
	}
	finalizers.Delete(finalizer)
	accessor.SetFinalizers(finalizers.List())
	_, err = c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(propagationWork.Namespace).Update(context.TODO(), propagationWork, v1.UpdateOptions{})
	return err
}

// ensureFinalizer ensure finalizer for the given PropagationWork
func (c *Controller) ensureFinalizer(propagationWork *pagationstrategy.PropagationWork) (bool, error) {
	accessor, err := meta.Accessor(propagationWork)
	if err != nil {
		return false, err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if finalizers.Has(finalizer) {
		return false, nil
	}
	finalizers.Insert(finalizer)
	accessor.SetFinalizers(finalizers.List())
	_, err = c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(propagationWork.Namespace).Update(context.TODO(), propagationWork, v1.UpdateOptions{})
	return true, err
}

// updateAppliedCondition update the Applied condition for the given PropagationWork
func (c *Controller) updateAppliedCondition(propagationWork *pagationstrategy.PropagationWork) error {
	currentTime := v1.Now()
	propagationWorkApplied := "Applied"
	appliedSuccess := "AppliedSuccess"
	resourceApplied := "Success sync resource in member cluster"
	newPropagationWorkAppliedCondition := v1.Condition{
		Type:               propagationWorkApplied,
		Status:             v1.ConditionTrue,
		Reason:             appliedSuccess,
		Message:            resourceApplied,
		LastTransitionTime: currentTime,
	}
	propagationWork.Status.Conditions = append(propagationWork.Status.Conditions, newPropagationWorkAppliedCondition)
	_, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(propagationWork.Namespace).Update(context.TODO(), propagationWork, v1.UpdateOptions{})
	return err
}
