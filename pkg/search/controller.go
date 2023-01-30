package search

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/backendstore"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

type registrySet map[string]struct{}

type clusterRegistry struct {
	registries registrySet
	resources  map[schema.GroupVersionResource]registrySet
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

	InformerManager genericmanager.MultiClusterInformerManager
}

// NewController returns a new ResourceRegistry controller
func NewController(restConfig *rest.Config, factory informerfactory.SharedInformerFactory, restMapper meta.RESTMapper) (*Controller, error) {
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &Controller{
		restConfig:      restConfig,
		informerFactory: factory,
		clusterLister:   clusterLister,
		queue:           queue,
		restMapper:      restMapper,

		InformerManager: genericmanager.GetInstance(),
	}
	c.addAllEventHandlers()

	// TODO: leader election and full sync
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	backendstore.Init(cs)
	return c, nil
}

// addAllEventHandlers adds all event handlers to the informer
func (c *Controller) addAllEventHandlers() {
	clusterInformer := c.informerFactory.Cluster().V1alpha1().Clusters().Informer()
	_, err := clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addCluster,
		UpdateFunc: c.updateCluster,
		DeleteFunc: c.deleteCluster,
	})
	if err != nil {
		klog.Errorf("Failed to add handlers for Clusters: %v", err)
	}

	resourceRegistryInformer := c.informerFactory.Search().V1alpha1().ResourceRegistries().Informer()
	_, err = resourceRegistryInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addResourceRegistry,
		UpdateFunc: c.updateResourceRegistry,
		DeleteFunc: c.deleteResourceRegistry,
	})
	if err != nil {
		klog.Errorf("Failed to add handlers for Clusters: %v", err)
	}

	// ignore the error here because the informers haven't been started
	_ = clusterInformer.SetTransform(fedinformer.StripUnusedFields)
	_ = resourceRegistryInformer.SetTransform(fedinformer.StripUnusedFields)
}

// Start the controller
func (c *Controller) Start(stopCh <-chan struct{}) {
	klog.Infof("Starting karmada search controller")

	defer runtime.HandleCrash()

	c.informerFactory.WaitForCacheSync(stopCh)

	go wait.Until(c.worker, time.Second, stopCh)

	go func() {
		<-stopCh
		genericmanager.StopInstance()
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

	klog.Errorf("Error cache member cluster %v, %v", key, err)
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

	// STEP1: stop informer manager for the cluster which does not exist anymore or is not ready.
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

	if !util.IsClusterReady(&cls.Status) {
		klog.Warningf("cluster %s is notReady try to stop this cluster informer", cluster)
		c.InformerManager.Stop(cluster)
		return nil
	}

	// STEP2: added/updated cluster, builds an informer manager for a specific cluster.
	if !c.InformerManager.IsManagerExist(cluster) {
		klog.Info("Try to build informer manager for cluster ", cluster)
		controlPlaneClient := gclient.NewForConfigOrDie(c.restConfig)

		clusterDynamicClient, err := util.NewClusterDynamicClientSet(cluster, controlPlaneClient)
		if err != nil {
			return err
		}
		_ = c.InformerManager.ForCluster(cluster, clusterDynamicClient.DynamicClientSet, 0)
	}

	handler := backendstore.GetBackend(cluster).ResourceEventHandlerFuncs()
	if handler == nil {
		return fmt.Errorf("failed to get resource event handler for cluster %s", cluster)
	}

	sci := c.InformerManager.GetSingleClusterManager(cluster)
	for gvr, registries := range cr.resources {
		if len(registries) == 0 {
			// unregisted resource, do nothing.
			continue
		}

		klog.Infof("add informer for %s, %v", cluster, gvr)
		sci.ForResource(gvr, handler)
	}
	klog.Infof("start informer for %s", cluster)
	sci.Start()
	_ = sci.WaitForCacheSync()
	klog.Infof("start informer for %s done", cluster)

	return nil
}

// addResourceRegistry parse the resourceRegistry object and add Cluster to the queue
func (c *Controller) addResourceRegistry(obj interface{}) {
	rr := obj.(*searchv1alpha1.ResourceRegistry)
	resources := c.getResources(rr.Spec.ResourceSelectors)

	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
		v, _ := c.clusterRegistry.LoadOrStore(cluster, clusterRegistry{
			resources:  make(map[schema.GroupVersionResource]registrySet),
			registries: make(registrySet)})
		cr := v.(clusterRegistry)

		for _, r := range resources {
			if cr.resources[r] == nil {
				cr.resources[r] = make(registrySet)
			}
			// add registry record for the resource.
			cr.resources[r][rr.GetName()] = struct{}{}
		}
		// add registry record for the cluster.
		cr.registries[rr.GetName()] = struct{}{}
		c.clusterRegistry.Store(cluster, cr)

		// set backendstore
		backendstore.AddBackend(cluster, rr.Spec.BackendStore)

		c.queue.Add(cluster)
	}
}

