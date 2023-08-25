package execution

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/memberclusterinformer"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "execution-controller"
)

// Controller is to sync Work.
type Controller struct {
	client.Client      // used to operate Work resources.
	Ctx                context.Context
	EventRecorder      record.EventRecorder
	ObjectWatcher      objectwatcher.ObjectWatcher
	PredicateFunc      predicate.Predicate
	RatelimiterOptions ratelimiterflag.Options

	// Extend execution controller like work status controller to handle event from member cluster.
	worker util.AsyncWorker // worker process resources periodic from rateLimitingQueue.
	// ConcurrentExecutionSyncs is the number of object that are allowed to sync concurrently.
	ConcurrentWorkSyncs   int
	eventHandler          cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan              <-chan struct{}
	MemberClusterInformer memberclusterinformer.MemberClusterInformer
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	// Enqueue to try removing object in member cluster if work deleted.
	if !work.DeletionTimestamp.IsZero() {
		c.worker.Add(req)
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
		return controllerruntime.Result{Requeue: true}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", clusterName)
		return controllerruntime.Result{Requeue: true}, err
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop sync work(%s/%s) for cluster(%s) as cluster not ready.", work.Namespace, work.Name, cluster.Name)
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	err = c.MemberClusterInformer.BuildResourceInformers(cluster, work, c.eventHandler)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	c.worker.Add(req)

	return controllerruntime.Result{}, nil
}

func (c *Controller) onUpdate(old, cur interface{}) {
	oldObj := old.(*unstructured.Unstructured)
	curObj := cur.(*unstructured.Unstructured)

	oldObjCopy := oldObj.DeepCopy()
	curObjCopy := curObj.DeepCopy()

	clusterName, _ := names.GetClusterName(curObjCopy.GetLabels()[workv1alpha1.WorkNamespaceLabel])
	if clusterName == "" {
		return
	}

	if c.ObjectWatcher.NeedsUpdate(clusterName, oldObjCopy, curObjCopy) {
		c.worker.Enqueue(curObj)
	}
}

func (c *Controller) onDelete(obj interface{}) {
	curObj := obj.(*unstructured.Unstructured)

	c.worker.Enqueue(curObj)
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *Controller) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "work-execution",
		KeyFunc:       generateKey,
		ReconcileFunc: c.syncWork,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.ConcurrentWorkSyncs, c.StopChan)
}

// generateKey generates a key from obj, the key contains cluster, GVK, namespace and name.
func generateKey(obj interface{}) (util.QueueKey, error) {
	resource, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object is not unstructured")
	}

	workName := util.GetLabelValue(resource.GetLabels(), workv1alpha1.WorkNameLabel)
	workNamespace := util.GetLabelValue(resource.GetLabels(), workv1alpha1.WorkNamespaceLabel)

	if workName == "" || workNamespace == "" {
		return nil, nil
	}

	return controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: workNamespace, Name: workName}}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	c.eventHandler = fedinformer.NewHandlerOnEvents(nil, c.onUpdate, c.onDelete)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RatelimiterOptions),
		}).
		Complete(c)
}

