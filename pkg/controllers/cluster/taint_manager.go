package cluster

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// TaintManagerName is the controller name that will be used for taint management.
const TaintManagerName = "taint-manager"

// NoExecuteTaintManager listens to Taint/Toleration changes and is responsible for removing objects
// from Clusters tainted with NoExecute Taints.
type NoExecuteTaintManager struct {
	client.Client // used to operate Cluster resources.
	EventRecorder record.EventRecorder

	ClusterTaintEvictionRetryFrequency time.Duration
	ConcurrentReconciles               int

	bindingEvictionWorker        util.AsyncWorker
	clusterBindingEvictionWorker util.AsyncWorker
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (tc *NoExecuteTaintManager) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("Reconciling cluster %s for taint manager", req.NamespacedName.Name)

	cluster := &clusterv1alpha1.Cluster{}
	if err := tc.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	// Check whether the target cluster has no execute taints.
	if !helper.HasNoExecuteTaints(cluster.Spec.Taints) {
		return controllerruntime.Result{}, nil
	}

	return tc.syncCluster(ctx, cluster)
}

func (tc *NoExecuteTaintManager) syncCluster(ctx context.Context, cluster *clusterv1alpha1.Cluster) (reconcile.Result, error) {
	// List all ResourceBindings which are assigned to this cluster.
	rbList := &workv1alpha2.ResourceBindingList{}
	if err := tc.List(ctx, rbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(rbClusterKeyIndex, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ResourceBindings", "cluster", cluster.Name)
		return controllerruntime.Result{Requeue: true}, err
	}
	for i := range rbList.Items {
		key, err := keys.FederatedKeyFunc(cluster.Name, &rbList.Items[i])
		if err != nil {
			klog.Warningf("Failed to generate key for obj: %s", rbList.Items[i].GetObjectKind().GroupVersionKind())
			continue
		}
		tc.bindingEvictionWorker.Add(key)
	}

	// List all ClusterResourceBindings which are assigned to this cluster.
	crbList := &workv1alpha2.ClusterResourceBindingList{}
	if err := tc.List(ctx, crbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(crbClusterKeyIndex, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ClusterResourceBindings", "cluster", cluster.Name)
		return controllerruntime.Result{Requeue: true}, err
	}
	for i := range crbList.Items {
		key, err := keys.FederatedKeyFunc(cluster.Name, &crbList.Items[i])
		if err != nil {
			klog.Warningf("Failed to generate key for obj: %s", crbList.Items[i].GetObjectKind().GroupVersionKind())
			continue
		}
		tc.clusterBindingEvictionWorker.Add(key)
	}
	return controllerruntime.Result{RequeueAfter: tc.ClusterTaintEvictionRetryFrequency}, nil
}

// Start starts an asynchronous loop that handle evictions.
func (tc *NoExecuteTaintManager) Start(ctx context.Context) error {
	bindingEvictionWorkerOptions := util.Options{
		Name:          "binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: tc.syncBindingEviction,
	}
	tc.bindingEvictionWorker = util.NewAsyncWorker(bindingEvictionWorkerOptions)
	tc.bindingEvictionWorker.Run(tc.ConcurrentReconciles, ctx.Done())

	clusterBindingEvictionWorkerOptions := util.Options{
		Name:          "cluster-binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: tc.syncClusterBindingEviction,
	}
	tc.clusterBindingEvictionWorker = util.NewAsyncWorker(clusterBindingEvictionWorkerOptions)
	tc.clusterBindingEvictionWorker.Run(tc.ConcurrentReconciles, ctx.Done())

	<-ctx.Done()
	return nil
}

func (tc *NoExecuteTaintManager) syncBindingEviction(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync binding eviction as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}
	cluster := fedKey.Cluster

	binding := &workv1alpha2.ResourceBinding{}
	if err := tc.Client.Get(context.TODO(), types.NamespacedName{Namespace: fedKey.Namespace, Name: fedKey.Name}, binding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get binding %s: %v", fedKey.NamespaceKey(), err)
	}

	if !binding.DeletionTimestamp.IsZero() || !binding.Spec.TargetContains(cluster) {
		return nil
	}

	needEviction, tolerationTime, err := tc.needEviction(cluster, binding.Annotations)
	if err != nil {
		klog.ErrorS(err, "Failed to check if binding needs eviction", "binding", fedKey.ClusterWideKey.NamespaceKey())
		return err
	}

	// Case 1: Need eviction now.
	// Case 2: Need eviction after toleration time. If time is up, do eviction right now.
	// Case 3: Tolerate forever, we do nothing.
	if needEviction || tolerationTime == 0 {
		// update final result to evict the target cluster
		if features.FeatureGate.Enabled(features.GracefulEviction) {
			binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.EvictionProducerTaintManager, workv1alpha2.EvictionReasonTaintUntolerated, "")
		} else {
			binding.Spec.RemoveCluster(cluster)
		}
		if err = tc.Update(context.TODO(), binding); err != nil {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, tc.EventRecorder, err)
			klog.ErrorS(err, "Failed to update binding", "binding", klog.KObj(binding))
			return err
		}
		if !features.FeatureGate.Enabled(features.GracefulEviction) {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, tc.EventRecorder, nil)
		}
	} else if tolerationTime > 0 {
		tc.bindingEvictionWorker.AddAfter(fedKey, tolerationTime)
	}

	return nil
}

