package status

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/memberclusterinformer"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// WorkStatusController is to sync status of Work.
type WorkStatusController struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
	eventHandler  cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan      <-chan struct{}
	worker        util.AsyncWorker // worker process resources periodic from rateLimitingQueue.
	// ConcurrentWorkStatusSyncs is the number of Work status that are allowed to sync concurrently.
	ConcurrentWorkStatusSyncs int
	PredicateFunc             predicate.Predicate
	RateLimiterOptions        ratelimiterflag.Options
	ResourceInterpreter       resourceinterpreter.ResourceInterpreter
	MemberClusterInformer     memberclusterinformer.MemberClusterInformer
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *WorkStatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of Work %s.", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if !helper.IsResourceApplied(&work.Status) {
		return controllerruntime.Result{}, nil
	}

	clusterName, err := names.GetClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return controllerruntime.Result{Requeue: true}, err
	}

	cluster, err := util.GetCluster(c.Client, clusterName)
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", clusterName)
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

	return controllerruntime.Result{}, nil
}

func (c *WorkStatusController) onAdd(obj interface{}) {
	curObj := obj.(runtime.Object)
	c.worker.Enqueue(curObj)
}

func (c *WorkStatusController) onUpdate(old, cur interface{}) {
	// Still need to compare the whole object because the interpreter is able to take any part of object to reflect status.
	if !reflect.DeepEqual(old, cur) {
		curObj := cur.(runtime.Object)
		c.worker.Enqueue(curObj)
	}
}

// RunWorkQueue initializes worker and run it, worker will process resource asynchronously.
func (c *WorkStatusController) RunWorkQueue() {
	workerOptions := util.Options{
		Name:          "work-status",
		KeyFunc:       generateKey,
		ReconcileFunc: c.syncWorkStatus,
	}
	c.worker = util.NewAsyncWorker(workerOptions)
	c.worker.Run(c.ConcurrentWorkStatusSyncs, c.StopChan)
}

// generateKey generates a key from obj, the key contains cluster, GVK, namespace and name.
func generateKey(obj interface{}) (util.QueueKey, error) {
	resource := obj.(*unstructured.Unstructured)
	cluster, err := getClusterNameFromLabel(resource)
	if err != nil {
		return nil, err
	}
	// return a nil key when the obj not managed by Karmada, which will be discarded before putting to queue.
	if cluster == "" {
		return nil, nil
	}

	return keys.FederatedKeyFunc(cluster, obj)
}

// getClusterNameFromLabel gets cluster name from ownerLabel, if label not exist, means resource is not created by karmada.
func getClusterNameFromLabel(resource *unstructured.Unstructured) (string, error) {
	workNamespace := util.GetLabelValue(resource.GetLabels(), workv1alpha1.WorkNamespaceLabel)
	if workNamespace == "" {
		klog.V(4).Infof("Ignore resource(%s/%s/%s) which not managed by karmada", resource.GetKind(), resource.GetNamespace(), resource.GetName())
		return "", nil
	}

	cluster, err := names.GetClusterName(workNamespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name from work namespace: %s, error: %v.", workNamespace, err)
		return "", err
	}
	return cluster, nil
}

// syncWorkStatus will collect status of object referencing by key and update to work which holds the object.
func (c *WorkStatusController) syncWorkStatus(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync status as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}

	klog.Infof("Begin to sync status to Work of object(%s)", fedKey.String())

	observedObj, err := c.MemberClusterInformer.GetObjectFromCache(fedKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			executionSpace := names.GenerateExecutionSpaceName(fedKey.Cluster)
			workName := names.GenerateWorkName(fedKey.Kind, fedKey.Name, fedKey.Namespace)
			klog.Warningf("Skip reflecting %s(%s/%s) status to Work(%s/%s) for not applied successfully yet.",
				fedKey.Kind, fedKey.Namespace, fedKey.Name, executionSpace, workName)
			return nil
		}
		return err
	}

	workNamespace := util.GetLabelValue(observedObj.GetLabels(), workv1alpha1.WorkNamespaceLabel)
	workName := util.GetLabelValue(observedObj.GetLabels(), workv1alpha1.WorkNameLabel)
	if workNamespace == "" || workName == "" {
		klog.Infof("Ignore object(%s) which not managed by karmada.", fedKey.String())
		return nil
	}

	workObject := &workv1alpha1.Work{}
	if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: workNamespace, Name: workName}, workObject); err != nil {
		// Stop processing if resource no longer exist.
		if apierrors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get Work(%s/%s) from cache: %v", workNamespace, workName, err)
		return err
	}

	// stop update status if Work object in terminating state.
	if !workObject.DeletionTimestamp.IsZero() {
		return nil
	}

	if !helper.IsResourceApplied(&workObject.Status) {
		err = fmt.Errorf("work hadn't been applied yet")
		klog.Errorf("Failed to reflect status of %s(%s/%s) to Work(%s/%s): %v.",
			observedObj.GetKind(), observedObj.GetNamespace(), observedObj.GetName(), workNamespace, workName, err)
		return err
	}

	klog.Infof("reflecting %s(%s/%s) status to Work(%s/%s)", observedObj.GetKind(), observedObj.GetNamespace(), observedObj.GetName(), workNamespace, workName)
	return c.reflectStatus(workObject, observedObj)
}

