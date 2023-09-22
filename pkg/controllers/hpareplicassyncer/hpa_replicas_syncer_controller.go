package hpareplicassyncer

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	err = r.updateScaleIfNeed(ctx, workloadGR, scale.DeepCopy(), hpa)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	// TODO(@lxtywypc): Add finalizer for HPA and remove them
	// when the HPA is deleting and the replicas have been synced.

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
			return gr, nil, nil
		}

		klog.Errorf("Failed to get scale for resource(kind=%s, %s/%s): %v",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, err)

		return schema.GroupResource{}, nil, err
	}

	return gr, scale, nil
}

// updateScaleIfNeed would update the scale of workload on fed-control plane
// if the replicas declared in the workload on karmada-control-plane does not match
// the actual replicas in member clusters effected by HPA.
func (r *HPAReplicasSyncer) updateScaleIfNeed(ctx context.Context, workloadGR schema.GroupResource, scale *autoscalingv1.Scale, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	// If the scale of workload is not found, skip processing.
	if scale == nil {
		klog.V(4).Infof("Scale of resource(kind=%s, %s/%s) not found, the resource might have been removed, skip",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)

		return nil
	}

	if scale.Spec.Replicas != hpa.Status.DesiredReplicas {
		oldReplicas := scale.Spec.Replicas

		scale.Spec.Replicas = hpa.Status.DesiredReplicas
		_, err := r.ScaleClient.Scales(hpa.Namespace).Update(ctx, workloadGR, scale, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to try to sync scale for resource(kind=%s, %s/%s) from %d to %d: %v",
				hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, oldReplicas, hpa.Status.DesiredReplicas, err)
			return err
		}

		klog.V(4).Infof("Successfully synced scale for resource(kind=%s, %s/%s) from %d to %d",
			hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, oldReplicas, hpa.Status.DesiredReplicas)
	}

	return nil
}
