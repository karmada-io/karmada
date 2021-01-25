package hpa

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "hpa-controller"

// HorizontalPodAutoscalerController is to sync HorizontalPodAutoscaler.
type HorizontalPodAutoscalerController struct {
	client.Client // used to operate HorizontalPodAutoscaler resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *HorizontalPodAutoscalerController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling HorizontalPodAutoscaler %s.", req.NamespacedName.String())

	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, hpa); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !hpa.DeletionTimestamp.IsZero() {
		// Do nothing, just return.
		return controllerruntime.Result{}, nil
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *HorizontalPodAutoscalerController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&autoscalingv1.HorizontalPodAutoscaler{}).Complete(c)
}
