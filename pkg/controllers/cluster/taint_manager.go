/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

// TaintManagerName is the controller name that will be used when reporting events and metrics.
const TaintManagerName = "taint-manager"

// NoExecuteTaintManager listens to Taint/Toleration changes and is responsible for removing objects
// from Clusters tainted with NoExecute Taints.
type NoExecuteTaintManager struct {
	client.Client // used to operate Cluster resources.
	EventRecorder record.EventRecorder

	ClusterTaintEvictionRetryFrequency time.Duration
	ConcurrentReconciles               int
	RateLimiterOptions                 ratelimiterflag.Options

	EnableNoExecuteTaintEviction    bool
	NoExecuteTaintEvictionPurgeMode string

	bindingEvictionWorker        util.AsyncWorker
	clusterBindingEvictionWorker util.AsyncWorker
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (tc *NoExecuteTaintManager) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.V(4).InfoS("Reconciling cluster for taint manager", "cluster", req.NamespacedName.Name)

	cluster := &clusterv1alpha1.Cluster{}
	if err := tc.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	// short path to skip reconciliation if NoExecute taint eviction is disabled or no target taints on Cluster
	if !tc.EnableNoExecuteTaintEviction || !helper.HasNoExecuteTaints(cluster.Spec.Taints) {
		return controllerruntime.Result{}, nil
	}

	return tc.syncCluster(ctx, cluster)
}

func (tc *NoExecuteTaintManager) syncCluster(ctx context.Context, cluster *clusterv1alpha1.Cluster) (reconcile.Result, error) {
	// List all ResourceBindings which are assigned to this cluster.
	rbList := &workv1alpha2.ResourceBindingList{}
	if err := tc.List(ctx, rbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(indexregistry.ResourceBindingIndexByFieldCluster, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ResourceBindings", "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}
	for i := range rbList.Items {
		key, err := keys.FederatedKeyFunc(cluster.Name, &rbList.Items[i])
		if err != nil {
			klog.ErrorS(err, "Failed to generate federated key for ResourceBinding", "gvk", rbList.Items[i].GetObjectKind().GroupVersionKind())
			continue
		}
		tc.bindingEvictionWorker.Add(key)
	}

	// List all ClusterResourceBindings which are assigned to this cluster.
	crbList := &workv1alpha2.ClusterResourceBindingList{}
	if err := tc.List(ctx, crbList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector(indexregistry.ClusterResourceBindingIndexByFieldCluster, cluster.Name),
	}); err != nil {
		klog.ErrorS(err, "Failed to list ClusterResourceBindings", "cluster", cluster.Name)
		return controllerruntime.Result{}, err
	}
	for i := range crbList.Items {
		key, err := keys.FederatedKeyFunc(cluster.Name, &crbList.Items[i])
		if err != nil {
			klog.ErrorS(err, "Failed to generate federated key for ClusterResourceBinding", "gvk", crbList.Items[i].GetObjectKind().GroupVersionKind())
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
	tc.bindingEvictionWorker.Run(ctx, tc.ConcurrentReconciles)

	clusterBindingEvictionWorkerOptions := util.Options{
		Name:          "cluster-binding-eviction",
		KeyFunc:       nil,
		ReconcileFunc: tc.syncClusterBindingEviction,
	}
	tc.clusterBindingEvictionWorker = util.NewAsyncWorker(clusterBindingEvictionWorkerOptions)
	tc.clusterBindingEvictionWorker.Run(ctx, tc.ConcurrentReconciles)

	<-ctx.Done()
	return nil
}

func (tc *NoExecuteTaintManager) syncBindingEviction(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		err := fmt.Errorf("invalid key type %T", key)
		klog.ErrorS(err, "Failed to sync binding eviction due to invalid key", "key", key)
		return err
	}
	cluster := fedKey.Cluster
	klog.V(4).InfoS("Begin syncing ResourceBinding for taint eviction", "binding", fedKey.ClusterWideKey.NamespaceKey(), "cluster", cluster)

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

	var purgeMode policyv1alpha1.PurgeMode
	if needEviction {
		switch tc.NoExecuteTaintEvictionPurgeMode {
		case "Gracefully":
			purgeMode = policyv1alpha1.Graciously
		case "Directly":
			purgeMode = policyv1alpha1.Immediately
		}
	}

	// Case 1: Need eviction now.
	// Case 2: Need eviction after toleration time. If time is up, do eviction right now.
	// Case 3: Tolerate forever, we do nothing.
	if needEviction || tolerationTime == 0 {
		// update final result to evict the target cluster
		binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(
			workv1alpha2.WithPurgeMode(purgeMode),
			workv1alpha2.WithProducer(workv1alpha2.EvictionProducerTaintManager),
			workv1alpha2.WithReason(workv1alpha2.EvictionReasonTaintUntolerated)))

		if err = tc.Update(context.TODO(), binding); err != nil {
			helper.EmitClusterEvictionEventForResourceBinding(binding, cluster, tc.EventRecorder, err)
			klog.ErrorS(err, "Failed to update binding", "binding", klog.KObj(binding))
			return err
		}
		klog.V(2).InfoS("Evicted cluster from ResourceBinding", "cluster", fedKey.Cluster, "binding", fedKey.ClusterWideKey.NamespaceKey())
	} else if tolerationTime > 0 {
		tc.bindingEvictionWorker.AddAfter(fedKey, tolerationTime)
	}

	return nil
}

