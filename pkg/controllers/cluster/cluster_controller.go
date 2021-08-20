package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
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

// Controller is to sync Cluster.
type Controller struct {
	client.Client // used to operate Cluster resources.
	EventRecorder record.EventRecorder

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

	// Per Cluster map stores last observed health together with a local time when it was observed.
	clusterHealthMap sync.Map
}

type clusterHealthData struct {
	probeTimestamp v1.Time
	status         *v1alpha1.ClusterStatus
	lease          *coordv1.Lease
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling cluster %s", req.NamespacedName.Name)

	cluster := &v1alpha1.Cluster{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return c.removeCluster(cluster)
	}

	return c.syncCluster(cluster)
}

// Start starts an asynchronous loop that monitors the status of cluster.
func (c *Controller) Start(ctx context.Context) error {
	klog.Infof("Starting cluster health monitor")
	defer klog.Infof("Shutting cluster health monitor")

	// Incorporate the results of cluster health signal pushed from cluster-status-controller to master.
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := c.monitorClusterHealth(); err != nil {
			klog.Errorf("Error monitoring cluster health: %v", err)
		}
	}, c.ClusterMonitorPeriod)
	<-ctx.Done()

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return utilerrors.NewAggregate([]error{
		controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.Cluster{}).Complete(c),
		mgr.Add(c),
	})
}

func (c *Controller) syncCluster(cluster *v1alpha1.Cluster) (controllerruntime.Result, error) {
	// create execution space
	err := c.createExecutionSpace(cluster)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	// ensure finalizer
	return c.ensureFinalizer(cluster)
}

