package karmadaquota

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	v1alpha1 "github.com/karmada-io/karmada/pkg/apis/quota/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	v1alpha1client "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/typed/quota/v1alpha1"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	v1alpha1lster "github.com/karmada-io/karmada/pkg/generated/listers/quota/v1alpha1"
	quotainstall "github.com/karmada-io/karmada/pkg/quota/v1alpha1/install"
	quota "github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/quota/v1alpha1/generic"
)

// NamespacedResourcesFunc knows how to discover namespaced resources.
type NamespacedResourcesFunc func() ([]*metav1.APIResourceList, error)

// ReplenishmentFunc is a signal that a resource changed in specified namespace
// that may require quota to be recalculated.
type ReplenishmentFunc func(groupResource schema.GroupResource, namespace string)

// ResyncPeriodFunc is a signal
type ResyncPeriodFunc func() time.Duration

var (
	// KeyFunc is the func key of DeletionHandlingMetaNamespaceKeyFunc
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// Controller is responsible for tracking quota usage status in the system
type Controller struct {
	// Must have authority to list all resources in the system, and update quota status
	rqClient v1alpha1client.KarmadaQuotasGetter
	// A lister/getter of resource quota objects
	rqLister v1alpha1lster.KarmadaQuotaLister
	// A list of functions that return true when their caches have synced
	informerSyncedFuncs []cache.InformerSynced
	// KarmadaQuota objects that need to be synchronized
	queue workqueue.RateLimitingInterface
	// missingUsageQueue holds objects that are missing the initial usage information
	missingUsageQueue workqueue.RateLimitingInterface
	// To allow injection of syncUsage for testing.
	syncHandler func(key string) error
	// function that controls full recalculation of quota usage
	resyncPeriod ResyncPeriodFunc
	// knows how to calculate usage
	registry quota.Registry
	// knows how to monitor all the resources tracked by quota and trigger replenishment
	quotaMonitor *QuotaMonitor
	// controls the workers that process quotas
	// this lock is acquired to control write access to the monitors and ensures that all
	// monitors are synced before the controller can process quotas.
	workerLock sync.RWMutex
}

// StartKarmadaQuotaController start a new work for karmadaquota
func StartKarmadaQuotaController(karmadaClient karmadaclientset.Interface) error {
	discoveryFunc := karmadaClient.Discovery().ServerPreferredNamespacedResources
	informerFactory := informerfactory.NewSharedInformerFactory(karmadaClient, 5*time.Minute)
	informersStarted := make(chan struct{})

	stopCh := context.TODO().Done()
	KarmadaQuotaController, err := NewController(karmadaClient, discoveryFunc, informerFactory, informersStarted)
	if err != nil {
		return err
	}

	go KarmadaQuotaController.Run(int(5), stopCh)
	// Periodically the quota controller to detect new resource types
	go KarmadaQuotaController.Sync(discoveryFunc, 5*time.Minute, stopCh)

	informerFactory.Start(wait.NeverStop)
	close(informersStarted)

	return nil
}

// NewController creates a quota controller with specified options
func NewController(karmadaClient karmadaclientset.Interface, discoveryFunc NamespacedResourcesFunc, informerFactory informerfactory.SharedInformerFactory,
	informersStarted <-chan struct{}) (*Controller, error) {
	KarmadaQuotaInformer := informerFactory.Quota().V1alpha1().KarmadaQuotas()
	listerFuncForResource := generic.ListerFuncForResourceFunc(informerFactory.ForResource)
	quotaConfiguration := quotainstall.NewQuotaConfigurationForControllers(listerFuncForResource)
	Registry := generic.NewRegistry(quotaConfiguration.Evaluators())

	// build the resource quota controller
	rq := &Controller{
		rqClient:            karmadaClient.QuotaV1alpha1(),
		rqLister:            KarmadaQuotaInformer.Lister(),
		informerSyncedFuncs: []cache.InformerSynced{KarmadaQuotaInformer.Informer().HasSynced},
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "KarmadaQuota_primary"),
		missingUsageQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "KarmadaQuota_priority"),
		resyncPeriod:        StaticResyncPeriodFunc(metav1.Duration{Duration: 5 * time.Minute}.Duration),
		registry:            Registry,
	}
	// set the synchronization handler
	rq.syncHandler = rq.syncKarmadaQuotaFromKey

	KarmadaQuotaInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: rq.addQuota,
			UpdateFunc: func(old, cur interface{}) {
				// We are only interested in observing updates to quota.spec to drive updates to quota.status.
				// We ignore all updates to quota.Status because they are all driven by this controller.
				// IMPORTANT:
				// We do not use this function to queue up a full quota recalculation.  To do so, would require
				// us to enqueue all quota.Status updates, and since quota.Status updates involve additional queries
				// that cannot be backed by a cache and result in a full query of a namespace's content, we do not
				// want to pay the price on spurious status updates.  As a result, we have a separate routine that is
				// responsible for enqueue of all resource quotas when doing a full resync (enqueueAll)
				oldKarmadaQuota := old.(*v1alpha1.KarmadaQuota)
				curKarmadaQuota := cur.(*v1alpha1.KarmadaQuota)
				if quota.Equals(oldKarmadaQuota.Spec.Hard, curKarmadaQuota.Spec.Hard) {
					return
				}
				rq.addQuota(curKarmadaQuota)
			},
			// This will enter the sync loop and no-op, because the controller has been deleted from the store.
			// Note that deleting a controller immediately after scaling it to 0 will not work. The recommended
			// way of achieving this is by performing a `stop` operation on the controller.
			DeleteFunc: rq.enqueueKarmadaQuota,
		},
		rq.resyncPeriod(),
	)

	if discoveryFunc != nil {
		qm := &QuotaMonitor{
			informersStarted:  informersStarted,
			informerFactory:   informerFactory,
			ignoredResources:  quotaConfiguration.IgnoredResources(),
			resourceChanges:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "resource_quota_controller_resource_changes"),
			resyncPeriod:      ResyncPeriod(metav1.Duration{Duration: 12 * time.Hour}.Duration),
			replenishmentFunc: rq.replenishQuota,
			registry:          rq.registry,
		}

		rq.quotaMonitor = qm

		// do initial quota monitor setup.  If we have a discovery failure here, it's ok. We'll discover more resources when a later sync happens.
		resources, err := GetQuotableResources(discoveryFunc)
		if discovery.IsGroupDiscoveryFailedError(err) {
			utilruntime.HandleError(fmt.Errorf("initial discovery check failure, continuing and counting on future sync update: %v", err))
		} else if err != nil {
			return nil, err
		}

		if err = qm.SyncMonitors(resources); err != nil {
			utilruntime.HandleError(fmt.Errorf("initial monitor sync has error: %v", err))
		}

		// only start quota once all informers synced
		rq.informerSyncedFuncs = append(rq.informerSyncedFuncs, qm.IsSynced)
	}

	return rq, nil
}