func (c *Controller) syncWork(key util.QueueKey) error {
	req, ok := key.(controllerruntime.Request)
	if !ok {
		klog.Warningf("Skip sync work for key(%+v) is not controllerruntime.Request type", key)
		return nil
	}

	klog.Infof("Begin to sync work %s/%s", req.Namespace, req.Name)

	start := time.Now()

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(c.Ctx, req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	clusterName, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name for work %s/%s", work.Namespace, work.Name)
		return err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to get the given member cluster %s", clusterName)
		return err
	}

	if !work.DeletionTimestamp.IsZero() {
		// Abort deleting workload if cluster is unready when unjoining cluster, otherwise the unjoin process will be failed.
		if util.IsClusterReady(&cluster.Status) {
			err := c.tryDeleteWorkload(clusterName, work)
			if err != nil {
				klog.Errorf("Failed to delete work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
				return err
			}
		} else if cluster.DeletionTimestamp.IsZero() { // cluster is unready, but not terminating
			return fmt.Errorf("cluster(%s) not ready", cluster.Name)
		}

		return c.removeFinalizer(work)
	}

	if !util.IsClusterReady(&cluster.Status) {
		klog.Errorf("Stop sync work(%s/%s) for cluster(%s) as cluster not ready.", work.Namespace, work.Name, cluster.Name)
		return fmt.Errorf("cluster(%s) not ready", cluster.Name)
	}

	err = c.syncToClusters(clusterName, work)
	metrics.ObserveSyncWorkloadLatency(err, start)
	if err != nil {
		msg := fmt.Sprintf("Failed to sync work(%s) to cluster(%s): %v", work.Name, clusterName, err)
		klog.Errorf(msg)
		c.EventRecorder.Event(work, corev1.EventTypeWarning, events.EventReasonSyncWorkloadFailed, msg)
		return err
	}
	msg := fmt.Sprintf("Sync work(%s) to cluster(%s) successfully.", work.Name, clusterName)
	klog.V(4).Infof(msg)
	c.EventRecorder.Event(work, corev1.EventTypeNormal, events.EventReasonSyncWorkloadSucceed, msg)
	return nil
}

// tryDeleteWorkload tries to delete resource in the given member cluster.
func (c *Controller) tryDeleteWorkload(clusterName string, work *workv1alpha1.Work) error {
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			return err
		}

		fedKey, err := keys.FederatedKeyFunc(clusterName, workload)
		if err != nil {
			klog.Errorf("Failed to get FederatedKey %s, error: %v", workload.GetName(), err)
			return err
		}

		clusterObj, err := c.MemberClusterInformer.GetObjectFromCache(fedKey)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			klog.Errorf("Failed to get resource %v from member cluster, err is %v ", workload.GetName(), err)
			return err
		}

		// Avoid deleting resources that not managed by karmada.
		if util.GetLabelValue(clusterObj.GetLabels(), workv1alpha1.WorkNameLabel) != util.GetLabelValue(workload.GetLabels(), workv1alpha1.WorkNameLabel) ||
			util.GetLabelValue(clusterObj.GetLabels(), workv1alpha1.WorkNamespaceLabel) != util.GetLabelValue(workload.GetLabels(), workv1alpha1.WorkNamespaceLabel) {
			klog.Infof("Abort deleting the resource(kind=%s, %s/%s) exists in cluster %v but not managed by karmada", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), clusterName)
			return nil
		}

		err = c.ObjectWatcher.Delete(clusterName, workload)
		if err != nil {
			klog.Errorf("Failed to delete resource in the given member cluster %v, err is %v", clusterName, err)
			return err
		}
	}

	return nil
}

// removeFinalizer remove finalizer from the given Work
func (c *Controller) removeFinalizer(work *workv1alpha1.Work) error {
	if !controllerutil.ContainsFinalizer(work, util.ExecutionControllerFinalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(work, util.ExecutionControllerFinalizer)
	err := c.Client.Update(context.TODO(), work)
	if err != nil {
		return err
	}
	return nil
}

// syncToClusters ensures that the state of the given object is synchronized to member clusters.
func (c *Controller) syncToClusters(clusterName string, work *workv1alpha1.Work) error {
	var errs []error
	syncSucceedNum := 0
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		util.MergeLabel(workload, workv1alpha2.WorkUIDLabel, string(work.UID))
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
			errs = append(errs, err)
			continue
		}

		if err = c.tryCreateOrUpdateWorkload(clusterName, workload); err != nil {
			klog.Errorf("Failed to create or update resource(%v/%v) in the given member cluster %s, err is %v", workload.GetNamespace(), workload.GetName(), clusterName, err)
			c.eventf(workload, corev1.EventTypeWarning, events.EventReasonSyncWorkloadFailed, "Failed to create or update resource(%s) in member cluster(%s): %v", klog.KObj(workload), clusterName, err)
			errs = append(errs, err)
			continue
		}
		c.eventf(workload, corev1.EventTypeNormal, events.EventReasonSyncWorkloadSucceed, "Successfully applied resource(%v/%v) to cluster %s", workload.GetNamespace(), workload.GetName(), clusterName)
		syncSucceedNum++
	}

	if len(errs) > 0 {
		total := len(work.Spec.Workload.Manifests)
		message := fmt.Sprintf("Failed to apply all manifests (%d/%d): %s", syncSucceedNum, total, errors.NewAggregate(errs).Error())
		err := c.updateAppliedConditionIfNeed(work, metav1.ConditionFalse, "AppliedFailed", message)
		if err != nil {
			klog.Errorf("Failed to update applied status for given work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
			errs = append(errs, err)
		}
		return errors.NewAggregate(errs)
	}

	err := c.updateAppliedConditionIfNeed(work, metav1.ConditionTrue, "AppliedSuccessful", "Manifest has been successfully applied")
	if err != nil {
		klog.Errorf("Failed to update applied status for given work %v, namespace is %v, err is %v", work.Name, work.Namespace, err)
		return err
	}

	return nil
}

