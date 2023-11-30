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

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointsliceDispatchControllerName is the controller name that will be used when reporting events.
const EndpointsliceDispatchControllerName = "endpointslice-dispatch-controller"

var (
	endpointSliceGVK = discoveryv1.SchemeGroupVersion.WithKind("EndpointSlice")
)

type EndpointsliceDispatchController struct {
	client.Client
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	InformerManager genericmanager.MultiClusterInformerManager
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *EndpointsliceDispatchController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling Work %s", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	if !helper.IsWorkContains(work.Spec.Workload.Manifests, endpointSliceGVK) {
		return controllerruntime.Result{}, nil
	}

	if !work.DeletionTimestamp.IsZero() {
		if err := c.cleanupEndpointSliceFromConsumerClusters(ctx, work); err != nil {
			klog.Errorf("Failed to cleanup EndpointSlice from consumer clusters for work %s/%s:%v", work.Namespace, work.Name, err)
			return controllerruntime.Result{Requeue: true}, err
		}
		return controllerruntime.Result{}, nil
	}

	mcsName := util.GetLabelValue(work.Labels, util.MultiClusterServiceNameLabel)
	mcsNS := util.GetLabelValue(work.Labels, util.MultiClusterServiceNamespaceLabel)
	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, types.NamespacedName{Namespace: mcsNS, Name: mcsName}, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("MultiClusterService %s/%s is not found", mcsNS, mcsName)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{Requeue: true}, err
	}

	var err error
	defer func() {
		if err != nil {
			_ = c.updateEndpointSliceSynced(mcs, metav1.ConditionFalse, "EndpointSliceSyncFailed", err.Error())
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonDispatchEndpointSliceFailed, err.Error())
			return
		}
		_ = c.updateEndpointSliceSynced(mcs, metav1.ConditionTrue, "EndpointSliceSyncSucceed", "EndpointSlice are synced successfully")
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventReasonDispatchEndpointSliceSucceed, "EndpointSlice are synced successfully")
	}()

	if err = c.cleanOrphanDispatchedEndpointSlice(ctx, mcs); err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	if err = c.syncEndpointSlice(ctx, work.DeepCopy(), mcs); err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *EndpointsliceDispatchController) updateEndpointSliceSynced(mcs *networkingv1alpha1.MultiClusterService, status metav1.ConditionStatus, reason, message string) error {
	EndpointSliceCollected := metav1.Condition{
		Type:               networkingv1alpha1.EndpointSliceDispatched,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		meta.SetStatusCondition(&mcs.Status.Conditions, EndpointSliceCollected)
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

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointsliceDispatchController) SetupWithManager(mgr controllerruntime.Manager) error {
	workPredicateFun := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			// We only care about the EndpointSlice work from provision clusters
			return util.GetLabelValue(createEvent.Object.GetLabels(), util.MultiClusterServiceNameLabel) != "" &&
				util.GetAnnotationValue(createEvent.Object.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// We only care about the EndpointSlice work from provision clusters
			return util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.MultiClusterServiceNameLabel) != "" &&
				util.GetAnnotationValue(updateEvent.ObjectNew.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			// We only care about the EndpointSlice work from provision clusters
			return util.GetLabelValue(deleteEvent.Object.GetLabels(), util.MultiClusterServiceNameLabel) != "" &&
				util.GetAnnotationValue(deleteEvent.Object.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).For(&workv1alpha1.Work{}, builder.WithPredicates(workPredicateFun)).
		Watches(&networkingv1alpha1.MultiClusterService{}, handler.EnqueueRequestsFromMapFunc(c.newMultiClusterServiceFunc())).
		Complete(c)
}

func (c *EndpointsliceDispatchController) newMultiClusterServiceFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		var mcsName, mcsNamespace string
		switch t := a.(type) {
		case *networkingv1alpha1.MultiClusterService:
			mcsNamespace = t.Namespace
			mcsName = t.Name
		default:
			return nil
		}

		workList := &workv1alpha1.WorkList{}
		if err := c.Client.List(context.TODO(), workList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				util.MultiClusterServiceNameLabel:      mcsName,
				util.MultiClusterServiceNamespaceLabel: mcsNamespace,
			}),
		}); err != nil {
			klog.Errorf("Failed to list work, error: %v", err)
			return nil
		}

		var requests []reconcile.Request
		for _, work := range workList.Items {
			// We only care about the EndpointSlice work from provision clusters
			if util.GetLabelValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) != "" {
				continue
			}

			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: work.Namespace, Name: work.Name}})
		}
		return requests
	}
}

func (c *EndpointsliceDispatchController) cleanOrphanDispatchedEndpointSlice(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	workList := &workv1alpha1.WorkList{}
	if err := c.Client.List(ctx, workList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			util.MultiClusterServiceNameLabel:      mcs.Name,
			util.MultiClusterServiceNamespaceLabel: mcs.Namespace,
		})}); err != nil {
		klog.Errorf("Failed to list works, error is: %v", err)
		return err
	}

	for _, work := range workList.Items {
		// We only care about the EndpointSlice work in consumption clusters
		if util.GetAnnotationValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) == "" {
			continue
		}

		consumptionClusters, err := helper.GetConsumptionClustres(c.Client, mcs)
		if err != nil {
			klog.Errorf("Failed to get consumption clusters, error is: %v", err)
			return err
		}

		cluster, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.Errorf("Failed to get cluster name for work %s/%s", work.Namespace, work.Name)
			return err
		}

		if consumptionClusters.Has(cluster) {
			continue
		}

		if err := c.Client.Delete(ctx, work.DeepCopy()); err != nil {
			klog.Errorf("Failed to delete work %s/%s, error is: %v", work.Namespace, work.Name, err)
			return err
		}
	}

	return nil
}

