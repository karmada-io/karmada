package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	utilhelper "github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "cluster-controller"
	// MonitorRetrySleepTime is the amount of time the cluster controller that should
	// sleep between retrying cluster health updates.
	MonitorRetrySleepTime = 20 * time.Millisecond
	// HealthUpdateRetry controls the number of retries of writing cluster health update.
	HealthUpdateRetry = 5
)

var (
	// UnreachableTaintTemplate is the taint for when a cluster becomes unreachable.
	// Used for taint based eviction.
	UnreachableTaintTemplate = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnreachable,
		Effect: corev1.TaintEffectNoExecute,
	}

	// UnreachableTaintTemplateForSched is the taint for when a cluster becomes unreachable.
	// Used for taint based schedule.
	UnreachableTaintTemplateForSched = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnreachable,
		Effect: corev1.TaintEffectNoSchedule,
	}

	// NotReadyTaintTemplate is the taint for when a cluster is not ready for executing resources.
	// Used for taint based eviction.
	NotReadyTaintTemplate = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterNotReady,
		Effect: corev1.TaintEffectNoExecute,
	}

	// NotReadyTaintTemplateForSched is the taint for when a cluster is not ready for executing resources.
	// Used for taint based schedule.
	NotReadyTaintTemplateForSched = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterNotReady,
		Effect: corev1.TaintEffectNoSchedule,
	}

	// TerminatingTaintTemplate is the taint for when a cluster is terminating executing resources.
	// Used for taint based eviction.
	TerminatingTaintTemplate = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterTerminating,
		Effect: corev1.TaintEffectNoExecute,
	}
)

// Controller is to sync Cluster.
type Controller struct {
	client.Client      // used to operate Cluster resources.
	EventRecorder      record.EventRecorder
	EnableTaintManager bool

	// ClusterMonitorPeriod represents cluster-controller monitoring period, i.e. how often does
	// cluster-controller check cluster health signal posted from cluster-status-controller.
	// This value should be lower than ClusterMonitorGracePeriod.
	ClusterMonitorPeriod time.Duration
	// ClusterMonitorGracePeriod represents the grace period after last cluster health probe time.
	// If it doesn't receive update for this amount of time, it will start posting
	// "ClusterReady==ConditionUnknown".
	ClusterMonitorGracePeriod time.Duration
	// When cluster is just created, e.g. agent bootstrap or cluster join, we give a longer grace period.
	ClusterStartupGracePeriod time.Duration
	// FailoverEvictionTimeout represents the grace period for deleting scheduling result on failed clusters.
	FailoverEvictionTimeout            time.Duration
	ClusterTaintEvictionRetryFrequency time.Duration

	// Per Cluster map stores last observed health together with a local time when it was observed.
	clusterHealthMap *clusterHealthMap
}

type clusterHealthMap struct {
	sync.RWMutex
	clusterHealths map[string]*clusterHealthData
}

func newClusterHealthMap() *clusterHealthMap {
	return &clusterHealthMap{
		clusterHealths: make(map[string]*clusterHealthData),
	}
}

// getDeepCopy - returns copy of cluster health data.
// It prevents data being changed after retrieving it from the map.
func (n *clusterHealthMap) getDeepCopy(name string) *clusterHealthData {
	n.RLock()
	defer n.RUnlock()
	return n.clusterHealths[name].deepCopy()
}

func (n *clusterHealthMap) set(name string, data *clusterHealthData) {
	n.Lock()
	defer n.Unlock()
	n.clusterHealths[name] = data
}

func (n *clusterHealthMap) delete(name string) {
	n.Lock()
	defer n.Unlock()
	delete(n.clusterHealths, name)
}

type clusterHealthData struct {
	probeTimestamp           metav1.Time
	readyTransitionTimestamp metav1.Time
	status                   *clusterv1alpha1.ClusterStatus
	lease                    *coordinationv1.Lease
}

func (n *clusterHealthData) deepCopy() *clusterHealthData {
	if n == nil {
		return nil
	}
	return &clusterHealthData{
		probeTimestamp:           n.probeTimestamp,
		readyTransitionTimestamp: n.readyTransitionTimestamp,
		status:                   n.status.DeepCopy(),
		lease:                    n.lease.DeepCopy(),
	}
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling cluster %s", req.NamespacedName.Name)

	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return c.removeCluster(ctx, cluster)
	}

	return c.syncCluster(ctx, cluster)
}

