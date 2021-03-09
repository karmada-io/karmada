package propagationpolicy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// Controller is to sync PropagationPolicy.
type Controller struct {
	client.Client
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationPolicy %s.", req.NamespacedName.String())

	policy := &v1alpha1.PropagationPolicy{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, policy); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	// TODO(RainbowMango): wait for moving this logic to detector.
	// Maybe there is another option that introduce status for policy apis.
	present, err := c.allDependentOverridesPresent(policy)
	if err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}
	if !present {
		klog.Infof("waiting for policy(%s) dependent overrides ready.", req.String())
		return controllerruntime.Result{Requeue: true}, fmt.Errorf("the specific overrides which current PropagationPolicy %v/%v rely on are not all present", policy.GetNamespace(), policy.GetName())
	}

	return controllerruntime.Result{}, nil
}

// allDependentOverridesPresent will ensure the specify overrides which current PropagationPolicy rely on are present
func (c *Controller) allDependentOverridesPresent(policy *v1alpha1.PropagationPolicy) (bool, error) {
	for _, override := range policy.Spec.DependentOverrides {
		overrideObj := &v1alpha1.OverridePolicy{}
		if err := c.Client.Get(context.TODO(), client.ObjectKey{Namespace: policy.Namespace, Name: override}, overrideObj); err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("The specific override policy %v/%v which current PropagationPolicy %v/%v rely on is not present", policy.Namespace, override, policy.Namespace, policy.Name)
				return false, nil
			}
			klog.Errorf("Failed to get override policy %v/%v, Error: %v", policy.Namespace, override, err)
			return false, err
		}
	}

	return true, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *Controller) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationPolicy{}).Complete(c)
}