func (tc *NoExecuteTaintManager) syncClusterBindingEviction(key util.QueueKey) error {
	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		err := fmt.Errorf("invalid key type %T", key)
		klog.ErrorS(err, "Failed to sync cluster binding eviction due to invalid key", "key", key)
		return err
	}
	cluster := fedKey.Cluster
	klog.V(4).InfoS("Begin syncing ClusterResourceBinding with taintManager", "binding", fedKey.ClusterWideKey.NamespaceKey(), "cluster", cluster)

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

	var purgeMode policyv1alpha1.PurgeMode
	if needEviction {
		switch tc.NoExecuteTaintEvictionPurgeMode {
		case "Gracefully":
			purgeMode = policyv1alpha1.Graciously
		case "Directly":
			purgeMode = policyv1alpha1.Immediately
		}
	}

	// Case 1: Need eviction now.
	// Case 2: Need eviction after toleration time. If time is up, do eviction right now.
	// Case 3: Tolerate forever, we do nothing.
	if needEviction || tolerationTime == 0 {
		// update final result to evict the target cluster
		binding.Spec.GracefulEvictCluster(cluster, workv1alpha2.NewTaskOptions(
			workv1alpha2.WithPurgeMode(purgeMode),
			workv1alpha2.WithProducer(workv1alpha2.EvictionProducerTaintManager),
			workv1alpha2.WithReason(workv1alpha2.EvictionReasonTaintUntolerated)))
		if err = tc.Update(context.TODO(), binding); err != nil {
			helper.EmitClusterEvictionEventForClusterResourceBinding(binding, cluster, tc.EventRecorder, err)
			klog.ErrorS(err, "Failed to update cluster binding", "binding", binding.Name)
			return err
		}
		klog.V(2).InfoS("Evicted cluster from ClusterResourceBinding", "cluster", fedKey.Cluster, "binding", fedKey.ClusterWideKey.NamespaceKey())
	} else if tolerationTime > 0 {
		tc.clusterBindingEvictionWorker.AddAfter(fedKey, tolerationTime)
		return nil
	}

	return nil
}

// needEviction returns whether the binding should be evicted from target cluster right now.
// If a toleration time is found, we return false along with a minimum toleration time as the
// second return value.
func (tc *NoExecuteTaintManager) needEviction(clusterName string, annotations map[string]string) (bool, time.Duration, error) {
	placement, err := helper.GetAppliedPlacement(annotations)
	if err != nil {
		return false, -1, err
	}
	// Under normal circumstances, placement will not be empty,
	// but when the default scheduler is not used, coordination problems may occur.
	// Therefore, in order to make the method more robust, add the empty judgement.
	if placement == nil {
		return false, -1, fmt.Errorf("the applied placement for ResourceBining can not be empty")
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
	if !tc.EnableNoExecuteTaintEviction || len(taints) == 0 {
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
		controllerruntime.NewControllerManagedBy(mgr).Named(TaintManagerName).For(&clusterv1alpha1.Cluster{}).
			WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](tc.RateLimiterOptions)}).Complete(tc),
		mgr.Add(tc),
	})
}
