package search

import (
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

type clusterRegistry struct {
	registries map[string]struct{}
	resources  map[schema.GroupVersionResource]struct{}
}

func (c *clusterRegistry) unregistry() bool {
	return len(c.registries) == 0
}

// Controller ResourceRegistry controller
type Controller struct {
	restConfig      *rest.Config
	restMapper      meta.RESTMapper
	informerFactory informerfactory.SharedInformerFactory
	clusterLister   clusterlister.ClusterLister
	queue           workqueue.RateLimitingInterface

	clusterRegistry sync.Map

	resourceHandler cache.ResourceEventHandler
	InformerManager informermanager.MultiClusterInformerManager
}

// NewController returns a new ResourceRegistry controller
func NewController(restConfig *rest.Config, rh cache.ResourceEventHandler) (*Controller, error) {
	karmadaClient := karmadaclientset.NewForConfigOrDie(restConfig)
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	restMapper, err := apiutil.NewDynamicRESTMapper(restConfig)
	if err != nil {
		klog.Errorf("Failed to create REST mapper: %v", err)
		return nil, err
	}

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Controller{
		restConfig:      restConfig,
		informerFactory: factory,
		clusterLister:   clusterLister,
		queue:           queue,
		restMapper:      restMapper,

		resourceHandler: rh,
		InformerManager: informermanager.GetInstance(),
	}
	c.addAllEventHandlers()
	return c, nil
}

// addAllEventHandlers adds all event handlers to the informer
func (c *Controller) addAllEventHandlers() {
	clusterInformer := c.informerFactory.Cluster().V1alpha1().Clusters().Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addCluster,
		UpdateFunc: c.updateCluster,
		DeleteFunc: c.deleteCluster,
	})

	resourceRegistryInformer := c.informerFactory.Search().V1alpha1().ResourceRegistries().Informer()
	resourceRegistryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addResourceRegistry,
		UpdateFunc: c.updateResourceRegistry,
		DeleteFunc: c.deleteResourceRegistry,
	})
}

// Start the controller
func (c *Controller) Start(stopCh <-chan struct{}) {
	klog.Infof("Starting karmada search controller")

	defer runtime.HandleCrash()

	c.informerFactory.Start(stopCh)
	c.informerFactory.WaitForCacheSync(stopCh)

	go wait.Until(c.worker, time.Second, stopCh)

	go func() {
		<-stopCh
		informermanager.StopInstance()
		klog.Infof("Shutting down karmada search controller")
	}()
}

// worker processes the queue of resourceRegistry objects.
func (c *Controller) worker() {
	for c.cacheNext() {
	}
}

// cacheNext processes the next cluster object in the queue.
func (c *Controller) cacheNext() bool {
	// Wait until there is a new item in the working queue
	key, shutdown := c.queue.Get()
	if shutdown {
		klog.Errorf("Fail to pop item from queue")
		return false
	}

	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	err := c.doCacheCluster(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	klog.Errorf("Error cache memeber cluster %v, %v", key, err)
	c.queue.AddRateLimited(key)
}

// doCacheCluster processes the resourceRegistry object
// TODO: update status
func (c *Controller) doCacheCluster(cluster string) error {
	// STEP0:  stop informer manager for the cluster which is not referenced by any `SearchRegistry` object.
	v, ok := c.clusterRegistry.Load(cluster)
	if !ok {
		klog.Infof("Cluster %s is not registered", cluster)
		return nil
	}

	cr := v.(clusterRegistry)
	if cr.unregistry() {
		klog.Infof("try to stop cluster informer %s", cluster)
		c.InformerManager.Stop(cluster)
		return nil
	}

	// STEP1: stop informer manager for the cluster which does not exist anymore.
	cls, err := c.clusterLister.Get(cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("try to stop cluster informer %s", cluster)
			c.InformerManager.Stop(cluster)
			return nil
		}
		return err
	}

	if !cls.DeletionTimestamp.IsZero() {
		klog.Infof("try to stop cluster informer %s", cluster)
		c.InformerManager.Stop(cluster)
		return nil
	}

	// STEP2: added/updated cluster, builds an informer manager for a specific cluster.
	if !c.InformerManager.IsManagerExist(cluster) {
		klog.Info("try to build informer manager for cluster ", cluster)
		controlPlaneClient := gclient.NewForConfigOrDie(c.restConfig)

		clusterDynamicClient, err := util.NewClusterDynamicClientSet(cluster, controlPlaneClient)
		if err != nil {
			return err
		}
		_ = c.InformerManager.ForCluster(cluster, clusterDynamicClient.DynamicClientSet, 0)
	}

	if c.resourceHandler != nil {
		sci := c.InformerManager.GetSingleClusterManager(cluster)
		for gvr := range cr.resources {
			klog.Infof("try to start informer for %s, %v", cluster, gvr)
			// TODO: gvr exists check
			sci.ForResource(gvr, c.resourceHandler)
		}
		sci.Start()
		_ = sci.WaitForCacheSync()
	}

	return nil
}

// addResourceRegistry parse the resourceRegistry object and add Cluster to the queue
func (c *Controller) addResourceRegistry(obj interface{}) {
	rr := obj.(*v1alpha1.ResourceRegistry)
	resources := c.getResources(rr.Spec.ResourceSelectors)

	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
		v, _ := c.clusterRegistry.LoadOrStore(cluster, clusterRegistry{
			resources:  make(map[schema.GroupVersionResource]struct{}),
			registries: make(map[string]struct{})})
		cr := v.(clusterRegistry)

		for _, r := range resources {
			cr.resources[r] = struct{}{}
		}
		cr.registries[rr.GetName()] = struct{}{}
		c.clusterRegistry.Store(cluster, cr)

		c.queue.Add(cluster)
	}
}

