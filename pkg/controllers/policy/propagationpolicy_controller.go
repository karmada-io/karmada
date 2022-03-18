package policy

import (
	"context"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/policy/resource"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/ratelimiter"
)

const (
	// PropagationPolicyControllerName is the controller name that will be used when reporting events.
	PropagationPolicyControllerName = "propagation-policy-controller"
)

// PropagationPolicyController is to sync PropagationPolicy.
type PropagationPolicyController struct {
	lock                            sync.RWMutex
	client.Client                                                                // used to operate ClusterResourceBinding resources.
	DynamicClient                   dynamic.Interface                            // used to fetch arbitrary resources from api server.
	InformerManager                 informermanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder                   record.EventRecorder
	RESTMapper                      meta.RESTMapper
	OverrideManager                 overridemanager.OverrideManager
	ResourceInterpreter             resourceinterpreter.ResourceInterpreter
	RatelimiterOptions              ratelimiter.Options
	ConcurrentResourceTemplateSyncs int
	resourceController              map[schema.GroupVersionKind]*resource.Controller
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *PropagationPolicyController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling PropagationPolicy %s.", req.NamespacedName.String())

	policy := &policyv1alpha1.PropagationPolicy{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, policy); err != nil {
		// The resource no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		err := c.handlePropagationPolicyDeletion(policy.Namespace, policy.Name)
		if err != nil {
			return controllerruntime.Result{}, err
		}
	}
	return c.syncPolicy(policy)
}

// syncPolicy will sync propagationpolicy.
func (c *PropagationPolicyController) syncPolicy(pp *policyv1alpha1.PropagationPolicy) (controllerruntime.Result, error) {
	for _, rs := range pp.Spec.ResourceSelectors {
		// TODO match policy first, if match do apply policy, if not match then start resource controller
		gvk, err := resourceSelectorToGVK(rs)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if !c.resourceControllerHasStarted(gvk) {
			restMapping, _ := c.RESTMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			resourceController, err := resource.NewController(&metav1.APIResource{
				Group:   gvk.Group,
				Kind:    gvk.Kind,
				Version: gvk.Version,
				Name:    restMapping.Resource.Resource,
			}, c.Client, c.DynamicClient, c.InformerManager, c.ResourceInterpreter, c.EventRecorder)
			if err != nil {
				return reconcile.Result{}, err
			}
			// TODO(pigletfly): stop resource controller when it's not needed, all PropagationPolicy and ClusterPropagationPolicy resourceSelector don't equal with the resource
			stopChan := make(chan struct{})
			resourceController.Run(c.ConcurrentResourceTemplateSyncs, stopChan)
			c.addResourceController(gvk, resourceController)
		}
	}
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationPolicyController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&policyv1alpha1.PropagationPolicy{}).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.DefaultControllerRateLimiter(c.RatelimiterOptions),
		}).
		Complete(c)
}

func (c *PropagationPolicyController) resourceControllerHasStarted(gvk schema.GroupVersionKind) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, ok := c.resourceController[gvk]
	return ok
}

func (c *PropagationPolicyController) addResourceController(gvk schema.GroupVersionKind, controller *resource.Controller) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, ok := c.resourceController[gvk]; !ok {
		c.resourceController[gvk] = controller
	}
}

// HandlePropagationPolicyDeletion handles PropagationPolicy delete event.
// After a policy is removed, the label marked on relevant resource template will be removed(which gives
// the resource template a change to match another policy).
//
// Note: The relevant ResourceBinding will continue to exist until the resource template is gone.
func (c *PropagationPolicyController) handlePropagationPolicyDeletion(policyNS string, policyName string) error {
	labelSet := labels.Set{
		policyv1alpha1.PropagationPolicyNamespaceLabel: policyNS,
		policyv1alpha1.PropagationPolicyNameLabel:      policyName,
	}

	rbs, err := helper.GetResourceBindings(c.Client, labelSet)
	if err != nil {
		klog.Errorf("Failed to list propagation bindings: %v", err)
		return err
	}

	for _, binding := range rbs.Items {
		// Cleanup the labels from the object referencing by binding.
		// In addition, this will give the object a chance to match another policy.
		if err := cleanupLabels(c.DynamicClient, c.InformerManager, c.RESTMapper, binding.Spec.Resource, policyv1alpha1.PropagationPolicyNamespaceLabel, policyv1alpha1.PropagationPolicyNameLabel); err != nil {
			klog.Errorf("Failed to cleanup label from resource(%s-%s/%s) when resource binding(%s/%s) removing, error: %v",
				binding.Spec.Resource.Kind, binding.Spec.Resource.Namespace, binding.Spec.Resource.Name, binding.Namespace, binding.Name, err)
			return err
		}
	}
	return nil
}
