package federatedresourcequota

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// SyncControllerName is the controller name that will be used when reporting events.
	SyncControllerName = "federated-resource-quota-sync-controller"
)

// SyncController is to sync FederatedResourceQuota.
type SyncController struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *SyncController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("FederatedResourceQuota sync controller reconciling %s", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Client.Get(ctx, req.NamespacedName, quota); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Begin to cleanup works created by federatedResourceQuota(%s)", req.NamespacedName.String())
			if err = c.cleanUpWorks(req.Namespace, req.Name); err != nil {
				klog.Errorf("Failed to cleanup works created by federatedResourceQuota(%s)", req.NamespacedName.String())
				return controllerruntime.Result{Requeue: true}, err
			}
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusterList); err != nil {
		klog.Errorf("Failed to list clusters, error: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	}

	if err := c.buildWorks(quota, clusterList.Items); err != nil {
		klog.Errorf("Failed to build works for federatedResourceQuota(%s), error: %v", req.NamespacedName.String(), err)
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonSyncFederatedResourceQuotaFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}
	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonSyncFederatedResourceQuotaSucceed, "Sync works for FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())

	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *SyncController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(context.Context, client.Object) []reconcile.Request {
			var requests []reconcile.Request

			FederatedResourceQuotaList := &policyv1alpha1.FederatedResourceQuotaList{}
			if err := c.Client.List(context.TODO(), FederatedResourceQuotaList); err != nil {
				klog.Errorf("Failed to list FederatedResourceQuota, error: %v", err)
			}

			for _, federatedResourceQuota := range FederatedResourceQuotaList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: federatedResourceQuota.Namespace,
						Name:      federatedResourceQuota.Name,
					},
				})
			}

			return requests
		},
	)

	clusterPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	})

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&policyv1alpha1.FederatedResourceQuota{}).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(fn), clusterPredicate).
		Complete(c)
}

func (c *SyncController) cleanUpWorks(namespace, name string) error {
	var errs []error
	workList := &workv1alpha1.WorkList{}
	if err := c.List(context.TODO(), workList, client.MatchingLabels{
		util.FederatedResourceQuotaNamespaceLabel: namespace,
		util.FederatedResourceQuotaNameLabel:      name,
	}); err != nil {
		klog.Errorf("Failed to list works, err: %v", err)
		return err
	}

	for index := range workList.Items {
		work := &workList.Items[index]
		if err := c.Delete(context.TODO(), work); err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to delete work(%s): %v", klog.KObj(work).String(), err)
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func (c *SyncController) buildWorks(quota *policyv1alpha1.FederatedResourceQuota, clusters []clusterv1alpha1.Cluster) error {
	var errs []error
	for _, cluster := range clusters {
		workNamespace := names.GenerateExecutionSpaceName(cluster.Name)
		workName := names.GenerateWorkName("ResourceQuota", quota.Name, quota.Namespace)

		resourceQuota := &corev1.ResourceQuota{}
		resourceQuota.APIVersion = "v1"
		resourceQuota.Kind = "ResourceQuota"
		resourceQuota.Namespace = quota.Namespace
		resourceQuota.Name = quota.Name
		resourceQuota.Labels = map[string]string{
			workv1alpha1.WorkNamespaceLabel: workNamespace,
			workv1alpha1.WorkNameLabel:      workName,
			util.ManagedByKarmadaLabel:      util.ManagedByKarmadaLabelValue,
		}
		resourceQuota.Spec.Hard = extractClusterHardResourceList(quota.Spec, cluster.Name)

		resourceQuotaObj, err := helper.ToUnstructured(resourceQuota)
		if err != nil {
			klog.Errorf("Failed to transform resourceQuota(%s), error: %v", klog.KObj(resourceQuota).String(), err)
			errs = append(errs, err)
			continue
		}

		objectMeta := metav1.ObjectMeta{
			Namespace:  workNamespace,
			Name:       workName,
			Finalizers: []string{util.ExecutionControllerFinalizer},
			Labels: map[string]string{
				util.FederatedResourceQuotaNamespaceLabel: quota.Namespace,
				util.FederatedResourceQuotaNameLabel:      quota.Name,
				util.ManagedByKarmadaLabel:                util.ManagedByKarmadaLabelValue,
			},
		}

		err = helper.CreateOrUpdateWork(c.Client, objectMeta, resourceQuotaObj)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.NewAggregate(errs)
}

func extractClusterHardResourceList(spec policyv1alpha1.FederatedResourceQuotaSpec, cluster string) corev1.ResourceList {
	for index := range spec.StaticAssignments {
		if spec.StaticAssignments[index].ClusterName == cluster {
			return spec.StaticAssignments[index].Hard
		}
	}
	return nil
}