func (tc *NoExecuteTaintManager) syncClusterBindingEviction(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		klog.Errorf("Failed to sync cluster binding eviction as invalid key: %v", key)
		return fmt.Errorf("invalid key")
	}
	cluster := fedKey.Cluster

	binding := &workv1alpha2.ClusterResourceBinding{}
	if err := tc.Client.Get(context.TODO(), types.NamespacedName{Name: fedKey.Name}, binding); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get cluster binding %s: %v", fedKey.Name, err)
	}

	if !binding.DeletionTimestamp.IsZero() || !binding.Spec.TargetContains(cluster) {
		return nil
	}

	needEviction, tolerationTime, err := tc.needEviction(cluster, binding.Annotations)
	if err != nil {
		klog.ErrorS(err, "Failed to check if cluster binding needs eviction", "binding", binding.Name)
		return err
	}

	// Case 1: Need eviction now.
	// Case 2: Need eviction after toleration time. If time is up, do eviction right now.
	// Case 3: Tolerate forever, we do nothing.
	if needEviction || tolerationTime == 0 {
		// update final result to evict the target cluster
		if features.FeatureGate.Enabled(features.GracefulEviction) {
			binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.EvictionProducerTaintManager, workv1alpha2.EvictionReasonTaintUntolerated, "")
		} else {
			binding.Spec.RemoveCluster(cluster)
		}
		if err = tc.Update(context.TODO(), binding); err != nil {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, tc.EventRecorder, err)
			klog.ErrorS(err, "Failed to update cluster binding", "binding", binding.Name)
			return err
		}
		if !features.FeatureGate.Enabled(features.GracefulEviction) {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, tc.EventRecorder, nil)
		}
	} else if tolerationTime > 0 {
		tc.clusterBindingEvictionWorker.AddAfter(fedKey, tolerationTime)
		return nil
	}

	return nil
}

// needEviction returns whether the binding should be evicted from target cluster right now.
// If a toleration time is found, we return false along with a minimum toleration time as the
// second return value.
func (tc *NoExecuteTaintManager) needEviction(clusterName string, appliedPlacement map[string]string) (bool, time.Duration, error) {
	placement, err := helper.GetAppliedPlacement(appliedPlacement)
	if err != nil {
		return false, -1, err
	}

	cluster := &clusterv1alpha1.Cluster{}
	if err = tc.Client.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return false, -1, nil
		}
		return false, -1, err
	}

	taints := helper.GetNoExecuteTaints(cluster.Spec.Taints)
	if len(taints) == 0 {
		return false, -1, nil
	}
	tolerations := placement.ClusterTolerations

	allTolerated, usedTolerations := helper.GetMatchingTolerations(taints, tolerations)
	if !allTolerated {
		return true, 0, nil
	}

	return false, helper.GetMinTolerationTime(taints, usedTolerations), nil
}

// SetupWithManager creates a controller and register to controller manager.
func (tc *NoExecuteTaintManager) SetupWithManager(mgr controllerruntime.Manager) error {
	return utilerrors.NewAggregate([]error{
		controllerruntime.NewControllerManagedBy(mgr).For(&clusterv1alpha1.Cluster{}).Complete(tc),
		mgr.Add(tc),
	})
}