// Start starts an asynchronous loop that monitors the status of cluster.
func (c *Controller) Start(ctx context.Context) error {
	// starts a goroutine to collect existed clusters' ID for upgrade scenario
	// And can be removed in next version
	klog.Infof("Starting collect ID of already existed clusters")
	go func() {
		err := c.collectIDForClusterObjectIfNeeded(ctx)
		if err != nil {
			klog.Errorf("Error collecting clusters' ID: %v", err)
		}

		klog.Infof("Finishing collect ID of already existed clusters")
	}()

	klog.Infof("Starting cluster health monitor")
	defer klog.Infof("Shutting cluster health monitor")

	// Incorporate the results of cluster health signal pushed from cluster-status-controller to master.
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := c.monitorClusterHealth(ctx); err != nil {
			klog.Errorf("Error monitoring cluster health: %v", err)
		}
	}, c.ClusterMonitorPeriod)
	<-ctx.Done()

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	c.clusterHealthMap = newClusterHealthMap()
	return utilerrors.NewAggregate([]error{
		controllerruntime.NewControllerManagedBy(mgr).For(&clusterv1alpha1.Cluster{}).Complete(c),
		mgr.Add(c),
	})
}

func (c *Controller) syncCluster(ctx context.Context, cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	// create execution space
	err := c.createExecutionSpace(cluster)
	if err != nil {
		c.EventRecorder.Event(cluster, corev1.EventTypeWarning, events.EventReasonCreateExecutionSpaceFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	// taint cluster by condition
	err = c.taintClusterByCondition(ctx, cluster)
	if err != nil {
		c.EventRecorder.Event(cluster, corev1.EventTypeWarning, events.EventReasonTaintClusterByConditionFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}

	// ensure finalizer
	return c.ensureFinalizer(cluster)
}

func (c *Controller) removeCluster(ctx context.Context, cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	// add terminating taint before cluster is deleted
	if err := utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{TerminatingTaintTemplate}, nil, cluster); err != nil {
		c.EventRecorder.Event(cluster, corev1.EventTypeWarning, events.EventReasonRemoveTargetClusterFailed, err.Error())
		klog.ErrorS(err, "Failed to update terminating taint", "cluster", cluster.Name)
		return controllerruntime.Result{Requeue: true}, err
	}

	if err := c.removeExecutionSpace(cluster); err != nil {
		klog.Errorf("Failed to remove execution space %s: %v", cluster.Name, err)
		c.EventRecorder.Event(cluster, corev1.EventTypeWarning, events.EventReasonRemoveExecutionSpaceFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}
	msg := fmt.Sprintf("Removed execution space for cluster(%s).", cluster.Name)
	c.EventRecorder.Event(cluster, corev1.EventTypeNormal, events.EventReasonRemoveExecutionSpaceSucceed, msg)

	if exist, err := c.ExecutionSpaceExistForCluster(cluster.Name); err != nil {
		klog.Errorf("Failed to check weather the execution space exist in the given member cluster or not, error is: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	} else if exist {
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("requeuing operation until the execution space %v deleted, ", cluster.Name)
	}

	// delete the health data from the map explicitly after we removing the cluster.
	c.clusterHealthMap.delete(cluster.Name)

	// check if target cluster is removed from all bindings.
	if c.EnableTaintManager {
		if done, err := c.isTargetClusterRemoved(ctx, cluster); err != nil {
			klog.ErrorS(err, "Failed to check whether target cluster is removed from bindings", "cluster", cluster.Name)
			return controllerruntime.Result{Requeue: true}, err
		} else if !done {
			klog.InfoS("Terminating taint eviction process has not finished yet, will try again later", "cluster", cluster.Name)
			return controllerruntime.Result{RequeueAfter: c.ClusterTaintEvictionRetryFrequency}, nil
		}
	}

	return c.removeFinalizer(cluster)
}

func (c *Controller) isTargetClusterRemoved(ctx context.Context, cluster *clusterv1alpha1.Cluster) (bool, error) {
	// List all ResourceBindings which are assigned to this cluster.
	rbList := &workv1alpha2.ResourceBindingList{}
	if err := c.List(ctx, rbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(rbClusterKeyIndex, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ResourceBindings", "cluster", cluster.Name)
		return false, err
	}
	if len(rbList.Items) != 0 {
		return false, nil
	}
	// List all ClusterResourceBindings which are assigned to this cluster.
	crbList := &workv1alpha2.ClusterResourceBindingList{}
	if err := c.List(ctx, crbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(crbClusterKeyIndex, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ClusterResourceBindings", "cluster", cluster.Name)
		return false, err
	}
	if len(crbList.Items) != 0 {
		return false, nil
	}
	return true, nil
}

// removeExecutionSpace deletes the given execution space
func (c *Controller) removeExecutionSpace(cluster *clusterv1alpha1.Cluster) error {
	executionSpaceName, err := names.GenerateExecutionSpaceName(cluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
		return err
	}

	executionSpaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: executionSpaceName,
		},
	}
	if err := c.Client.Delete(context.TODO(), executionSpaceObj); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Error while deleting namespace %s: %s", executionSpaceName, err)
		return err
	}

	return nil
}

// ExecutionSpaceExistForCluster determine whether the execution space exists in the cluster
func (c *Controller) ExecutionSpaceExistForCluster(clusterName string) (bool, error) {
	executionSpaceName, err := names.GenerateExecutionSpaceName(clusterName)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", clusterName, err)
		return false, err
	}

	executionSpaceObj := &corev1.Namespace{}
	err = c.Client.Get(context.TODO(), types.NamespacedName{Name: executionSpaceName}, executionSpaceObj)
	if apierrors.IsNotFound(err) {
		klog.V(2).Infof("execution space(%s) no longer exists", executionSpaceName)
		return false, nil
	}
	if err != nil {
		klog.Errorf("Failed to get execution space %v, err is %v ", executionSpaceName, err)
		return false, err
	}
	return true, nil
}

func (c *Controller) removeFinalizer(cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	if !controllerutil.ContainsFinalizer(cluster, util.ClusterControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.RemoveFinalizer(cluster, util.ClusterControllerFinalizer)
	err := c.Client.Update(context.TODO(), cluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *Controller) ensureFinalizer(cluster *clusterv1alpha1.Cluster) (controllerruntime.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, util.ClusterControllerFinalizer) {
		return controllerruntime.Result{}, nil
	}

	controllerutil.AddFinalizer(cluster, util.ClusterControllerFinalizer)
	err := c.Client.Update(context.TODO(), cluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

// createExecutionSpace creates member cluster execution space when member cluster joined
func (c *Controller) createExecutionSpace(cluster *clusterv1alpha1.Cluster) error {
	executionSpaceName, err := names.GenerateExecutionSpaceName(cluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
		return err
	}

	// create member cluster execution space when member cluster joined
	executionSpaceObj := &corev1.Namespace{}
	err = c.Client.Get(context.TODO(), types.NamespacedName{Name: executionSpaceName}, executionSpaceObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get namespace(%s): %v", executionSpaceName, err)
			return err
		}

		// create only when not exist
		executionSpace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: executionSpaceName,
			},
		}
		err := c.Client.Create(context.TODO(), executionSpace)
		if err != nil {
			klog.Errorf("Failed to create execution space for cluster(%s): %v", cluster.Name, err)
			return err
		}
		msg := fmt.Sprintf("Created execution space(%s) for cluster(%s).", executionSpaceName, cluster.Name)
		klog.V(2).Info(msg)
		c.EventRecorder.Event(cluster, corev1.EventTypeNormal, events.EventReasonCreateExecutionSpaceSucceed, msg)
	}

	return nil
}

func (c *Controller) monitorClusterHealth(ctx context.Context) (err error) {
	clusterList := &clusterv1alpha1.ClusterList{}
	if err = c.Client.List(ctx, clusterList); err != nil {
		return err
	}

	clusters := clusterList.Items
	for i := range clusters {
		cluster := &clusters[i]
		var observedReadyCondition, currentReadyCondition *metav1.Condition
		if err = wait.PollImmediate(MonitorRetrySleepTime, MonitorRetrySleepTime*HealthUpdateRetry, func() (bool, error) {
			// Cluster object may be changed in this function.
			observedReadyCondition, currentReadyCondition, err = c.tryUpdateClusterHealth(ctx, cluster)
			if err == nil {
				return true, nil
			}
			clusterName := cluster.Name
			if err = c.Get(ctx, client.ObjectKey{Name: clusterName}, cluster); err != nil {
				// If the cluster does not exist any more, we delete the health data from the map.
				if apierrors.IsNotFound(err) {
					c.clusterHealthMap.delete(clusterName)
					return true, nil
				}
				klog.Errorf("Getting a cluster to retry updating cluster health error: %v", clusterName, err)
				return false, err
			}
			return false, nil
		}); err != nil {
			klog.Errorf("Update health of Cluster '%v' from Controller error: %v. Skipping.", cluster.Name, err)
			continue
		}

		if currentReadyCondition != nil {
			if err = c.processTaintBaseEviction(ctx, cluster, observedReadyCondition); err != nil {
				klog.Errorf("Failed to process taint base eviction error: %v. Skipping.", err)
				continue
			}
		}
	}

	return nil
}

// tryUpdateClusterHealth checks a given cluster's conditions and tries to update it.
//nolint:gocyclo
func (c *Controller) tryUpdateClusterHealth(ctx context.Context, cluster *clusterv1alpha1.Cluster) (*metav1.Condition, *metav1.Condition, error) {
	// Step 1: Get the last cluster heath from `clusterHealthMap`.
	clusterHealth := c.clusterHealthMap.getDeepCopy(cluster.Name)
	defer func() {
		c.clusterHealthMap.set(cluster.Name, clusterHealth)
	}()

	// Step 2: Get the cluster ready condition.
	var gracePeriod time.Duration
	var observedReadyCondition *metav1.Condition
	currentReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)
	if currentReadyCondition == nil {
		// If ready condition is nil, then cluster-status-controller has never posted cluster status.
		// A fake ready condition is created, where LastTransitionTime is set to cluster.CreationTimestamp
		// to avoid handle the corner case.
		observedReadyCondition = &metav1.Condition{
			Type:               clusterv1alpha1.ClusterConditionReady,
			Status:             metav1.ConditionUnknown,
			LastTransitionTime: cluster.CreationTimestamp,
		}
		gracePeriod = c.ClusterStartupGracePeriod
		if clusterHealth != nil {
			clusterHealth.status = &cluster.Status
		} else {
			clusterHealth = &clusterHealthData{
				status:                   &cluster.Status,
				probeTimestamp:           cluster.CreationTimestamp,
				readyTransitionTimestamp: cluster.CreationTimestamp,
			}
		}
	} else {
		// If ready condition is not nil, make a copy of it, since we may modify it in place later.
		observedReadyCondition = currentReadyCondition.DeepCopy()
		gracePeriod = c.ClusterMonitorGracePeriod
	}

	// Step 3: Get the last condition and lease from `clusterHealth`.
	var savedCondition *metav1.Condition
	var savedLease *coordinationv1.Lease
	if clusterHealth != nil {
		savedCondition = meta.FindStatusCondition(clusterHealth.status.Conditions, clusterv1alpha1.ClusterConditionReady)
		savedLease = clusterHealth.lease
	}

	// Step 4: Update the clusterHealth if necessary.
	// If this condition has no difference from last condition, we leave everything as it is.
	// Otherwise, we only update the probeTimestamp.
	if clusterHealth == nil {
		clusterHealth = &clusterHealthData{
			status:                   cluster.Status.DeepCopy(),
			probeTimestamp:           metav1.Now(),
			readyTransitionTimestamp: metav1.Now(),
		}
	} else if !equality.Semantic.DeepEqual(savedCondition, currentReadyCondition) {
		transitionTime := metav1.Now()
		if savedCondition != nil && currentReadyCondition != nil && savedCondition.LastTransitionTime == currentReadyCondition.LastTransitionTime {
			transitionTime = clusterHealth.readyTransitionTimestamp
		}
		clusterHealth = &clusterHealthData{
			status:                   cluster.Status.DeepCopy(),
			probeTimestamp:           metav1.Now(),
			readyTransitionTimestamp: transitionTime,
		}
	}

	if cluster.Spec.SyncMode == clusterv1alpha1.Push {
		return observedReadyCondition, currentReadyCondition, nil
	}

	// Always update the probe time if cluster lease is renewed.
	// Note: If cluster-status-controller never posted the cluster status, but continues renewing the
	// heartbeat leases, the cluster controller will assume the cluster is healthy and take no action.
	observedLease := &coordinationv1.Lease{}
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: util.NamespaceClusterLease, Name: cluster.Name}, observedLease)
	if err == nil && (savedLease == nil || savedLease.Spec.RenewTime.Before(observedLease.Spec.RenewTime)) {
		clusterHealth.lease = observedLease
		clusterHealth.probeTimestamp = metav1.Now()
	}

	// Step 5: Check whether the probe timestamp has timed out.
	if metav1.Now().After(clusterHealth.probeTimestamp.Add(gracePeriod)) {
		clusterConditionTypes := []string{
			clusterv1alpha1.ClusterConditionReady,
		}

		nowTimestamp := metav1.Now()
		for _, clusterConditionType := range clusterConditionTypes {
			currentCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterConditionType)
			if currentCondition == nil {
				klog.V(2).Infof("Condition %v of cluster %v was never updated by cluster-status-controller",
					clusterConditionType, cluster.Name)
				cluster.Status.Conditions = append(cluster.Status.Conditions, metav1.Condition{
					Type:               clusterConditionType,
					Status:             metav1.ConditionUnknown,
					Reason:             "ClusterStatusNeverUpdated",
					Message:            "Cluster status controller never posted cluster status.",
					LastTransitionTime: nowTimestamp,
				})
			} else {
				klog.V(2).Infof("cluster %v hasn't been updated for %+v. Last %v is: %+v",
					cluster.Name, metav1.Now().Time.Sub(clusterHealth.probeTimestamp.Time), clusterConditionType, currentCondition)
				if currentCondition.Status != metav1.ConditionUnknown {
					currentCondition.Status = metav1.ConditionUnknown
					currentCondition.Reason = "ClusterStatusUnknown"
					currentCondition.Message = "Cluster status controller stopped posting cluster status."
					currentCondition.LastTransitionTime = nowTimestamp
				}
			}
		}
		// We need to update currentReadyCondition due to its value potentially changed.
		currentReadyCondition = meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)

		if !equality.Semantic.DeepEqual(currentReadyCondition, observedReadyCondition) {
			if err := c.Status().Update(ctx, cluster); err != nil {
				klog.Errorf("Error updating cluster %s: %v", cluster.Name, err)
				return observedReadyCondition, currentReadyCondition, err
			}
			clusterHealth = &clusterHealthData{
				status:                   &cluster.Status,
				probeTimestamp:           clusterHealth.probeTimestamp,
				readyTransitionTimestamp: metav1.Now(),
				lease:                    observedLease,
			}
			return observedReadyCondition, currentReadyCondition, nil
		}
	}
	return observedReadyCondition, currentReadyCondition, nil
}