func (c *Controller) tryCreateOrUpdateWorkload(clusterName string, workload *unstructured.Unstructured) error {
	fedKey, err := keys.FederatedKeyFunc(clusterName, workload)
	if err != nil {
		klog.Errorf("Failed to get FederatedKey %s, error: %v", workload.GetName(), err)
		return err
	}

	clusterObj, err := c.MemberClusterInformer.GetObjectFromCache(fedKey)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get resource %v from member cluster, err is %v ", workload.GetName(), err)
			return err
		}
		err = c.ObjectWatcher.Create(clusterName, workload)
		if err != nil {
			klog.Errorf("Failed to create resource(%v/%v) in the given member cluster %s, err is %v", workload.GetNamespace(), workload.GetName(), clusterName, err)
			return err
		}
		return nil
	}

	err = c.ObjectWatcher.Update(clusterName, workload, clusterObj)
	if err != nil {
		klog.Errorf("Failed to update resource in the given member cluster %s, err is %v", clusterName, err)
		return err
	}
	return nil
}

// updateAppliedConditionIfNeed update the Applied condition for the given Work if the reason or message of Applied condition changed
func (c *Controller) updateAppliedConditionIfNeed(work *workv1alpha1.Work, status metav1.ConditionStatus, reason, message string) error {
	newWorkAppliedCondition := metav1.Condition{
		Type:               workv1alpha1.WorkApplied,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// needUpdateCondition judges if the Applied condition needs to update.
	needUpdateCondition := func() bool {
		lastWorkAppliedCondition := meta.FindStatusCondition(work.Status.Conditions, workv1alpha1.WorkApplied).DeepCopy()

		if lastWorkAppliedCondition != nil {
			lastWorkAppliedCondition.LastTransitionTime = newWorkAppliedCondition.LastTransitionTime

			return !reflect.DeepEqual(newWorkAppliedCondition, *lastWorkAppliedCondition)
		}

		return true
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		if !needUpdateCondition() {
			return nil
		}

		meta.SetStatusCondition(&work.Status.Conditions, newWorkAppliedCondition)
		updateErr := c.Status().Update(context.TODO(), work)
		if updateErr == nil {
			return nil
		}
		updated := &workv1alpha1.Work{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: work.Namespace, Name: work.Name}, updated); err == nil {
			work = updated
		} else {
			klog.Errorf("Failed to get updated work %s/%s: %v", work.Namespace, work.Name, err)
		}
		return updateErr
	})
}

func (c *Controller) eventf(object *unstructured.Unstructured, eventType, reason, messageFmt string, args ...interface{}) {
	ref, err := helper.GenEventRef(object)
	if err != nil {
		klog.Errorf("ignore event(%s) as failed to build event reference for: kind=%s, %s due to %v", reason, object.GetKind(), klog.KObj(object), err)
		return
	}
	c.EventRecorder.Eventf(ref, eventType, reason, messageFmt, args...)
}