// reflectStatus grabs cluster object's running status then updates to its owner object(Work).
func (c *WorkStatusController) reflectStatus(work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) error {
	statusRaw, err := c.ResourceInterpreter.ReflectStatus(clusterObj)
	if err != nil {
		klog.Errorf("Failed to reflect status for object(%s/%s/%s) with resourceInterpreter.",
			clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err)
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonReflectStatusFailed, "Reflect status for object(%s/%s/%s) failed, err: %s.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err.Error())
		return err
	}
	c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonReflectStatusSucceed, "Reflect status for object(%s/%s/%s) succeed.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())

	if statusRaw == nil {
		return nil
	}

	var resourceHealth workv1alpha1.ResourceHealth
	// When an unregistered resource kind is requested with the ResourceInterpreter,
	// the interpreter will return an error, we treat its health status as Unknown.
	healthy, err := c.ResourceInterpreter.InterpretHealth(clusterObj)
	if err != nil {
		resourceHealth = workv1alpha1.ResourceUnknown
		c.EventRecorder.Eventf(work, corev1.EventTypeWarning, events.EventReasonInterpretHealthFailed, "Interpret health of object(%s/%s/%s) failed, err: %s.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName(), err.Error())
	} else if healthy {
		resourceHealth = workv1alpha1.ResourceHealthy
		c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonInterpretHealthSucceed, "Interpret health of object(%s/%s/%s) as healthy.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())
	} else {
		resourceHealth = workv1alpha1.ResourceUnhealthy
		c.EventRecorder.Eventf(work, corev1.EventTypeNormal, events.EventReasonInterpretHealthSucceed, "Interpret health of object(%s/%s/%s) as unhealthy.", clusterObj.GetKind(), clusterObj.GetNamespace(), clusterObj.GetName())
	}

	identifier, err := c.buildStatusIdentifier(work, clusterObj)
	if err != nil {
		return err
	}

	manifestStatus := workv1alpha1.ManifestStatus{
		Identifier: *identifier,
		Status:     statusRaw,
		Health:     resourceHealth,
	}

	workCopy := work.DeepCopy()
	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		manifestStatuses := c.mergeStatus(workCopy.Status.ManifestStatuses, manifestStatus)
		if reflect.DeepEqual(workCopy.Status.ManifestStatuses, manifestStatuses) {
			return nil
		}
		workCopy.Status.ManifestStatuses = manifestStatuses
		updateErr := c.Status().Update(context.TODO(), workCopy)
		if updateErr == nil {
			return nil
		}

		updated := &workv1alpha1.Work{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: workCopy.Namespace, Name: workCopy.Name}, updated); err == nil {
			workCopy = updated
		} else {
			klog.Errorf("Failed to get updated work %s/%s: %v", workCopy.Namespace, workCopy.Name, err)
		}
		return updateErr
	})
}

func (c *WorkStatusController) buildStatusIdentifier(work *workv1alpha1.Work, clusterObj *unstructured.Unstructured) (*workv1alpha1.ResourceIdentifier, error) {
	ordinal, err := helper.GetManifestIndex(work.Spec.Workload.Manifests, clusterObj)
	if err != nil {
		return nil, err
	}

	groupVersion, err := schema.ParseGroupVersion(clusterObj.GetAPIVersion())
	if err != nil {
		return nil, err
	}

	identifier := &workv1alpha1.ResourceIdentifier{
		Ordinal: ordinal,
		// TODO(RainbowMango): Consider merge Group and Version to APIVersion from Work API.
		Group:     groupVersion.Group,
		Version:   groupVersion.Version,
		Kind:      clusterObj.GetKind(),
		Namespace: clusterObj.GetNamespace(),
		Name:      clusterObj.GetName(),
	}

	return identifier, nil
}

func (c *WorkStatusController) mergeStatus(_ []workv1alpha1.ManifestStatus, newStatus workv1alpha1.ManifestStatus) []workv1alpha1.ManifestStatus {
	// TODO(RainbowMango): update 'statuses' if 'newStatus' already exist.
	// For now, we only have at most one manifest in Work, so just override current 'statuses'.
	return []workv1alpha1.ManifestStatus{newStatus}
}

// SetupWithManager creates a controller and register to controller manager.
func (c *WorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.eventHandler = fedinformer.NewHandlerOnEvents(c.onAdd, c.onUpdate, nil)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&workv1alpha1.Work{}, builder.WithPredicates(c.PredicateFunc)).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions),
		}).Complete(c)
}
