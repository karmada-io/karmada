package federatedresourcequota

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// StatusControllerName is the controller name that will be used when reporting events.
	StatusControllerName = "federated-resource-quota-status-controller"
)

// StatusController is to collect status from work to FederatedResourceQuota.
type StatusController struct {
	client.Client // used to operate Work resources.
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The SyncController will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *StatusController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("FederatedResourceQuota status controller reconciling %s", req.NamespacedName.String())

	quota := &policyv1alpha1.FederatedResourceQuota{}
	if err := c.Get(ctx, req.NamespacedName, quota); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if !quota.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	if err := c.collectQuotaStatus(quota); err != nil {
		klog.Errorf("Failed to collect status from works to federatedResourceQuota(%s), error: %v", req.NamespacedName.String(), err)
		c.EventRecorder.Eventf(quota, corev1.EventTypeWarning, events.EventReasonCollectFederatedResourceQuotaStatusFailed, err.Error())
		return controllerruntime.Result{Requeue: true}, err
	}
	c.EventRecorder.Eventf(quota, corev1.EventTypeNormal, events.EventReasonCollectFederatedResourceQuotaStatusSucceed, "Collect status of FederatedResourceQuota(%s) succeed.", req.NamespacedName.String())
	return controllerruntime.Result{}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *StatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	fn := handler.MapFunc(
		func(obj client.Object) []reconcile.Request {
			var requests []reconcile.Request

			quotaNamespace, namespaceExist := obj.GetLabels()[util.FederatedResourceQuotaNamespaceLabel]
			quotaName, nameExist := obj.GetLabels()[util.FederatedResourceQuotaNameLabel]
			if namespaceExist && nameExist {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: quotaNamespace,
						Name:      quotaName,
					},
				})
			}

			return requests
		},
	)

	workPredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			objOld := e.ObjectOld.(*workv1alpha1.Work)
			objNew := e.ObjectNew.(*workv1alpha1.Work)
			_, namespaceExist := objNew.GetLabels()[util.FederatedResourceQuotaNamespaceLabel]
			_, nameExist := objNew.GetLabels()[util.FederatedResourceQuotaNameLabel]
			if !namespaceExist || !nameExist {
				return false
			}
			return !reflect.DeepEqual(objOld.Status, objNew.Status)
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
		Watches(&source.Kind{Type: &workv1alpha1.Work{}}, handler.EnqueueRequestsFromMapFunc(fn), workPredicate).
		Complete(c)
}

func (c *StatusController) collectQuotaStatus(quota *policyv1alpha1.FederatedResourceQuota) error {
	workList, err := helper.GetWorksByLabelsSet(c.Client, labels.Set{
		util.FederatedResourceQuotaNamespaceLabel: quota.Namespace,
		util.FederatedResourceQuotaNameLabel:      quota.Name,
	})
	if err != nil {
		klog.Errorf("Failed to list workList created by federatedResourceQuota(%s), error: %v", klog.KObj(quota).String(), err)
		return err
	}

	aggregatedStatuses, err := aggregatedStatusFormWorks(workList.Items)
	if err != nil {
		return err
	}

	quotaStatus := quota.Status.DeepCopy()
	quotaStatus.Overall = quota.Spec.Overall
	quotaStatus.AggregatedStatus = aggregatedStatuses
	quotaStatus.OverallUsed = calculateUsed(aggregatedStatuses)

	if reflect.DeepEqual(quota.Status, *quotaStatus) {
		klog.V(4).Infof("New quotaStatus are equal with old federatedResourceQuota(%s) status, no update required.", klog.KObj(quota).String())
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		quota.Status = *quotaStatus
		updateErr := c.Status().Update(context.TODO(), quota)
		if updateErr == nil {
			return nil
		}

		updated := &policyv1alpha1.FederatedResourceQuota{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: quota.Namespace, Name: quota.Name}, updated); err == nil {
			quota = updated
		} else {
			klog.Errorf("Failed to get updated  federatedResourceQuota(%s): %v", klog.KObj(quota).String(), err)
		}

		return updateErr
	})
}

func aggregatedStatusFormWorks(works []workv1alpha1.Work) ([]policyv1alpha1.ClusterQuotaStatus, error) {
	var aggregatedStatuses []policyv1alpha1.ClusterQuotaStatus
	for index := range works {
		work := works[index]
		var applied bool
		if cond := meta.FindStatusCondition(work.Status.Conditions, workv1alpha1.WorkApplied); cond != nil {
			switch cond.Status {
			case metav1.ConditionTrue:
				applied = true
			case metav1.ConditionUnknown:
				fallthrough
			case metav1.ConditionFalse:
				applied = false
			default: // should not happen unless the condition api changed.
				panic("unexpected status")
			}
		}
		if !applied {
			klog.Warningf("Work(%s) applied failed, skip aggregated status", klog.KObj(&work).String())
			continue
		}

		if len(work.Status.ManifestStatuses) == 0 {
			klog.Warningf("The ManifestStatuses length of work(%s) is zero", klog.KObj(&work).String())
			continue
		}

		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.Errorf("Failed to get clusterName from work namespace %s. Error: %v.", work.Namespace, err)
			return nil, err
		}

		status := &corev1.ResourceQuotaStatus{}
		if err := json.Unmarshal(work.Status.ManifestStatuses[0].Status.Raw, status); err != nil {
			klog.Errorf("Failed to unmarshal work(%s) status to ResourceQuotaStatus", klog.KObj(&work).String())
			return nil, err
		}

		aggregatedStatus := policyv1alpha1.ClusterQuotaStatus{
			ClusterName:         clusterName,
			ResourceQuotaStatus: *status,
		}
		aggregatedStatuses = append(aggregatedStatuses, aggregatedStatus)
	}

	sort.Slice(aggregatedStatuses, func(i, j int) bool {
		return aggregatedStatuses[i].ClusterName < aggregatedStatuses[j].ClusterName
	})
	return aggregatedStatuses, nil
}

func calculateUsed(aggregatedStatuses []policyv1alpha1.ClusterQuotaStatus) corev1.ResourceList {
	overallUsed := corev1.ResourceList{}
	for index := range aggregatedStatuses {
		used := aggregatedStatuses[index].Used
		for resourceName, quantity := range used {
			r, exist := overallUsed[resourceName]
			if !exist {
				overallUsed[resourceName] = quantity
			} else {
				r.Add(quantity)
				overallUsed[resourceName] = r
			}
		}
	}
	return overallUsed
}
