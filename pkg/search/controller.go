package search

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
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

func (c *Controller) getClusterMatchedRegistries(cluster *clusterv1alpha1.Cluster) (indexedByName, matched map[string]*searchv1alpha1.ResourceRegistry, err error) {
	client := c.informerFactory.Search().V1alpha1().ResourceRegistries().Lister()
	var registries []*searchv1alpha1.ResourceRegistry
	if registries, err = client.List(labels.Everything()); err != nil {
		klog.Errorf("List resource registries for reconciling cluster failed, err: %s", err)
		return
	}
	if len(registries) == 0 {
		klog.Infof("No resource registries, no need to reconcile cluster")
		return
	}
	indexedByName = make(map[string]*searchv1alpha1.ResourceRegistry, len(registries))
	matched = make(map[string]*searchv1alpha1.ResourceRegistry, len(registries))
	for i, registry := range registries {
		indexedByName[registry.Name] = registries[i]
		if util.ClusterMatches(cluster, registry.Spec.TargetCluster) {
			matched[registry.Name] = registries[i]
		}
	}
	return
}

func (c *Controller) getRegistryAddedResources(registry *searchv1alpha1.ResourceRegistry, cr *clusterRegistry, added []schema.GroupVersionResource) []schema.GroupVersionResource {
	resourcesToWatch := c.getResources(registry.Spec.ResourceSelectors)
	for _, resource := range resourcesToWatch {
		if resourcesOnWatching, exist := cr.resources[resource]; exist {
			resourcesOnWatching[registry.Name] = struct{}{}
		} else {
			added = append(added, resource)
			cr.resources[resource] = registrySet{registry.Name: struct{}{}}
		}
	}
	return added
}

func (c *Controller) getClusterRegistriesModification(registries map[string]*searchv1alpha1.ResourceRegistry, matched map[string]*searchv1alpha1.ResourceRegistry, cr *clusterRegistry, added []string, removed []string) (updated *clusterRegistry, addedResources []schema.GroupVersionResource, removedResources []schema.GroupVersionResource) {
	defer func() {
		updated = cr
	}()
	if cr == nil {
		cr = &clusterRegistry{
			resources:  make(map[schema.GroupVersionResource]registrySet),
			registries: make(registrySet)}
	}
	for _, name := range removed {
		delete(cr.registries, name)
		for resource, previousRegistries := range cr.resources {
			delete(previousRegistries, name)
			if len(previousRegistries) == 0 {
				removedResources = append(removedResources, resource)
			}
		}
		for _, resource := range removedResources {
			delete(cr.resources, resource)
		}
	}
	for _, name := range added {
		cr.registries[name] = struct{}{}
		addedResources = c.getRegistryAddedResources(registries[name], cr, addedResources)
	}
	for _, registry := range matched {
		addedResources = c.getRegistryAddedResources(registry, cr, addedResources)
	}
	return
}

func (c *Controller) findOpenSearchBackend(matchedRegistries map[string]*searchv1alpha1.ResourceRegistry) *searchv1alpha1.BackendStoreConfig {
	for _, registry := range matchedRegistries {
		if backend := registry.Spec.BackendStore; backend != nil && backend.OpenSearch != nil {
			// one cluster may be related to multi registries,
			// however only one backend could keep in memory with one cluster
			return backend
		}
	}
	return nil
}

func (c *Controller) clusterAbleToCache(cluster string) (cls *clusterv1alpha1.Cluster, able bool, err error) {
	cls, err = c.clusterLister.Get(cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Try to stop cluster informer %s", cluster)
			c.InformerManager.Stop(cluster)
			return
		}
		return
	}

	if !cls.DeletionTimestamp.IsZero() {
		klog.Infof("Try to stop cluster informer %s", cluster)
		c.InformerManager.Stop(cluster)
		return
	}

	if !util.IsClusterReady(&cls.Status) {
		klog.Warningf("cluster %s is notReady try to stop this cluster informer", cluster)
		c.InformerManager.Stop(cluster)
		return
	}
	able = true
	return
}