func (c *Controller) removeCluster(cluster *v1alpha1.Cluster) (controllerruntime.Result, error) {
	err := c.removeExecutionSpace(cluster)
	if apierrors.IsNotFound(err) {
		return c.removeFinalizer(cluster)
	}
	if err != nil {
		klog.Errorf("Failed to remove execution space %v, err is %v", cluster.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	// make sure the given execution space has been deleted
	existES, err := c.ensureRemoveExecutionSpace(cluster)
	if err != nil {
		klog.Errorf("Failed to check weather the execution space exist in the given member cluster or not, error is: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	} else if existES {
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("requeuing operation until the execution space %v deleted, ", cluster.Name)
	}

	// delete the health data from the map explicitly after we removing the cluster.
	c.clusterHealthMap.Delete(cluster.Name)

	return c.removeFinalizer(cluster)
}

// removeExecutionSpace delete the given execution space
func (c *Controller) removeExecutionSpace(cluster *v1alpha1.Cluster) error {
	executionSpaceName, err := names.GenerateExecutionSpaceName(cluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
		return err
	}

	executionSpaceObj := &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: executionSpaceName,
		},
	}
	if err := c.Client.Delete(context.TODO(), executionSpaceObj); err != nil {
		klog.Errorf("Error while deleting namespace %s: %s", executionSpaceName, err)
		return err
	}

	return nil
}

// ensureRemoveExecutionSpace make sure the given execution space has been deleted
func (c *Controller) ensureRemoveExecutionSpace(cluster *v1alpha1.Cluster) (bool, error) {
	executionSpaceName, err := names.GenerateExecutionSpaceName(cluster.Name)
	if err != nil {
		klog.Errorf("Failed to generate execution space name for member cluster %s, err is %v", cluster.Name, err)
		return false, err
	}

	executionSpaceObj := &corev1.Namespace{}
	err = c.Client.Get(context.TODO(), types.NamespacedName{Name: executionSpaceName}, executionSpaceObj)
	if apierrors.IsNotFound(err) {
		klog.V(2).Infof("Removed execution space(%s)", executionSpaceName)
		return false, nil
	}
	if err != nil {
		klog.Errorf("Failed to get execution space %v, err is %v ", executionSpaceName, err)
		return false, err
	}
	return true, nil
}

func (c *Controller) removeFinalizer(cluster *v1alpha1.Cluster) (controllerruntime.Result, error) {
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

func (c *Controller) ensureFinalizer(cluster *v1alpha1.Cluster) (controllerruntime.Result, error) {
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

// createExecutionSpace create member cluster execution space when member cluster joined
func (c *Controller) createExecutionSpace(cluster *v1alpha1.Cluster) error {
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
			ObjectMeta: v1.ObjectMeta{
				Name: executionSpaceName,
			},
		}
		err := c.Client.Create(context.TODO(), executionSpace)
		if err != nil {
			klog.Errorf("Failed to create execution space for cluster(%s): %v", cluster.Name, err)
			return err
		}
		klog.V(2).Infof("Created execution space(%s) for cluster(%s).", executionSpaceName, cluster.Name)
	}

	return nil
}

func (c *Controller) monitorClusterHealth() error {
	clusterList := &v1alpha1.ClusterList{}
	if err := c.Client.List(context.TODO(), clusterList); err != nil {
		return err
	}

	clusters := clusterList.Items
	for i := range clusters {
		cluster := &clusters[i]
		if err := wait.PollImmediate(MonitorRetrySleepTime, MonitorRetrySleepTime*HealthUpdateRetry, func() (bool, error) {
			// Cluster object may be changed in this function.
			if err := c.tryUpdateClusterHealth(cluster); err == nil {
				return true, nil
			}
			clusterName := cluster.Name
			if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, cluster); err != nil {
				// If the cluster does not exist any more, we delete the health data from the map.
				if apierrors.IsNotFound(err) {
					c.clusterHealthMap.Delete(clusterName)
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
	}

	return nil
}

// tryUpdateClusterHealth checks a given cluster's conditions and tries to update it.
//nolint:gocyclo
func (c *Controller) tryUpdateClusterHealth(cluster *v1alpha1.Cluster) error {
	// Step 1: Get the last cluster heath from `clusterHealthMap`.
	var clusterHealth *clusterHealthData
	if value, exists := c.clusterHealthMap.Load(cluster.Name); exists {
		clusterHealth = value.(*clusterHealthData)
	}

	defer func() {
		c.clusterHealthMap.Store(cluster.Name, clusterHealth)
	}()

	// Step 2: Get the cluster ready condition.
	var gracePeriod time.Duration
	var observedReadyCondition *v1.Condition
	currentReadyCondition := meta.FindStatusCondition(cluster.Status.Conditions, v1alpha1.ClusterConditionReady)
	if currentReadyCondition == nil {
		// If ready condition is nil, then cluster-status-controller has never posted cluster status.
		// A fake ready condition is created, where LastTransitionTime is set to cluster.CreationTimestamp
		// to avoid handle the corner case.
		observedReadyCondition = &v1.Condition{
			Type:               v1alpha1.ClusterConditionReady,
			Status:             v1.ConditionUnknown,
			LastTransitionTime: cluster.CreationTimestamp,
		}
		gracePeriod = c.ClusterStartupGracePeriod
		if clusterHealth != nil {
			clusterHealth.status = &cluster.Status
		} else {
			clusterHealth = &clusterHealthData{
				status:         &cluster.Status,
				probeTimestamp: cluster.CreationTimestamp,
			}
		}
	} else {
		// If ready condition is not nil, make a copy of it, since we may modify it in place later.
		observedReadyCondition = currentReadyCondition.DeepCopy()
		gracePeriod = c.ClusterMonitorGracePeriod
	}

	// Step 3: Get the last condition and lease from `clusterHealth`.
	var savedCondition *v1.Condition
	var savedLease *coordv1.Lease
	if clusterHealth != nil {
		savedCondition = meta.FindStatusCondition(clusterHealth.status.Conditions, v1alpha1.ClusterConditionReady)
		savedLease = clusterHealth.lease
	}

	// Step 4: Update the clusterHealth if necessary.
	// If this condition have no difference from last condition, we leave everything as it is.
	// Otherwise, we only update the probeTimestamp.
	if clusterHealth == nil || !equality.Semantic.DeepEqual(savedCondition, currentReadyCondition) {
		clusterHealth = &clusterHealthData{
			status:         cluster.Status.DeepCopy(),
			probeTimestamp: v1.Now(),
		}
	}
	// Always update the probe time if cluster lease is renewed.
	// Note: If cluster-status-controller never posted the cluster status, but continues renewing the
	// heartbeat leases, the cluster controller will assume the cluster is healthy and take no action.
	observedLease := &coordv1.Lease{}
	err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: util.NamespaceClusterLease, Name: cluster.Name}, observedLease)
	if err == nil && (savedLease == nil || savedLease.Spec.RenewTime.Before(observedLease.Spec.RenewTime)) {
		clusterHealth.lease = observedLease
		clusterHealth.probeTimestamp = v1.Now()
	}

	// Step 5: Check whether the probe timestamp has timed out.
	if v1.Now().After(clusterHealth.probeTimestamp.Add(gracePeriod)) {
		clusterConditionTypes := []string{
			v1alpha1.ClusterConditionReady,
		}

		nowTimestamp := v1.Now()
		for _, clusterConditionType := range clusterConditionTypes {
			currentCondition := meta.FindStatusCondition(cluster.Status.Conditions, clusterConditionType)
			if currentCondition == nil {
				klog.V(2).Infof("Condition %v of cluster %v was never updated by cluster-status-controller",
					clusterConditionType, cluster.Name)
				cluster.Status.Conditions = append(cluster.Status.Conditions, v1.Condition{
					Type:               clusterConditionType,
					Status:             v1.ConditionUnknown,
					Reason:             "ClusterStatusNeverUpdated",
					Message:            "Cluster status controller never posted cluster status.",
					LastTransitionTime: nowTimestamp,
				})
			} else {
				klog.V(2).Infof("cluster %v hasn't been updated for %+v. Last %v is: %+v",
					cluster.Name, v1.Now().Time.Sub(clusterHealth.probeTimestamp.Time), clusterConditionType, currentCondition)
				if currentCondition.Status != v1.ConditionUnknown {
					currentCondition.Status = v1.ConditionUnknown
					currentCondition.Reason = "ClusterStatusUnknown"
					currentCondition.Message = "Cluster status controller stopped posting cluster status."
					currentCondition.LastTransitionTime = nowTimestamp
				}
			}
		}
		// We need to update currentReadyCondition due to its value potentially changed.
		currentReadyCondition = meta.FindStatusCondition(cluster.Status.Conditions, v1alpha1.ClusterConditionReady)

		if !equality.Semantic.DeepEqual(currentReadyCondition, observedReadyCondition) {
			if err := c.Status().Update(context.TODO(), cluster); err != nil {
				klog.Errorf("Error updating cluster %s: %v", cluster.Name, err)
				return err
			}
			clusterHealth = &clusterHealthData{
				status:         &cluster.Status,
				probeTimestamp: clusterHealth.probeTimestamp,
				lease:          observedLease,
			}
			return nil
		}
	}
	return nil
}
