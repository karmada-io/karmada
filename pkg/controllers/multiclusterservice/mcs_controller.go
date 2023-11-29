/*
Copyright 2023 The Karmada Authors.

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

package multiclusterservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "multiclusterservice-controller"

// MCSController is to sync MultiClusterService.
type MCSController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *MCSController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling MultiClusterService", "namespace", req.Namespace, "name", req.Name)

	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, req.NamespacedName, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			// The mcs no longer exist, in which case we stop processing.
			return controllerruntime.Result{}, nil
		}
		klog.ErrorS(err, "failed to get MultiClusterService object", "NamespacedName", req.NamespacedName)
		return controllerruntime.Result{}, err
	}

	if !mcs.DeletionTimestamp.IsZero() {
		return c.handleMCSDelete(ctx, mcs.DeepCopy())
	}

	var err error
	defer func() {
		if err != nil {
			_ = c.updateMCSStatus(mcs, metav1.ConditionFalse, "ServiceAppliedFailed", err.Error())
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceWorkFailed, err.Error())
			return
		}
		_ = c.updateMCSStatus(mcs, metav1.ConditionTrue, "ServiceAppliedSucceed", "Service is propagated to target clusters.")
	}()

	if err = c.handleMCSCreateOrUpdate(ctx, mcs.DeepCopy()); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (c *MCSController) handleMCSDelete(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Begin to handle MultiClusterService delete event", "namespace", mcs.Namespace, "name", mcs.Name)

	if err := c.deleteServiceWork(mcs, sets.New[string]()); err != nil {
		c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceWorkFailed,
			fmt.Sprintf("failed to delete service work :%v", err))
		return controllerruntime.Result{}, err
	}

	finalizersUpdated := controllerutil.RemoveFinalizer(mcs, util.MCSControllerFinalizer)
	if finalizersUpdated {
		err := c.Client.Update(ctx, mcs)
		if err != nil {
			klog.V(4).ErrorS(err, "failed to update MultiClusterService with finalizer",
				"namespace", mcs.Namespace, "name", mcs.Name)
			return controllerruntime.Result{}, err
		}
	}

	klog.V(4).InfoS("Success to delete MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
	return controllerruntime.Result{}, nil
}

func (c *MCSController) deleteServiceWork(mcs *networkingv1alpha1.MultiClusterService, retainClusters sets.Set[string]) error {
	mcsID, err := c.getMultiClusterServiceID(mcs)
	if err != nil {
		klog.Errorf("Get MultiClusterService(%s/%s) ID error:%v", mcs.Namespace, mcs.Name, err)
		return err
	}

	workList, err := helper.GetWorksByLabelsSet(c, labels.Set{networkingv1alpha1.MultiClusterServicePermanentIDLabel: mcsID})
	if err != nil {
		klog.ErrorS(err, "failed to get work", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	for index := range workList.Items {
		clusterName := strings.TrimPrefix(workList.Items[index].Namespace, names.ExecutionSpacePrefix)
		if retainClusters.Has(clusterName) {
			continue
		}

		if err = c.Client.Delete(context.TODO(), &workList.Items[index]); err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Error while updating work(%s/%s) deletion timestamp: %s",
				workList.Items[index].Namespace, workList.Items[index].Name, err)
			return err
		}
	}

	klog.V(4).InfoS("success to delete service work", "namespace", mcs.Namespace, "name", mcs.Name)
	return nil
}

func (c *MCSController) handleMCSCreateOrUpdate(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	klog.V(4).InfoS("Begin to handle MultiClusterService create or update event",
		"namespace", mcs.Namespace, "name", mcs.Name)

	// 1. if mcs not contain CrossCluster type, delete service work if needed
	if !helper.MultiClusterServiceCrossClusterEnabled(mcs) {
		return c.deleteServiceWork(mcs, sets.New[string]())
	}

	// 2. add finalizer if needed
	finalizersUpdated := controllerutil.AddFinalizer(mcs, util.MCSControllerFinalizer)
	if finalizersUpdated {
		err := c.Client.Update(ctx, mcs)
		if err != nil {
			klog.V(4).ErrorS(err, "failed to update mcs with finalizer", "namespace", mcs.Namespace, "name", mcs.Name)
			return err
		}
	}

	// 3. make sure service exist
	svc := &corev1.Service{}
	err := c.Client.Get(ctx, types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}, svc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "failed to get service", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	// 4. if service not exist, delete service work if needed
	if apierrors.IsNotFound(err) {
		delErr := c.deleteServiceWork(mcs, sets.New[string]())
		if delErr != nil {
			klog.ErrorS(err, "failed to delete service work", "namespace", mcs.Namespace, "name", mcs.Name)
			c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceWorkFailed,
				fmt.Sprintf("failed to delete service work :%v", err))
		}
		return err
	}

	// 5. if service exist, create or update corresponding work in clusters
	syncClusters, err := c.syncSVCWorkToClusters(ctx, mcs, svc)
	if err != nil {
		return err
	}

	// 6. delete service work not in need sync clusters
	if err = c.deleteServiceWork(mcs, syncClusters); err != nil {
		return err
	}

	klog.V(4).InfoS("success to ensure service work", "namespace", mcs.Namespace, "name", mcs.Name)
	return nil
}

func (c *MCSController) syncSVCWorkToClusters(
	ctx context.Context,
	mcs *networkingv1alpha1.MultiClusterService,
	svc *corev1.Service,
) (sets.Set[string], error) {
	syncClusters := sets.New[string]()
	clusters := &clusterv1alpha1.ClusterList{}
	err := c.Client.List(ctx, clusters)
	if err != nil {
		klog.ErrorS(err, "failed to list clusters")
		return syncClusters, err
	}

	serverLocations := sets.New[string](mcs.Spec.ServiceProvisionClusters...)
	clientLocations := sets.New[string](mcs.Spec.ServiceConsumptionClusters...)
	for _, cluster := range clusters.Items {
		// if ServerLocations or ClientLocations are empty, we will sync work to the all clusters
		if len(serverLocations) == 0 || len(clientLocations) == 0 ||
			serverLocations.Has(cluster.Name) || clientLocations.Has(cluster.Name) {
			syncClusters.Insert(cluster.Name)
		}
	}

	svcObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(svc)
	if err != nil {
		return syncClusters, err
	}

	mcsID, err := c.getMultiClusterServiceID(mcs)
	if err != nil {
		klog.Errorf("Get MultiClusterService(%s/%s) ID error:%v", mcs.Namespace, mcs.Name, err)
		return syncClusters, err
	}

	var errs []error
	for clusterName := range syncClusters {
		workMeta := metav1.ObjectMeta{
			Name:       names.GenerateWorkName(svc.Kind, svc.Name, clusterName+"/"+svc.Namespace),
			Namespace:  names.GenerateExecutionSpaceName(clusterName),
			Finalizers: []string{util.ExecutionControllerFinalizer},
			Labels: map[string]string{
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: mcsID,
				util.ManagedByKarmadaLabel:                             util.ManagedByKarmadaLabelValue,
			},
		}

		if err = helper.CreateOrUpdateWork(c, workMeta, &unstructured.Unstructured{Object: svcObj}); err != nil {
			klog.Errorf("Failed to create or update resource(%v/%v) in the given member cluster %s, err is %v",
				workMeta.GetNamespace(), workMeta.GetName(), clusterName, err)
			c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceWorkFailed, fmt.Sprintf(
				"Failed to create or update resource(%v/%v) in member cluster(%s): %v",
				workMeta.GetNamespace(), workMeta.GetName(), clusterName, err))
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return syncClusters, errors.NewAggregate(errs)
	}

	return syncClusters, nil
}

func (c *MCSController) updateMCSStatus(mcs *networkingv1alpha1.MultiClusterService, status metav1.ConditionStatus, reason, message string) error {
	serviceAppliedCondition := metav1.Condition{
		Type:               networkingv1alpha1.MCSServiceAppliedConditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		meta.SetStatusCondition(&mcs.Status.Conditions, serviceAppliedCondition)
		updateErr := c.Status().Update(context.TODO(), mcs)
		if updateErr == nil {
			return nil
		}
		updated := &networkingv1alpha1.MultiClusterService{}
		if err = c.Get(context.TODO(), client.ObjectKey{Namespace: mcs.Namespace, Name: mcs.Name}, updated); err == nil {
			mcs = updated
		} else {
			klog.Errorf("Failed to get updated MultiClusterService %s/%s: %v", mcs.Namespace, mcs.Name, err)
		}
		return updateErr
	})
}

func (c *MCSController) getMultiClusterServiceID(mcs *networkingv1alpha1.MultiClusterService) (string, error) {
	id := util.GetLabelValue(mcs.GetLabels(), networkingv1alpha1.MultiClusterServicePermanentIDLabel)
	if id == "" {
		id = uuid.New().String()
		mcs.Labels = util.DedupeAndMergeLabels(mcs.Labels, map[string]string{networkingv1alpha1.MultiClusterServicePermanentIDLabel: id})
		if err := c.Client.Update(context.TODO(), mcs); err != nil {
			return id, err
		}
	}

	return id, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *MCSController) SetupWithManager(mgr controllerruntime.Manager) error {
	mcsPredicateFunc := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			mcs := e.Object.(*networkingv1alpha1.MultiClusterService)
			return helper.MultiClusterServiceCrossClusterEnabled(mcs)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			mcsOld := e.ObjectOld.(*networkingv1alpha1.MultiClusterService)
			mcsNew := e.ObjectNew.(*networkingv1alpha1.MultiClusterService)
			if !helper.MultiClusterServiceCrossClusterEnabled(mcsOld) && !helper.MultiClusterServiceCrossClusterEnabled(mcsNew) {
				return false
			}

			// We only care about the update events below:
			if equality.Semantic.DeepEqual(mcsOld.Annotations, mcsNew.Annotations) &&
				equality.Semantic.DeepEqual(mcsOld.Spec, mcsNew.Spec) &&
				equality.Semantic.DeepEqual(mcsOld.DeletionTimestamp.IsZero(), mcsNew.DeletionTimestamp.IsZero()) {
				return false
			}
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			// Since finalizer is added to the MultiClusterService object,
			// the delete event is processed by the update event.
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return true
		},
	}

	svcPredicateFunc := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			svcOld := e.ObjectOld.(*corev1.Service)
			svcNew := e.ObjectNew.(*corev1.Service)

			// We only care about the update events below:
			if equality.Semantic.DeepEqual(svcOld.Annotations, svcNew.Annotations) &&
				equality.Semantic.DeepEqual(svcOld.Spec, svcNew.Spec) {
				return false
			}
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return true
		},
	}

	svcMapFunc := handler.MapFunc(
		func(ctx context.Context, svcObj client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: svcObj.GetNamespace(),
					Name:      svcObj.GetName(),
				},
			}}
		})

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.MultiClusterService{}, builder.WithPredicates(mcsPredicateFunc)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(svcMapFunc), builder.WithPredicates(svcPredicateFunc)).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}