func (c *Controller) processTaintBaseEviction(ctx context.Context, cluster *clusterv1alpha1.Cluster, observedReadyCondition *metav1.Condition) error {
	decisionTimestamp := metav1.Now()
	clusterHealth := c.clusterHealthMap.getDeepCopy(cluster.Name)
	if clusterHealth == nil {
		return fmt.Errorf("health data doesn't exist for cluster %q", cluster.Name)
	}
	// Check eviction timeout against decisionTimestamp
	switch observedReadyCondition.Status {
	case metav1.ConditionFalse:
		if features.FeatureGate.Enabled(features.Failover) && decisionTimestamp.After(clusterHealth.readyTransitionTimestamp.Add(c.FailoverEvictionTimeout)) {
			// We want to update the taint straight away if Cluster is already tainted with the UnreachableTaint
			taintToAdd := *NotReadyTaintTemplate
			if err := utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{&taintToAdd}, []*corev1.Taint{UnreachableTaintTemplate}, cluster); err != nil {
				klog.ErrorS(err, "Failed to instantly update UnreachableTaint to NotReadyTaint, will try again in the next cycle.", "cluster", cluster.Name)
			}
		}
	case metav1.ConditionUnknown:
		if features.FeatureGate.Enabled(features.Failover) && decisionTimestamp.After(clusterHealth.probeTimestamp.Add(c.FailoverEvictionTimeout)) {
			// We want to update the taint straight away if Cluster is already tainted with the UnreachableTaint
			taintToAdd := *UnreachableTaintTemplate
			if err := utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{&taintToAdd}, []*corev1.Taint{NotReadyTaintTemplate}, cluster); err != nil {
				klog.ErrorS(err, "Failed to instantly swap NotReadyTaint to UnreachableTaint, will try again in the next cycle.", "cluster", cluster.Name)
			}
		}
	case metav1.ConditionTrue:
		if err := utilhelper.UpdateClusterControllerTaint(ctx, c.Client, nil, []*corev1.Taint{NotReadyTaintTemplate, UnreachableTaintTemplate}, cluster); err != nil {
			klog.ErrorS(err, "Failed to remove taints from cluster, will retry in next iteration.", "cluster", cluster.Name)
		}
	}
	return nil
}

