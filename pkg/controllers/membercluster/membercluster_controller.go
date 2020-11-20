package membercluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	"github.com/huawei-cloudnative/karmada/pkg/controllers/util"
	clientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
	karmadakubecheme "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned/scheme"
	informers "github.com/huawei-cloudnative/karmada/pkg/generated/informers/externalversions"
	listers "github.com/huawei-cloudnative/karmada/pkg/generated/listers/membercluster/v1alpha1"
)

const controllerAgentName = "membercluster-controller"

// Controller is the controller implementation for membercluster resources
type Controller struct {
	// karmadaClientSet is the clientset for our own API group.
	karmadaClientSet clientset.Interface

	// kubeClientSet is a standard kubernetes clientset.
	kubeClientSet kubernetes.Interface

	karmadaInformerFactory informers.SharedInformerFactory
	memberclusterLister    listers.MemberClusterLister
	memberclusterSynced    cache.InformerSynced

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

// StartMemberClusterController starts a new cluster controller.
func StartMemberClusterController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	controller, err := newMemberClusterController(config)
	if err != nil {
		return err
	}
	klog.Infof("Starting member cluster controller")

	go wait.Until(func() {
		if err := controller.Run(2, stopChan); err != nil {
			klog.Errorf("controller exit unexpected! will restart later, controller: %s, error: %v", controllerAgentName, err)
		}
	}, 1*time.Second, stopChan)

	return nil
}

// newMemberClusterController returns a new controller.
func newMemberClusterController(config *util.ControllerConfig) (*Controller, error) {

	headClusterConfig := rest.CopyConfig(config.HeadClusterConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(headClusterConfig)

	karmadaClientSet := clientset.NewForConfigOrDie(headClusterConfig)
	karmadaInformerFactory := informers.NewSharedInformerFactory(karmadaClientSet, 10*time.Second)
	memberclusterInformer := karmadaInformerFactory.Membercluster().V1alpha1().MemberClusters()

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
		memberclusterLister:    memberclusterInformer.Lister(),
		memberclusterSynced:    memberclusterInformer.Informer().HasSynced,
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		eventRecorder:          eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}

	klog.Info("Setting up event handlers")
	memberclusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueueEventResource(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueEventResource(new)
		},
		DeleteFunc: func(obj interface{}) {
			castObj, ok := obj.(*v1alpha1.MemberCluster)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					klog.Errorf("Couldn't get object from tombstone %#v", obj)
					return
				}
				castObj, ok = tombstone.Obj.(*v1alpha1.MemberCluster)
				if !ok {
					klog.Errorf("Tombstone contained object that is not expected %#v", obj)
					return
				}
			}

			// TODO: postpone delete namespace if there is any work object which should be deleted by binding controller.

			// delete member cluster workspace when member cluster unjoined
			if err := controller.kubeClientSet.CoreV1().Namespaces().Delete(context.TODO(), castObj.Name, v1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					klog.Errorf("Error while deleting namespace %s: %s", castObj.Name, err)
					return
				}
			}
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
	if ok := cache.WaitForCacheSync(stopCh, c.memberclusterSynced); !ok {
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
	membercluster, err := c.memberclusterLister.MemberClusters(namespace).Get(name)
	if err != nil {
		// The membercluster resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("membercluster '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// create member cluster workspace when member cluster joined
	_, err = c.kubeClientSet.CoreV1().Namespaces().Get(context.TODO(), membercluster.Name, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			memberclusterNS := &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: membercluster.Name,
				},
			}
			_, err = c.kubeClientSet.CoreV1().Namespaces().Create(context.TODO(), memberclusterNS, v1.CreateOptions{})
			if err != nil {

				return err
			}
		} else {
			klog.V(2).Infof("Could not get %s namespace: %v", membercluster.Name, err)
			return err
		}
	}

	// create a ClusterClient for the given member cluster
	memberclusterClient, err := NewClusterClientSet(membercluster, c.kubeClientSet, membercluster.Spec.SecretRef.Namespace)
	if err != nil {
		c.eventRecorder.Eventf(membercluster, corev1.EventTypeWarning, "MalformedClusterConfig", err.Error())
		return err
	}

	// update status of the given member cluster
	// TODO(RainbowMango): need to check errors and return error if update status failed.
	updateIndividualClusterStatus(membercluster, c.karmadaClientSet, memberclusterClient)

	return nil
}

// enqueueFoo takes a resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than membercluster.
func (c *Controller) enqueueEventResource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