func (c *Controller) reconcileClusterWithRegistries(cls *clusterv1alpha1.Cluster) (matchedRegistries map[string]*searchv1alpha1.ResourceRegistry, cr *clusterRegistry, resourcesChanged, newRegistry bool, err error) {
	cluster := cls.Name
	var allRegistries map[string]*searchv1alpha1.ResourceRegistry
	allRegistries, matchedRegistries, err = c.getClusterMatchedRegistries(cls)
	if err != nil {
		klog.Errorf("Get cluster matched resource registries failed, cluster: %s, error: %s", cluster, err)
		return
	}
	klog.V(4).Infof("Registries matched with cluster, cluster: %s, registries: %s",
		cluster, strings.Join(util.Keys(matchedRegistries), ", "))
	v, hasExistingRegistries := c.clusterRegistry.Load(cluster)
	var addedRegistries, removedRegistries []string
	if !hasExistingRegistries {
		klog.Infof("Cluster %s has no existing registries", cluster)
		addedRegistries, removedRegistries = util.DiffKey(registrySet(nil), matchedRegistries)
	} else {
		crv := v.(clusterRegistry)
		cr = &crv
		addedRegistries, removedRegistries = util.DiffKey(cr.registries, matchedRegistries)
	}
	if len(addedRegistries) > 0 {
		klog.V(4).Infof("New registries added to cluster, cluster: %s, registries: %s", cluster, strings.Join(addedRegistries, ", "))
	} else {
		klog.V(4).Infof("No registries added to cluster, cluster: %s", cluster)
	}
	if len(removedRegistries) > 0 {
		klog.V(4).Infof("Old registries removed from cluster, cluster: %s, registries: %s", cluster, strings.Join(removedRegistries, ", "))
	} else {
		klog.V(4).Infof("No registries removed from cluster, cluster: %s", cluster)
	}
	var addedResources, removedResources []schema.GroupVersionResource
	cr, addedResources, removedResources = c.getClusterRegistriesModification(allRegistries, matchedRegistries, cr, addedRegistries, removedRegistries)
	currentWatchingResources := util.Keys(cr.resources)
	resourcesString := util.StringerJoin(currentWatchingResources, ", ")
	if len(addedResources) > 0 {
		klog.V(4).Infof("New watching resources added to cluster, cluster: %s, resources: %s", cluster, util.StringerJoin(addedResources, ", "))
	} else {
		klog.V(4).Infof("No watching resources added to cluster, cluster: %s, resources: %s", cluster, resourcesString)
	}
	if len(removedResources) > 0 {
		klog.V(4).Infof("Resources watching removed from cluster, cluster: %s, resources: %s", cluster, util.StringerJoin(removedResources, ", "))
	} else {
		klog.V(4).Infof("No resources watching removed from cluster, cluster: %s, resources: %s", cluster, resourcesString)
	}
	resourcesChanged = len(addedResources) > 0 || len(removedResources) > 0
	newRegistry = !hasExistingRegistries
	return
}

func (c *Controller) getRegistryBackendHandler(cluster string, matchedRegistries map[string]*searchv1alpha1.ResourceRegistry) (cache.ResourceEventHandler, error) {
	backend := backendstore.GetBackend(cluster)
	if backend == nil {
		backendConfig := c.findOpenSearchBackend(matchedRegistries)
		backendstore.AddBackend(cluster, backendConfig)
		backend = backendstore.GetBackend(cluster)
	}
	if backend == nil {
		return nil, fmt.Errorf("failed to get backend store for cluster %s", cluster)
	}
	handler := backend.ResourceEventHandlerFuncs()
	if handler == nil {
		return nil, fmt.Errorf("failed to get resource event handler for cluster %s", cluster)
	}
	return handler, nil
}

// doCacheCluster processes the resourceRegistry object
// TODO: update status
func (c *Controller) doCacheCluster(cluster string) error {
	// STEP0: stop informer manager for the cluster which does not exist anymore or is not ready.
	cls, able, err := c.clusterAbleToCache(cluster)
	if err != nil || !able {
		return err
	}
	matchedRegistries, cr, resourcesChanged, newRegistry, err := c.reconcileClusterWithRegistries(cls)
	if err != nil {
		return err
	}

	// STEP1:  stop informer manager for the cluster which is not referenced by any `SearchRegistry` object.
	if cr.unregistry() {
		klog.Infof("Try to stop cluster informer %s", cluster)
		c.InformerManager.Stop(cluster)
		return nil
	}

	if resourcesChanged {
		klog.Infof("Create new informer for cluster %s", cluster)
		c.InformerManager.Stop(cluster)
	}

	if newRegistry {
		c.clusterRegistry.Store(cluster, *cr)
	}
	handler, err := c.getRegistryBackendHandler(cluster, matchedRegistries)
	if err != nil {
		return err
	}

	var newInformerCreated bool
	// STEP2: added/updated cluster, builds an informer manager for a specific cluster.
	if !c.InformerManager.IsManagerExist(cluster) {
		klog.Info("Try to build informer manager for cluster ", cluster)
		controlPlaneClient := gclient.NewForConfigOrDie(c.restConfig)

		clusterDynamicClient, err := util.NewClusterDynamicClientSet(cluster, controlPlaneClient)
		if err != nil {
			return err
		}
		_ = c.InformerManager.ForCluster(cluster, clusterDynamicClient.DynamicClientSet, 0)
		newInformerCreated = true
	}

	if !newInformerCreated && !resourcesChanged {
		return nil
	}

	sci := c.InformerManager.GetSingleClusterManager(cluster)
	for gvr := range cr.resources {
		klog.Infof("Add informer for %s, %v", cluster, gvr)
		sci.ForResource(gvr, handler)
	}

	klog.Infof("Start informer for %s", cluster)
	sci.Start()
	_ = sci.WaitForCacheSync()
	klog.Infof("Start informer for %s done", cluster)

	return nil
}