// updateResourceRegistry parse the resourceRegistry object and add (added/deleted) Cluster to the queue
func (c *Controller) updateResourceRegistry(oldObj, newObj interface{}) {
	oldRR := oldObj.(*v1alpha1.ResourceRegistry)
	newRR := newObj.(*v1alpha1.ResourceRegistry)

	// TODO: stop resource informers if it is not in the new resource registry
	resources := c.getResources(newRR.Spec.ResourceSelectors)

	clusters := c.getClusters(newRR.Spec.TargetCluster)
	clusterSets := make(map[string]struct{})

	for _, cls := range clusters {
		v, _ := c.clusterRegistry.LoadOrStore(cls, clusterRegistry{
			resources:  make(map[schema.GroupVersionResource]struct{}),
			registries: make(map[string]struct{})})
		cr := v.(clusterRegistry)

		for _, r := range resources {
			cr.resources[r] = struct{}{}
		}
		cr.registries[newRR.GetName()] = struct{}{}
		c.clusterRegistry.Store(cls, cr)

		clusterSets[cls] = struct{}{}
		c.queue.Add(cls)
	}

	for _, cls := range c.getClusters(oldRR.Spec.TargetCluster) {
		if _, ok := clusterSets[cls]; ok {
			continue
		}

		v, ok := c.clusterRegistry.Load(cls)
		if !ok {
			continue
		}

		cr := v.(clusterRegistry)
		delete(cr.registries, oldRR.GetName())
		c.clusterRegistry.Store(cls, cr)

		c.queue.Add(cls)
	}
}

// deleteResourceRegistry parse the resourceRegistry object and add deleted Cluster to the queue
func (c *Controller) deleteResourceRegistry(obj interface{}) {
	rr := obj.(*v1alpha1.ResourceRegistry)

	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
		v, ok := c.clusterRegistry.Load(cluster)
		if !ok {
			return
		}
		cr := v.(clusterRegistry)
		delete(cr.registries, rr.GetName())
		c.clusterRegistry.Store(cluster, cr)

		c.queue.Add(cluster)
	}
}

// addCluster adds a cluster object to the queue if needed
func (c *Controller) addCluster(obj interface{}) {
	cluster, ok := obj.(*clusterV1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert to *clusterV1alpha1.Cluster: %v", obj)
		return
	}

	_, ok = c.clusterRegistry.Load(cluster.GetName())
	if ok {
		// unregistered cluster, do nothing.
		return
	}

	c.queue.Add(cluster.GetName())
}

// updateCluster TODO: rebuild informer if Cluster.Spec is changed
func (c *Controller) updateCluster(oldObj, newObj interface{}) {}

// deleteCluster set cluster to not exists
func (c *Controller) deleteCluster(obj interface{}) {
	cluster, ok := obj.(*clusterV1alpha1.Cluster)
	if !ok {
		klog.Errorf("cannot convert to *clusterV1alpha1.Cluster: %v", obj)
		return
	}

	_, ok = c.clusterRegistry.Load(cluster.GetName())
	if !ok {
		// unregistered cluster, do nothing.
		return
	}

	c.queue.Add(cluster.GetName())
}

// getClusterAndResource returns the cluster and resources from the resourceRegistry object
func (c *Controller) getClusters(affinity policyv1alpha1.ClusterAffinity) []string {
	clusters := make([]string, 0)
	lst, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("failed to list clusters: %v", err)
		return clusters
	}
	for _, cls := range lst {
		if util.ClusterMatches(cls, affinity) {
			clusters = append(clusters, cls.GetName())
		}
	}
	return clusters
}

// getClusterAndResource returns the cluster and resources from the resourceRegistry object
func (c *Controller) getResources(selectors []v1alpha1.ResourceSelector) []schema.GroupVersionResource {
	resources := make([]schema.GroupVersionResource, 0)
	for _, rs := range selectors {
		gvr, err := restmapper.GetGroupVersionResource(
			c.restMapper, schema.FromAPIVersionAndKind(rs.APIVersion, rs.Kind),
		)
		if err != nil {
			klog.Errorf("failed to get gvr: %v", err)
			continue
		}
		resources = append(resources, gvr)
	}
	return resources
}

// CachedResourceHandler is the default handler for resource events
func CachedResourceHandler() cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			us, ok := obj.(*unstructured.Unstructured)
			if !ok {
				klog.Errorf("cannot convert to Unstructured: %v", obj)
			}
			klog.V(4).Infof("add resource %s, %s, %s, %s", us.GetAPIVersion(), us.GetKind(), us.GetNamespace(), us.GetName())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			us, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				klog.Errorf("cannot convert to Unstructured: %v", newObj)
			}
			klog.V(4).Infof("update resource %s, %s, %s, %s", us.GetAPIVersion(), us.GetKind(), us.GetNamespace(), us.GetName())
		},
		DeleteFunc: func(obj interface{}) {
			us, ok := obj.(*unstructured.Unstructured)
			if !ok {
				klog.Errorf("cannot convert to Unstructured: %v", obj)
			}
			klog.V(4).Infof("delete resource %s, %s, %s, %s", us.GetAPIVersion(), us.GetKind(), us.GetNamespace(), us.GetName())
		},
	}
}
