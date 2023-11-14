package hpareplicassyncer

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// ControllerName is the controller name that will be used when reporting events.
	ControllerName = "hpa-replicas-syncer"
	// scaleRefWorkerNum is the async Worker number
	scaleRefWorkerNum = 1
)

// HPAReplicasSyncer is to sync replicas from status of HPA to resource template.
type HPAReplicasSyncer struct {
	Client        client.Client
	DynamicClient dynamic.Interface
	RESTMapper    meta.RESTMapper

	ScaleClient    scale.ScalesGetter
	scaleRefWorker util.AsyncWorker
}

// SetupWithManager creates a controller and register to controller manager.
func (r *HPAReplicasSyncer) SetupWithManager(mgr controllerruntime.Manager) error {
	scaleRefWorkerOptions := util.Options{
		Name:          "scale ref worker",
		ReconcileFunc: r.reconcileScaleRef,
	}
	r.scaleRefWorker = util.NewAsyncWorker(scaleRefWorkerOptions)
	r.scaleRefWorker.Run(scaleRefWorkerNum, context.Background().Done())

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&autoscalingv2.HorizontalPodAutoscaler{}, builder.WithPredicates(r)).
		Complete(r)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *HPAReplicasSyncer) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling for HPA %s/%s", req.Namespace, req.Name)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Client.Get(ctx, req.NamespacedName, hpa)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	workloadGR, scale, err := r.getGroupResourceAndScaleForWorkloadFromHPA(ctx, hpa)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	synced := isReplicasSynced(scale, hpa)

	if !hpa.DeletionTimestamp.IsZero() && synced {
		return controllerruntime.Result{}, r.removeFinalizerIfNeed(ctx, hpa)
	}

	err = r.addFinalizerIfNeed(ctx, hpa)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	if !synced {
		err = r.updateScale(ctx, workloadGR, scale, hpa)
		if err != nil {
			return controllerruntime.Result{}, err
		}

		// If it needs to sync, we had better to requeue once immediately to make sure
		// the syncing successfully and the finalizer removed successfully.
		return controllerruntime.Result{Requeue: true}, nil
	}

	return controllerruntime.Result{}, nil
}

// getGroupResourceAndScaleForWorkloadFromHPA parses GroupResource and get Scale
// of the workload declared in spec.scaleTargetRef of HPA.
func (r *HPAReplicasSyncer) getGroupResourceAndScaleForWorkloadFromHPA(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler,
) (schema.GroupResource, *autoscalingv1.Scale, error) {
	gvk := schema.FromAPIVersionAndKind(hpa.Spec.ScaleTargetRef.APIVersion, hpa.Spec.ScaleTargetRef.Kind)
	mapping, err := r.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.Errorf("Failed to get group resource for resource(kind=%s, %s/%s): %v",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, err)

		return schema.GroupResource{}, nil, err
	}

	gr := mapping.Resource.GroupResource()

	scale, err := r.ScaleClient.Scales(hpa.Namespace).Get(ctx, gr, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the scale of workload is not found, skip processing.
			klog.V(4).Infof("Scale of resource(kind=%s, %s/%s) not found, the resource might have been removed",
				hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)

			return gr, nil, nil
		}

		klog.Errorf("Failed to get scale for resource(kind=%s, %s/%s): %v",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, err)

		return schema.GroupResource{}, nil, err
	}

	return gr, scale, nil
}

// updateScale would update the scale of workload on fed-control plane
// with the actual replicas in member clusters effected by HPA.
func (r *HPAReplicasSyncer) updateScale(ctx context.Context, workloadGR schema.GroupResource, scale *autoscalingv1.Scale, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	scaleCopy := scale.DeepCopy()

	scaleCopy.Spec.Replicas = hpa.Status.DesiredReplicas
	_, err := r.ScaleClient.Scales(hpa.Namespace).Update(ctx, workloadGR, scaleCopy, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to try to sync scale for resource(kind=%s, %s/%s) from %d to %d: %v",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, scale.Spec.Replicas, hpa.Status.CurrentReplicas, err)

		return err
	}

	klog.V(4).Infof("Successfully synced scale for resource(kind=%s, %s/%s) from %d to %d",
		hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, scale.Spec.Replicas, hpa.Status.CurrentReplicas)

	return nil
}

// removeFinalizerIfNeed tries to remove HPAReplicasSyncerFinalizer from HPA if the finalizer exists.
func (r *HPAReplicasSyncer) removeFinalizerIfNeed(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	hpaCopy := hpa.DeepCopy()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updated := controllerutil.RemoveFinalizer(hpaCopy, util.HPAReplicasSyncerFinalizer)
		if !updated {
			// If the hpa does not have the finalizer, return immediately without error.
			return nil
		}

		updateErr := r.Client.Update(ctx, hpaCopy)
		if updateErr == nil {
			klog.V(4).Infof("Remove HPAReplicasSyncerFinalizer from horizontalpodautoscaler %s/%s successfully", hpa.Namespace, hpa.Name)

			return nil
		}

		err := r.Client.Get(ctx, types.NamespacedName{Namespace: hpa.Namespace, Name: hpa.Name}, hpaCopy)
		if err != nil {
			return err
		}

		return updateErr
	})
	if err != nil {
		klog.Errorf("Failed to remove HPAReplicasSyncerFinalizer from horizontalpodautoscaler %s/%s: %v", hpa.Namespace, hpa.Name, err)
	}

	return err
}

// addFinalizerIfNeed tries to add HPAReplicasSyncerFinalizer to HPA if the finalizer does not exist.
func (r *HPAReplicasSyncer) addFinalizerIfNeed(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	hpaCopy := hpa.DeepCopy()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updated := controllerutil.AddFinalizer(hpaCopy, util.HPAReplicasSyncerFinalizer)
		if !updated {
			// If the hpa has had the finalizer, return immediately without error.
			return nil
		}

		updateErr := r.Client.Update(ctx, hpaCopy)
		if updateErr == nil {
			klog.V(4).Infof("Add HPAReplicasSyncerFinalizer to horizontalpodautoscaler %s/%s successfully", hpa.Namespace, hpa.Name)

			return nil
		}

		err := r.Client.Get(ctx, types.NamespacedName{Namespace: hpa.Namespace, Name: hpa.Name}, hpaCopy)
		if err != nil {
			return err
		}

		return updateErr
	})
	if err != nil {
		klog.Errorf("Failed to add HPAReplicasSyncerFinalizer to horizontalpodautoscaler %s/%s: %v", hpa.Namespace, hpa.Name, err)
	}

	return err
}

// isReplicasSynced judges if the replicas of workload scale has been synced successfully.
func isReplicasSynced(scale *autoscalingv1.Scale, hpa *autoscalingv2.HorizontalPodAutoscaler) bool {
	if scale == nil || hpa == nil {
		// If the scale not found, the workload might has been removed;
		// if the hpa not found, the workload is not managed by hpa;
		// we treat these as replicas synced.
		return true
	}

	return scale.Spec.Replicas == hpa.Status.DesiredReplicas
}