// addResourceRegistry parse the resourceRegistry object and add Cluster to the queue
func (c *Controller) addResourceRegistry(obj interface{}) {
	rr := obj.(*searchv1alpha1.ResourceRegistry)

	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
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
	clusterSet := sets.New[string]()
	for _, registry := range []*searchv1alpha1.ResourceRegistry{newRR, oldRR} {
		clusters := c.getClusters(registry.Spec.TargetCluster)
		clusterSet.Insert(clusters...)
	}
	for _, cluster := range clusterSet.UnsortedList() {
		c.queue.Add(cluster)
	}
}

// deleteResourceRegistry parse the resourceRegistry object and add deleted Cluster to the queue
func (c *Controller) deleteResourceRegistry(obj interface{}) {
	rr, isRR := obj.(*searchv1alpha1.ResourceRegistry)
	// We can get DeletedFinalStateUnknown instead of *searchv1alpha1.ResourceRegistry here and
	// we need to handle that correctly.
	if !isRR {
		deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.ErrorS(nil, "Received unpexpected object", "object", obj)
			return
		}
		rr, ok = deletedState.Obj.(*searchv1alpha1.ResourceRegistry)
		if !ok {
			klog.ErrorS(nil, "DeletedFinalStateUnknown contained non-ResourceRegistry object", "object", deletedState.Obj)
			return
		}
	}

	for _, cluster := range c.getClusters(rr.Spec.TargetCluster) {
		c.queue.Add(cluster)
	}
}

// addCluster adds a cluster object to the queue if needed
func (c *Controller) addCluster(obj interface{}) {
	cluster := obj.(*clusterv1alpha1.Cluster)

	c.queue.Add(cluster.GetName())
}

// updateCluster rebuild informer if Cluster.Spec is changed
func (c *Controller) updateCluster(oldObj, curObj interface{}) {
	curCluster := curObj.(*clusterv1alpha1.Cluster)

	oldCluster := oldObj.(*clusterv1alpha1.Cluster)
	if curCluster.ResourceVersion == oldCluster.ResourceVersion {
		// no change, do nothing.
		return
	}

	if curCluster.DeletionTimestamp != nil {
		// cluster is being deleted.
		c.queue.Add(curCluster.GetName())
	}

	if !reflect.DeepEqual(curCluster.Spec, oldCluster.Spec) || !reflect.DeepEqual(curCluster.Labels, oldCluster.Labels) {
		c.queue.Add(curCluster.GetName())
	}
}

// deleteCluster set cluster to not exists
func (c *Controller) deleteCluster(obj interface{}) {
	cluster, isCluster := obj.(*clusterv1alpha1.Cluster)
	// We can get DeletedFinalStateUnknown instead of *clusterV1alpha1.Cluster here and
	// we need to handle that correctly.
	if !isCluster {
		deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.ErrorS(nil, "Received unexpected object", "object", obj)
			return
		}
		cluster, ok = deletedState.Obj.(*clusterv1alpha1.Cluster)
		if !ok {
			klog.ErrorS(nil, "DeletedFinalStateUnknown contained non-Cluster object", "object", deletedState.Obj)
			return
		}
	}

	_, ok := c.clusterRegistry.Load(cluster.GetName())
	if !ok {
		// unregistered cluster, do nothing.
		return
	}

	// remove backend store
	backendstore.DeleteBackend(cluster.GetName())

	c.queue.Add(cluster.GetName())
}

// getClusters returns the cluster from the resourceRegistry object
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

// getResources returns the resources from the resourceRegistry object
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
