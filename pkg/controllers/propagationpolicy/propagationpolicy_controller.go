package propagationpolicy

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// Controller is to sync PropagationPolicy.
type Controller struct {
	client.Client
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationPolicy %s.", req.NamespacedName.String())

	policy := &policyv1alpha1.PropagationPolicy{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, policy); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	// TODO(RainbowMango): wait for moving this logic to detector.
	// Maybe there is another option that introduce status for policy apis.
	present, err := helper.IsDependentOverridesPresent(c.Client, policy)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	if !present {
		klog.Infof("waiting for policy(%s) dependent overrides ready.", req.String())
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("waiting for policy(%s) dependent overrides ready", req.String())
	}

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&policyv1alpha1.PropagationPolicy{}).Complete(c)
}