// enqueueAll is called at the fullResyncPeriod interval to force a full recalculation of quota usage statistics
func (rq *Controller) enqueueAll() {
	defer klog.V(4).Infof("Resource quota controller queued all resource quota for full calculation of usage")
	rqs, err := rq.rqLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to enqueue all - error listing resource quotas: %v", err))
		return
	}
	for i := range rqs {
		key, err := KeyFunc(rqs[i])
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", rqs[i], err))
			continue
		}
		rq.queue.Add(key)
	}
}

// obj could be an *v1alpha1.KarmadaQuota, or a DeletionFinalStateUnknown marker item.
func (rq *Controller) enqueueKarmadaQuota(obj interface{}) {
	key, err := KeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	rq.queue.Add(key)
}

func (rq *Controller) addQuota(obj interface{}) {
	key, err := KeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	KarmadaQuota := obj.(*v1alpha1.KarmadaQuota)

	// if we declared an intent that is not yet captured in status (prioritize it)
	if !apiequality.Semantic.DeepEqual(KarmadaQuota.Spec.Hard, KarmadaQuota.Status.Hard) {
		rq.missingUsageQueue.Add(key)
		return
	}

	// if we declared a constraint that has no usage (which this controller can calculate, prioritize it)
	for constraint := range KarmadaQuota.Status.Hard {
		if _, usageFound := KarmadaQuota.Status.Used[constraint]; !usageFound {
			matchedResources := []corev1.ResourceName{constraint}
			for _, evaluator := range rq.registry.List() {
				if intersection := evaluator.MatchingResources(matchedResources); len(intersection) > 0 {
					rq.missingUsageQueue.Add(key)
					return
				}
			}
		}
	}
	// no special priority, go in normal recalc queue
	rq.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
func (rq *Controller) worker(queue workqueue.RateLimitingInterface) func() {
	workFunc := func() bool {
		key, quit := queue.Get()
		if quit {
			return true
		}
		defer queue.Done(key)
		rq.workerLock.RLock()
		defer rq.workerLock.RUnlock()
		klog.V(4).Infof("do worker key:%q", key)
		err := rq.syncHandler(key.(string))
		if err == nil {
			queue.Forget(key)
			return false
		}
		utilruntime.HandleError(err)
		queue.AddRateLimited(key)
		return false
	}

	return func() {
		for {
			if quit := workFunc(); quit {
				klog.Infof("resource quota controller worker shutting down")
				return
			}
		}
	}
}

// Run begins quota controller using the specified number of workers
func (rq *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer rq.queue.ShutDown()

	klog.Infof("Starting resource quota controller")
	defer klog.Infof("Shutting down resource quota controller")

	if rq.quotaMonitor != nil {
		go rq.quotaMonitor.Run(stopCh)
	}

	if !cache.WaitForNamedCacheSync("resource quota", stopCh, rq.informerSyncedFuncs...) {
		return
	}

	// the workers that chug through the quota calculation backlog
	for i := 0; i < workers; i++ {
		go wait.Until(rq.worker(rq.queue), time.Second, stopCh)
		go wait.Until(rq.worker(rq.missingUsageQueue), time.Second, stopCh)
	}
	// the timer for how often we do a full recalculation across all quotas
	go wait.Until(func() { rq.enqueueAll() }, rq.resyncPeriod(), stopCh)
	<-stopCh
}

// syncKarmadaQuotaFromKey syncs a quota key
func (rq *Controller) syncKarmadaQuotaFromKey(key string) (err error) {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing karmada quota %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	KarmadaQuota, err := rq.rqLister.KarmadaQuotas(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.Infof("Resource quota has been deleted %v", key)
		return nil
	}
	if err != nil {
		klog.Infof("Unable to retrieve resource quota %v from store: %v", key, err)
		return err
	}
	return rq.syncKarmadaQuota(KarmadaQuota)
}

// syncKarmadaQuota runs a complete sync of resource quota status across all known kinds
func (rq *Controller) syncKarmadaQuota(KarmadaQuota *v1alpha1.KarmadaQuota) (err error) {
	klog.Infof("start syncKarmadaQuota namespace[%s], name[%s]", KarmadaQuota.Namespace, KarmadaQuota.Name)
	// quota is dirty if any part of spec hard limits differs from the status hard limits
	statusLimitsDirty := !apiequality.Semantic.DeepEqual(KarmadaQuota.Spec.Hard, KarmadaQuota.Status.Hard)

	// dirty tracks if the usage status differs from the previous sync,
	// if so, we send a new usage with latest status
	// if this is our first sync, it will be dirty by default, since we need track usage
	dirty := statusLimitsDirty || KarmadaQuota.Status.Hard == nil || KarmadaQuota.Status.Used == nil

	used := corev1.ResourceList{}
	if KarmadaQuota.Status.Used != nil {
		used = quota.Add(corev1.ResourceList{}, KarmadaQuota.Status.Used)
	}
	hardLimits := quota.Add(corev1.ResourceList{}, KarmadaQuota.Spec.Hard)

	var errs []error

	newUsage, err := quota.CalculateUsage(KarmadaQuota.Namespace, KarmadaQuota.Spec.Scopes, hardLimits, rq.registry, KarmadaQuota.Spec.ScopeSelector)
	if err != nil {
		// if err is non-nil, remember it to return, but continue updating status with any resources in newUsage
		klog.V(4).Infof("syncKarmadaQuota error:%v", err)
		errs = append(errs, err)
	}

	for key, value := range newUsage {
		used[key] = value
	}

	// ensure set of used values match those that have hard constraints
	hardResources := quota.ResourceNames(hardLimits)
	used = quota.Mask(used, hardResources)

	// Create a usage object that is based on the quota resource version that will handle updates
	// by default, we preserve the past usage observation, and set hard to the current spec
	usage := KarmadaQuota.DeepCopy()
	usage.Status = v1alpha1.KarmadaQuotaStatus{
		Hard: hardLimits,
		Used: used,
	}

	dirty = dirty || !quota.Equals(usage.Status.Used, KarmadaQuota.Status.Used)

	// there was a change observed by this controller that requires we update quota
	if dirty {
		_, err = rq.rqClient.KarmadaQuotas(usage.Namespace).UpdateStatus(context.TODO(), usage, metav1.UpdateOptions{})
		//_, err = rq.karmadaClient.QuotaV1alpha1().KarmadaQuotas(usage.Namespace).UpdateStatus(context.TODO(), usage, metav1.UpdateOptions{})
		if err != nil {
			klog.V(4).Infof("syncKarmadaQuota error:%v", err)
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}

// replenishQuota is a replenishment function invoked by a controller to notify that a quota should be recalculated
func (rq *Controller) replenishQuota(groupResource schema.GroupResource, namespace string) {
	// check if the quota controller can evaluate this groupResource, if not, ignore it altogether...
	evaluator := rq.registry.Get(groupResource)
	if evaluator == nil {
		return
	}

	// check if this namespace even has a quota...
	KarmadaQuotas, err := rq.rqLister.KarmadaQuotas(namespace).List(labels.Everything())
	if apierrors.IsNotFound(err) {
		utilruntime.HandleError(fmt.Errorf("quota controller could not find KarmadaQuota associated with namespace: %s, could take up to %v before a quota replenishes", namespace, rq.resyncPeriod()))
		return
	}
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error checking to see if namespace %s has any KarmadaQuota associated with it: %v", namespace, err))
		return
	}
	if len(KarmadaQuotas) == 0 {
		return
	}

	// only queue those quotas that are tracking a resource associated with this kind.
	for i := range KarmadaQuotas {
		KarmadaQuota := KarmadaQuotas[i]
		KarmadaQuotaResources := quota.ResourceNames(KarmadaQuota.Status.Hard)
		if intersection := evaluator.MatchingResources(KarmadaQuotaResources); len(intersection) > 0 {
			// TODO: make this support targeted replenishment to a specific kind, right now it does a full recalc on that quota.
			rq.enqueueKarmadaQuota(KarmadaQuota)
		}
	}
}

// Sync periodically resyncs the controller when new resources are observed from discovery.
func (rq *Controller) Sync(discoveryFunc NamespacedResourcesFunc, period time.Duration, stopCh <-chan struct{}) {
	// Something has changed, so track the new state and perform a sync.
	oldResources := make(map[schema.GroupVersionResource]struct{})
	wait.Until(func() {
		// Get the current resource list from discovery.
		newResources, err := GetQuotableResources(discoveryFunc)
		if err != nil {
			utilruntime.HandleError(err)

			if discovery.IsGroupDiscoveryFailedError(err) && len(newResources) > 0 {
				// In partial discovery cases, don't remove any existing informers, just add new ones
				for k, v := range oldResources {
					newResources[k] = v
				}
			} else {
				// short circuit in non-discovery error cases or if discovery returned zero resources
				return
			}
		}

		// Decide whether discovery has reported a change.
		if reflect.DeepEqual(oldResources, newResources) {
			klog.V(4).Infof("no resource updates from discovery, skipping resource quota sync")
			return
		}

		// Ensure workers are paused to avoid processing events before informers
		// have resynced.
		rq.workerLock.Lock()
		defer rq.workerLock.Unlock()

		// Something has changed, so track the new state and perform a sync.
		if klog.V(2).Enabled() {
			klog.Infof("syncing resource quota controller with updated resources from discovery: %s", printDiff(oldResources, newResources))
		}

		// Perform the monitor resync and wait for controllers to report cache sync.
		if err := rq.resyncMonitors(newResources); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to sync resource monitors: %v", err))
			return
		}
		// wait for caches to fill for a while (our sync period).
		// this protects us from deadlocks where available resources changed and one of our informer caches will never fill.
		// informers keep attempting to sync in the background, so retrying doesn't interrupt them.
		// the call to resyncMonitors on the reattempt will no-op for resources that still exist.
		if rq.quotaMonitor != nil && !cache.WaitForNamedCacheSync("resource quota", waitForStopOrTimeout(stopCh, period), rq.quotaMonitor.IsSynced) {
			utilruntime.HandleError(fmt.Errorf("timed out waiting for quota monitor sync"))
			return
		}

		// success, remember newly synced resources
		oldResources = newResources
		klog.V(2).Infof("synced quota controller")
	}, period, stopCh)
}

// printDiff returns a human-readable summary of what resources were added and removed
func printDiff(oldResources, newResources map[schema.GroupVersionResource]struct{}) string {
	removed := sets.NewString()
	for oldResource := range oldResources {
		if _, ok := newResources[oldResource]; !ok {
			removed.Insert(fmt.Sprintf("%+v", oldResource))
		}
	}
	added := sets.NewString()
	for newResource := range newResources {
		if _, ok := oldResources[newResource]; !ok {
			added.Insert(fmt.Sprintf("%+v", newResource))
		}
	}
	return fmt.Sprintf("added: %v, removed: %v", added.List(), removed.List())
}

// waitForStopOrTimeout returns a stop channel that closes when the provided stop channel closes or when the specified timeout is reached
func waitForStopOrTimeout(stopCh <-chan struct{}, timeout time.Duration) <-chan struct{} {
	stopChWithTimeout := make(chan struct{})
	go func() {
		defer close(stopChWithTimeout)
		select {
		case <-stopCh:
		case <-time.After(timeout):
		}
	}()
	return stopChWithTimeout
}

// resyncMonitors starts or stops quota monitors as needed to ensure that all
// (and only) those resources present in the map are monitored.
func (rq *Controller) resyncMonitors(resources map[schema.GroupVersionResource]struct{}) error {
	if rq.quotaMonitor == nil {
		return nil
	}

	if err := rq.quotaMonitor.SyncMonitors(resources); err != nil {
		return err
	}
	rq.quotaMonitor.StartMonitors()
	return nil
}

// GetQuotableResources returns all resources that the quota system should recognize.
// It requires a resource supports the following verbs: 'create','list','delete'
// This function may return both results and an error.  If that happens, it means that the discovery calls were only
// partially successful.  A decision about whether to proceed or not is left to the caller.
func GetQuotableResources(discoveryFunc NamespacedResourcesFunc) (map[schema.GroupVersionResource]struct{}, error) {
	possibleResources, discoveryErr := discoveryFunc()
	if discoveryErr != nil && len(possibleResources) == 0 {
		return nil, fmt.Errorf("failed to discover resources: %v", discoveryErr)
	}
	quotableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"create", "list", "watch", "delete"}}, possibleResources)
	quotableGroupVersionResources, err := discovery.GroupVersionResources(quotableResources)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resources: %v", err)
	}
	// return the original discovery error (if any) in addition to the list
	return quotableGroupVersionResources, discoveryErr
}

// StaticResyncPeriodFunc returns the resync period specified
func StaticResyncPeriodFunc(resyncPeriod time.Duration) ResyncPeriodFunc {
	return func() time.Duration {
		return resyncPeriod
	}
}

// ResyncPeriod return a random time.Duration
func ResyncPeriod(resyncPeriod time.Duration) func() time.Duration {
	return func() time.Duration {
		dice, err := rand.Int(rand.Reader, big.NewInt(6))
		if err != nil {
			fmt.Println(err)
		}
		factor := float64(dice.Int64()) + 1
		return time.Duration(float64(resyncPeriod) * factor)
	}
}