func (c *Controller) taintClusterByCondition(ctx context.Context, cluster *clusterv1alpha1.Cluster) error {
	currentReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionReady)
	var err error
	if currentReadyCondition != nil {
		switch currentReadyCondition.Status {
		case metav1.ConditionFalse:
			// Add NotReadyTaintTemplateForSched taint immediately.
			if err = utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{NotReadyTaintTemplateForSched}, []*corev1.Taint{UnreachableTaintTemplateForSched}, cluster); err != nil {
				klog.ErrorS(err, "Failed to instantly update UnreachableTaintForSched to NotReadyTaintForSched, will try again in the next cycle.", "cluster", cluster.Name)
			}
		case metav1.ConditionUnknown:
			// Add UnreachableTaintTemplateForSched taint immediately.
			if err = utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{UnreachableTaintTemplateForSched}, []*corev1.Taint{NotReadyTaintTemplateForSched}, cluster); err != nil {
				klog.ErrorS(err, "Failed to instantly swap NotReadyTaintForSched to UnreachableTaintForSched, will try again in the next cycle.", "cluster", cluster.Name)
			}
		case metav1.ConditionTrue:
			if err = utilhelper.UpdateClusterControllerTaint(ctx, c.Client, nil, []*corev1.Taint{NotReadyTaintTemplateForSched, UnreachableTaintTemplateForSched}, cluster); err != nil {
				klog.ErrorS(err, "Failed to remove schedule taints from cluster, will retry in next iteration.", "cluster", cluster.Name)
			}
		}
	} else {
		// Add NotReadyTaintTemplateForSched taint immediately.
		if err = utilhelper.UpdateClusterControllerTaint(ctx, c.Client, []*corev1.Taint{NotReadyTaintTemplateForSched}, nil, cluster); err != nil {
			klog.ErrorS(err, "Failed to add a NotReady taint to the newly added cluster, will try again in the next cycle.", "cluster", cluster.Name)
		}
	}
	return err
}

func (c *Controller) collectIDForClusterObjectIfNeeded(ctx context.Context) (err error) {
	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		return err
	}

	clusters := clusterList.Items
	var errs []error

	for i := range clusters {
		cluster := &clusters[i]
		if cluster.Spec.ID != "" {
			continue
		}

		if cluster.Spec.SyncMode == clusterv1alpha1.Pull {
			continue
		}

		clusterClient, err := util.NewClusterClientSet(cluster.Name, c.Client, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s", cluster.Name, err.Error()))
			continue
		}

		id, err := util.ObtainClusterID(clusterClient.KubeClient)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s", cluster.Name, err.Error()))
			continue
		}
		cluster.Spec.ID = id

		err = c.Client.Update(ctx, cluster)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s", cluster.Name, err.Error()))
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}
