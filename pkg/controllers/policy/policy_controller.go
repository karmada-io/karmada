package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

const controllerAgentName = "policy-controller"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("PropagationPolicy")

// Controller is the controller implementation for policy resources
type Controller struct {
	// karmadaClientSet is the clientset for our own API group.
	karmadaClientSet clientset.Interface

	// kubeClientSet is a standard kubernetes clientset.
	kubeClientSet    kubernetes.Interface
	dynamicClientSet dynamic.Interface

	karmadaInformerFactory  informers.SharedInformerFactory
	propagationPolicyLister listers.PropagationPolicyLister
	propagationPolicySynced cache.InformerSynced
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

// StartPropagationPolicyController starts a new policy controller.
func StartPropagationPolicyController(config *util.ControllerConfig, stopChan <-chan struct{}) error {
	controller, err := newPropagationPolicyController(config)
	if err != nil {
		return err
	}
	klog.Infof("Starting PropagationPolicy controller")

	go wait.Until(func() {
		if err := controller.Run(1, stopChan); err != nil {
			klog.Errorf("controller exit unexpected! will restart later, controller: %s, error: %v", controllerAgentName, err)
		}
	}, 1*time.Second, stopChan)

	return nil
}

// newPropagationPolicyController returns a new controller.
func newPropagationPolicyController(config *util.ControllerConfig) (*Controller, error) {
	headClusterConfig := rest.CopyConfig(config.HeadClusterConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(headClusterConfig)
	dynamicClientSet, err := dynamic.NewForConfig(headClusterConfig)
	if err != nil {
		return nil, err
	}
	karmadaClientSet := clientset.NewForConfigOrDie(headClusterConfig)
	karmadaInformerFactory := informers.NewSharedInformerFactory(karmadaClientSet, 0)
	propagationPolicyInformer := karmadaInformerFactory.Propagationstrategy().V1alpha1().PropagationPolicies()
	// Add karmada types to the default Kubernetes Scheme so Events can be logged for karmada types.
	utilruntime.Must(karmadaScheme.AddToScheme(scheme.Scheme))

	// Create event broadcaster
	klog.V(1).Infof("Creating event broadcaster for %s", controllerAgentName)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})

	controller := &Controller{
		karmadaClientSet:        karmadaClientSet,
		kubeClientSet:           kubeClientSet,
		dynamicClientSet:        dynamicClientSet,
		karmadaInformerFactory:  karmadaInformerFactory,
		propagationPolicyLister: propagationPolicyInformer.Lister(),
		propagationPolicySynced: propagationPolicyInformer.Informer().HasSynced,
		workqueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerAgentName),
		eventRecorder:           eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}

	klog.Info("Setting up event handlers")
	propagationPolicyInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("Received add event. just add to queue.")
			controller.enqueueEventResource(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			klog.Infof("Received update event. just add to queue.")
			controller.enqueueEventResource(new)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("Received delete event. Do nothing just log.")
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
	if ok := cache.WaitForCacheSync(stopCh, c.propagationPolicySynced); !ok {
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

// fetchWorkloads query matched kubernetes resource by resourceSelector
func (c *Controller) fetchWorkloads(resourceSelectors []v1alpha1.ResourceSelector) ([]*unstructured.Unstructured, error) {
	var workloads []*unstructured.Unstructured
	// todo: if resources repetitive, deduplication.
	// todo: if namespaces, names, labelSelector is nil, need to do something
	for _, resourceSelector := range resourceSelectors {
		matchNamespaces := util.GetMatchItems(resourceSelector.Namespaces, resourceSelector.ExcludeNamespaces)
		deduplicationNames := util.GetDeduplicationArray(resourceSelector.Names)
		for _, namespace := range matchNamespaces {
			if resourceSelector.LabelSelector == nil {
				err := c.fetchWorkloadsWithOutLabelSelector(resourceSelector, namespace, deduplicationNames, &workloads)
				if err != nil {
					klog.Errorf("failed to fetch workloads by names in namespace %s. error: %v", namespace, err)
					return nil, err
				}
			} else {
				err := c.fetchWorkloadsWithLabelSelector(resourceSelector, namespace, deduplicationNames, &workloads)
				if err != nil {
					klog.Errorf("failed to fetch workloads with labelSelector in namespace %s. error: %v", namespace, err)
					return nil, err
				}
			}
		}
	}
	return workloads, nil
}

// fetchWorkloadsWithOutLabelSelector query workloads by names
func (c *Controller) fetchWorkloadsWithOutLabelSelector(resourceSelector v1alpha1.ResourceSelector, namespace string, names []string, workloads *[]*unstructured.Unstructured) error {
	for _, name := range names {
		workload, err := util.GetResourceStructure(c.dynamicClientSet, resourceSelector.APIVersion,
			resourceSelector.Kind, namespace, name)
		if err != nil {
			return err
		}
		*workloads = append(*workloads, workload)
	}
	return nil
}

// fetchWorkloadsWithLabelSelector query workloads by labelSelector and names
func (c *Controller) fetchWorkloadsWithLabelSelector(resourceSelector v1alpha1.ResourceSelector, namespace string, names []string, workloads *[]*unstructured.Unstructured) error {
	unstructuredWorkLoadList, err := util.GetResourcesStructureByFilter(c.dynamicClientSet, resourceSelector.APIVersion,
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

// getTargetClusters get targetClusters by placement
func (c *Controller) getTargetClusters(placement v1alpha1.Placement) []v1alpha1.TargetCluster {
	matchClusterNames := util.GetMatchItems(placement.ClusterAffinity.ClusterNames, placement.ClusterAffinity.ExcludeClusters)

	// todo: cluster labelSelector, fieldSelector, clusterTolerations
	// todo: calc spread contraints. such as maximumClusters, minimumClusters
	var targetClusters []v1alpha1.TargetCluster
	for _, matchClusterName := range matchClusterNames {
		targetClusters = append(targetClusters, v1alpha1.TargetCluster{Name: matchClusterName})
	}
	return targetClusters
}

// transform propagationPolicy to propagationBindings
func (c *Controller) transformPolicyToBinding(propagationPolicy *v1alpha1.PropagationPolicy) error {
	workloads, err := c.fetchWorkloads(propagationPolicy.Spec.ResourceSelectors)
	if err != nil {
		return err
	}

	targetCluster := c.getTargetClusters(propagationPolicy.Spec.Placement)

	for _, workload := range workloads {
		err := c.ensurePropagationBinding(propagationPolicy, workload, targetCluster)
		if err != nil {
			return err
		}
	}
	return nil
}

// create propagationBinding
func (c *Controller) ensurePropagationBinding(propagationPolicy *v1alpha1.PropagationPolicy, workload *unstructured.Unstructured, clusterNames []v1alpha1.TargetCluster) error {
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

	bindingGetResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Get(context.TODO(), propagationBinding.Name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		bindingCreateResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Create(context.TODO(), &propagationBinding, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create propagationBinding %s/%s. error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
			return err
		}
		klog.Infof("create propagationBinding %s/%s success", propagationBinding.Namespace, propagationBinding.Name)
		klog.V(2).Infof("create propagationBinding: %+v", bindingCreateResult)
		return nil
	} else if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("failed to get propagationBinding %s/%s. error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
		return err
	}
	bindingGetResult.Spec = propagationBinding.Spec
	bindingGetResult.ObjectMeta.OwnerReferences = propagationBinding.ObjectMeta.OwnerReferences
	bindingUpdateResult, err := c.karmadaClientSet.PropagationstrategyV1alpha1().PropagationBindings(propagationBinding.Namespace).Update(context.TODO(), bindingGetResult, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("failed to update propagationBinding %s/%s. error: %v", propagationBinding.Namespace, propagationBinding.Name, err)
		return err
	}
	klog.Infof("update propagationBinding %s/%s success", propagationBinding.Namespace, propagationBinding.Name)
	klog.V(2).Infof("update propagationBinding: %+v", bindingUpdateResult)

	return nil
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
	propagationPolicy, err := c.propagationPolicyLister.PropagationPolicies(namespace).Get(name)
	if err != nil {
		// The propagationPolicy resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("propagationPolicy '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	klog.V(2).Infof("Sync propagationPolicy: %s/%s", propagationPolicy.Namespace, propagationPolicy.Name)
	err = c.transformPolicyToBinding(propagationPolicy)
	if err != nil {
		klog.Errorf("failed to transform propagationPolicy %s/%s to propagationBindings. error: %+v",
			propagationPolicy.Namespace, propagationPolicy.Name, err)
		return err
	}
	return nil
}

// enqueueFoo takes a resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than propagationPolicy.
func (c *Controller) enqueueEventResource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
