package binding

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	clientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
	karmadaScheme "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned/scheme"
	informers "github.com/huawei-cloudnative/karmada/pkg/generated/informers/externalversions"
	listers "github.com/huawei-cloudnative/karmada/pkg/generated/listers/propagationstrategy/v1alpha1"
)

const (
	controllerAgentName = "binding-controller"
)

// Controller is the controller implementation for binding resources
type Controller struct {
	// karmadaClientSet is the clientset for our own API group.
	karmadaClientSet clientset.Interface

	// kubeClientSet is a standard kubernetes clientset.
	kubeClientSet            kubernetes.Interface
	dynamicClientSet         dynamic.Interface
	karmadaInformerFactory   informers.SharedInformerFactory
	propagationBindingLister listers.PropagationBindingLister
	propagationBindingSynced cache.InformerSynced
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

// StartPropagationBindingController starts a new binding controller.
func StartPropagationBindingController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	controller, err := newPropagationBindingController(config)
	if err != nil {
		return err
	}
	klog.Infof("Starting PropagationBinding controller")

	go wait.Until(func() {
		if err := controller.Run(1, stopChan); err != nil {
			klog.Errorf("controller exit unexpected! will restart later, controller: %s, error: %v", controllerAgentName, err)
		}
	}, 1*time.Second, stopChan)

	return nil
}

// newPropagationBindingController returns a new controller.
func newPropagationBindingController(config *util.ControllerConfig) (*Controller, error) {
	headClusterConfig := rest.CopyConfig(config.HeadClusterConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(headClusterConfig)

	karmadaClientSet := clientset.NewForConfigOrDie(headClusterConfig)
	dynamicClientSet, err := dynamic.NewForConfig(headClusterConfig)
	if err != nil {
		return nil, err
	}
	karmadaInformerFactory := informers.NewSharedInformerFactory(karmadaClientSet, 0)
	propagationBindingInformer := karmadaInformerFactory.Propagationstrategy().V1alpha1().PropagationBindings()
	// Add karmada types to the default Kubernetes Scheme so Events can be logged for karmada types.
	utilruntime.Must(karmadaScheme.AddToScheme(scheme.Scheme))

	// Create event broadcaster
	klog.V(1).Infof("Creating event broadcaster for %s", controllerAgentName)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})

	controller := &Controller{
		karmadaClientSet:         karmadaClientSet,
		kubeClientSet:            kubeClientSet,
		dynamicClientSet:         dynamicClientSet,
		karmadaInformerFactory:   karmadaInformerFactory,
		propagationBindingLister: propagationBindingInformer.Lister(),
		propagationBindingSynced: propagationBindingInformer.Informer().HasSynced,
		workqueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		eventRecorder:            eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}

	klog.Info("Setting up event handlers")
	propagationBindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("Received add event. just add to queue.")
			controller.enqueueEventResource(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			klog.Infof("Received update event. just add to queue.")
			controller.enqueueEventResource(new)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("Received delete event. Do delete action.")
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
	if ok := cache.WaitForCacheSync(stopCh, c.propagationBindingSynced); !ok {
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
	propagationBinding, err := c.propagationBindingLister.PropagationBindings(namespace).Get(name)
	if err != nil {
		// The propagationBinding resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("propagationBinding '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	klog.V(2).Infof("Sync propagationBinding: %s/%s", propagationBinding.Namespace, propagationBinding.Name)
	err = c.transformBindingToWorks(propagationBinding)
	if err != nil {
		klog.Errorf("failed to transform propagationBinding %s/%s to propagationWorks. error: %+v",
			propagationBinding.Namespace, propagationBinding.Name, err)
		return err
	}
	return nil
}

// get clusterName list from bind clusters field
func (c *Controller) getBindingClusterNames(propagationBinding *v1alpha1.PropagationBinding) []string {
	var clusterNames []string
	for _, targetCluster := range propagationBinding.Spec.Clusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// delete irrelevant field from workload. such as uid, timestamp, status
func (c *Controller) removeIrrelevantField(workload *unstructured.Unstructured) {
	unstructured.RemoveNestedField(workload.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(workload.Object, "metadata", "generation")
	unstructured.RemoveNestedField(workload.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(workload.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(workload.Object, "metadata", "uid")
	unstructured.RemoveNestedField(workload.Object, "status")
}

// transform propagationBinding resource to propagationWork resources
func (c *Controller) transformBindingToWorks(propagationBinding *v1alpha1.PropagationBinding) error {
	workload, err := util.GetResourceStructure(c.dynamicClientSet, propagationBinding.Spec.Resource.APIVersion,
		propagationBinding.Spec.Resource.Kind, propagationBinding.Spec.Resource.Namespace, propagationBinding.Spec.Resource.Name)
	if err != nil {
		klog.Errorf("failed to get resource. error: %v", err)
		return err
	}

	clusterNames := c.getBindingClusterNames(propagationBinding)

	for _, clusterNameMirrorNamespace := range clusterNames {
		c.removeIrrelevantField(workload)
		formatWorkload, err := workload.MarshalJSON()
		if err != nil {
			klog.Errorf("failed to marshal workload. error: %v", err)
			return err
		}

		rawExtension := runtime.RawExtension{
			Raw: formatWorkload,
		}

		propagationWork := v1alpha1.PropagationWork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      propagationBinding.Name,
				Namespace: clusterNameMirrorNamespace,
			},
			Spec: v1alpha1.PropagationWorkSpec{
				Workload: v1alpha1.WorkloadTemplate{
					Manifests: []v1alpha1.Manifest{
						{
							RawExtension: rawExtension,
						},
					},
				},
			},
		}

		workGetResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(clusterNameMirrorNamespace).Get(context.TODO(), propagationWork.Name, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			workCreateResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(clusterNameMirrorNamespace).Create(context.TODO(), &propagationWork, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("failed to create propagationWork %s/%s. error: %v", propagationWork.Namespace, propagationWork.Name, err)
				return err
			}
			klog.Infof("create propagationWork %s/%s success", propagationWork.Namespace, propagationWork.Name)
			klog.V(2).Infof("create propagationWork: %+v", workCreateResult)
			return nil
		} else if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("failed to get propagationWork %s/%s. error: %v", propagationWork.Namespace, propagationWork.Name, err)
			return err
		}
		workGetResult.Spec = propagationWork.Spec
		workUpdateResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationWorks(clusterNameMirrorNamespace).Update(context.TODO(), workGetResult, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update propagationWork %s/%s. error: %v", propagationWork.Namespace, propagationWork.Name, err)
			return err
		}
		klog.Infof("update propagationWork %s/%s success", propagationWork.Namespace, propagationWork.Name)
		klog.V(2).Infof("update propagationWork: %+v", workUpdateResult)
	}

	return nil
}

// enqueueFoo takes a resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than propagationBinding.
func (c *Controller) enqueueEventResource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