// updateResourceRegistry parse the resourceRegistry object and add (added/deleted) Cluster to the queue
func (c *Controller) updateResourceRegistry(oldObj, newObj interface{}) {
	oldRR := oldObj.(*searchv1alpha1.ResourceRegistry)
	newRR := newObj.(*searchv1alpha1.ResourceRegistry)

	if reflect.DeepEqual(oldRR.Spec, newRR.Spec) {
		klog.V(4).Infof("Ignore ResourceRegistry(%s) update event as spec not changed", newRR.Name)
		return
	}

	// TODO: stop resource informers if it is not in the new resource registry
	resources := c.getResources(newRR.Spec.ResourceSelectors)

	clusters := c.getClusters(newRR.Spec.TargetCluster)
	clusterSets := make(map[string]struct{}, len(clusters))

	for _, cls := range clusters {
		v, _ := c.clusterRegistry.LoadOrStore(cls, clusterRegistry{
			resources:  make(map[schema.GroupVersionResource]registrySet),
			registries: make(registrySet)})
		cr := v.(clusterRegistry)

		for _, r := range resources {
			if cr.resources[r] == nil {
				cr.resources[r] = make(registrySet)
			}
			// add registry record for the resource.
			cr.resources[r][newRR.GetName()] = struct{}{}
		}
		// add registry record for the cluster.
		cr.registries[newRR.GetName()] = struct{}{}
		c.clusterRegistry.Store(cls, cr)

		clusterSets[cls] = struct{}{}

		// set backendstore
		backendstore.AddBackend(cls, newRR.Spec.BackendStore)

		c.queue.Add(cls)
	}

	for _, cls := range c.getClusters(oldRR.Spec.TargetCluster) {
		if _, ok := clusterSets[cls]; ok {
			// Cluster is in both old and new clusters, do nothing.
			continue
		}

		v, ok := c.clusterRegistry.Load(cls)
		if !ok {
			// unregisted cluster, do nothing.
			continue
		}

		cr := v.(clusterRegistry)
		delete(cr.registries, oldRR.GetName())
		if cr.unregistry() {
			cr.resources = make(map[schema.GroupVersionResource]registrySet)
		}
		c.clusterRegistry.Store(cls, cr)

		c.queue.Add(cls)
	}
}

// deleteResourceRegistry parse the resourceRegistry object and add deleted Cluster to the queue
func (c *Controller) deleteResourceRegistry(obj interface{}) {
	rr := obj.(*searchv1alpha1.ResourceRegistry)
	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
		v, ok := c.clusterRegistry.Load(cluster)
		if !ok {
			// unregisted cluster, do nothing.
			continue
		}

		cr := v.(clusterRegistry)
		// delete registry record for the cluster.
		delete(cr.registries, rr.GetName())
		if cr.unregistry() {
			cr.resources = make(map[schema.GroupVersionResource]registrySet)
		}

		// delete registry record for the resource.
		for k := range cr.resources {
			delete(cr.resources[k], rr.GetName())
		}

		c.clusterRegistry.Store(cluster, cr)

		c.queue.Add(cluster)
	}
}

// addCluster adds a cluster object to the queue if needed
func (c *Controller) addCluster(obj interface{}) {
	cluster := obj.(*clusterV1alpha1.Cluster)
	_, ok := c.clusterRegistry.Load(cluster.GetName())
	if ok {
		// unregistered cluster, do nothing.
		return
	}
	c.queue.Add(cluster.GetName())
}

// updateCluster rebuild informer if Cluster.Spec is changed
func (c *Controller) updateCluster(oldObj, curObj interface{}) {
	curCluster := curObj.(*clusterV1alpha1.Cluster)
	_, ok := c.clusterRegistry.Load(curCluster.GetName())
	if !ok {
		// unregistered cluster, do nothing.
		return
	}

	oldCluster := oldObj.(*clusterV1alpha1.Cluster)
	if curCluster.ResourceVersion == oldCluster.ResourceVersion {
		// no change, do nothing.
		return
	}

	if curCluster.DeletionTimestamp != nil {
		// cluster is being deleted.
		c.queue.Add(curCluster.GetName())
	}

	if !reflect.DeepEqual(curCluster.Spec, oldCluster.Spec) {
		// Cluster.Spec is changed, rebuild informer.
		c.InformerManager.Stop(curCluster.GetName())
		c.queue.Add(curCluster.GetName())
	}
}

// deleteCluster set cluster to not exists
func (c *Controller) deleteCluster(obj interface{}) {
	cluster := obj.(*clusterV1alpha1.Cluster)
	_, ok := c.clusterRegistry.Load(cluster.GetName())
	if !ok {
		// unregistered cluster, do nothing.
		return
	}

	// remove backend store
	backendstore.DeleteBackend(cluster.GetName())

	c.queue.Add(cluster.GetName())
}

// getClusterAndResource returns the cluster and resources from the resourceRegistry object
func (c *Controller) getClusters(affinity policyv1alpha1.ClusterAffinity) []string {
	clusters := make([]string, 0)
	lst, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list clusters: %v", err)
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
func (c *Controller) getResources(selectors []searchv1alpha1.ResourceSelector) []schema.GroupVersionResource {
	resources := make([]schema.GroupVersionResource, 0)
	for _, rs := range selectors {
		gvr, err := restmapper.GetGroupVersionResource(
			c.restMapper, schema.FromAPIVersionAndKind(rs.APIVersion, rs.Kind),
		)
		if err != nil {
			klog.Errorf("Failed to get gvr: %v", err)
			continue
		}
		resources = append(resources, gvr)
	}
	return resources
}