func (c *EndpointsliceDispatchController) syncEndpointSlice(ctx context.Context, work *workv1alpha1.Work, mcs *networkingv1alpha1.MultiClusterService) error {
	epsSourceCluster, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get EndpointSlice source cluster name for work %s/%s", work.Namespace, work.Name)
		return err
	}

	consumptionClusters := sets.New[string](mcs.Spec.ServiceConsumptionClusters...)
	if len(consumptionClusters) == 0 {
		consumptionClusters, err = util.GetClusterSet(c.Client)
		if err != nil {
			klog.Errorf("Failed to get cluster set, error is: %v", err)
			return err
		}
	}
	for clusterName := range consumptionClusters {
		if clusterName == epsSourceCluster {
			continue
		}

		// It couldn't happen here
		if len(work.Spec.Workload.Manifests) == 0 {
			continue
		}

		// There should be only one manifest in the work, let's use the first one.
		manifest := work.Spec.Workload.Manifests[0]
		unstructuredObj := &unstructured.Unstructured{}
		if err := unstructuredObj.UnmarshalJSON(manifest.Raw); err != nil {
			klog.Errorf("Failed to unmarshal work manifest, error is: %v", err)
			return err
		}

		endpointSlice := &discoveryv1.EndpointSlice{}
		if err := helper.ConvertToTypedObject(unstructuredObj, endpointSlice); err != nil {
			klog.Errorf("Failed to convert unstructured object to typed object, error is: %v", err)
			return err
		}

		// Use this name to avoid naming conflicts and locate the EPS source cluster.
		endpointSlice.Name = epsSourceCluster + "-" + endpointSlice.Name
		clusterNamespace := names.GenerateExecutionSpaceName(clusterName)
		endpointSlice.Labels = map[string]string{
			discoveryv1.LabelServiceName:    mcs.Name,
			workv1alpha1.WorkNamespaceLabel: clusterNamespace,
			workv1alpha1.WorkNameLabel:      work.Name,
			util.ManagedByKarmadaLabel:      util.ManagedByKarmadaLabelValue,
			discoveryv1.LabelManagedBy:      util.EndpointSliceDispatchControllerLabelValue,
		}
		endpointSlice.Annotations = map[string]string{
			// This annotation is used to identify the source cluster of EndpointSlice and whether the eps are the newest version
			util.EndpointSliceProvisionClusterAnnotation: epsSourceCluster,
		}

		workMeta := metav1.ObjectMeta{
			Name:       work.Name,
			Namespace:  clusterNamespace,
			Finalizers: []string{util.ExecutionControllerFinalizer},
			Annotations: map[string]string{
				util.EndpointSliceProvisionClusterAnnotation: epsSourceCluster,
			},
			Labels: map[string]string{
				util.ManagedByKarmadaLabel:             util.ManagedByKarmadaLabelValue,
				util.MultiClusterServiceNameLabel:      mcs.Name,
				util.MultiClusterServiceNamespaceLabel: mcs.Namespace,
			},
		}
		unstructuredEPS, err := helper.ToUnstructured(endpointSlice)
		if err != nil {
			klog.Errorf("Failed to convert typed object to unstructured object, error is: %v", err)
			return err
		}
		if err := helper.CreateOrUpdateWork(c.Client, workMeta, unstructuredEPS); err != nil {
			klog.Errorf("Failed to sync EndpointSlice %s/%s from %s to cluster %s:%v",
				work.GetNamespace(), work.GetName(), epsSourceCluster, clusterName, err)
			return err
		}
	}

	if controllerutil.AddFinalizer(work, util.MCSEndpointSliceDispatchControllerFinalizer) {
		if err := c.Client.Update(ctx, work); err != nil {
			klog.Errorf("Failed to add finalizer %s for work %s/%s:%v", util.MCSEndpointSliceDispatchControllerFinalizer, work.Namespace, work.Name, err)
			return err
		}
	}

	return nil
}

func (c *EndpointsliceDispatchController) cleanupEndpointSliceFromConsumerClusters(ctx context.Context, work *workv1alpha1.Work) error {
	// TBD: There may be a better way without listing all works.
	workList := &workv1alpha1.WorkList{}
	err := c.Client.List(ctx, workList)
	if err != nil {
		klog.Errorf("Failed to list works serror: %v", err)
		return err
	}

	epsSourceCluster, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.Errorf("Failed to get EndpointSlice provision cluster name for work %s/%s", work.Namespace, work.Name)
		return err
	}
	for _, item := range workList.Items {
		if item.Name != work.Name || util.GetAnnotationValue(item.Annotations, util.EndpointSliceProvisionClusterAnnotation) != epsSourceCluster {
			continue
		}
		if err := c.Client.Delete(ctx, item.DeepCopy()); err != nil {
			return err
		}
	}

	if controllerutil.RemoveFinalizer(work, util.MCSEndpointSliceDispatchControllerFinalizer) {
		if err := c.Client.Update(ctx, work); err != nil {
			klog.Errorf("Failed to remove %s finalizer for work %s/%s:%v", util.MCSEndpointSliceDispatchControllerFinalizer, work.Namespace, work.Name, err)
			return err
		}
	}

	return nil
}
