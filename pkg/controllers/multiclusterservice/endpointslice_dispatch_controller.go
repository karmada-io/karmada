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
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// EndpointsliceDispatchControllerName is the controller name that will be used when reporting events and metrics.
const EndpointsliceDispatchControllerName = "endpointslice-dispatch-controller"

// EndpointsliceDispatchController will reconcile a MultiClusterService object
type EndpointsliceDispatchController struct {
	client.Client
	EventRecorder      record.EventRecorder
	RESTMapper         meta.RESTMapper
	InformerManager    genericmanager.MultiClusterInformerManager
	RateLimiterOptions ratelimiterflag.Options
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
func (c *EndpointsliceDispatchController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling Work", "namespacedName", req.NamespacedName.String())

	work := &workv1alpha1.Work{}
	if err := c.Client.Get(ctx, req.NamespacedName, work); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	if !util.IsWorkContains(work.Spec.Workload.Manifests, util.EndpointSliceGVK) {
		return controllerruntime.Result{}, nil
	}

	mcsName := util.GetLabelValue(work.Labels, util.MultiClusterServiceNameLabel)
	if !work.DeletionTimestamp.IsZero() || mcsName == "" {
		if err := c.cleanupEndpointSliceFromConsumerClusters(ctx, work); err != nil {
			klog.ErrorS(err, "Failed to cleanup EndpointSlice from consumer clusters for work", "namespace", work.Namespace, "name", work.Name)
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, nil
	}

	mcsNS := util.GetLabelValue(work.Labels, util.MultiClusterServiceNamespaceLabel)
	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, types.NamespacedName{Namespace: mcsNS, Name: mcsName}, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			klog.ErrorS(err, "MultiClusterService is not found", "namespace", mcsNS, "name", mcsName)
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

	var err error
	defer func() {
		if err != nil {
			_ = c.updateEndpointSliceDispatched(ctx, mcs, metav1.ConditionFalse, "EndpointSliceDispatchedFailed", err.Error())
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonDispatchEndpointSliceFailed, err.Error())
			return
		}
		_ = c.updateEndpointSliceDispatched(ctx, mcs, metav1.ConditionTrue, "EndpointSliceDispatchedSucceed", "EndpointSlice are dispatched successfully")
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventReasonDispatchEndpointSliceSucceed, "EndpointSlice are dispatched successfully")
	}()

	if err = c.cleanOrphanDispatchedEndpointSlice(ctx, mcs); err != nil {
		return controllerruntime.Result{}, err
	}

	if err = c.dispatchEndpointSlice(ctx, work.DeepCopy(), mcs); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}

func (c *EndpointsliceDispatchController) updateEndpointSliceDispatched(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, status metav1.ConditionStatus, reason, message string) error {
	EndpointSliceCollected := metav1.Condition{
		Type:               networkingv1alpha1.EndpointSliceDispatched,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, mcs, func() error {
			meta.SetStatusCondition(&mcs.Status.Conditions, EndpointSliceCollected)
			return nil
		})
		return err
	})
}

// SetupWithManager creates a controller and register to controller manager.
func (c *EndpointsliceDispatchController) SetupWithManager(mgr controllerruntime.Manager) error {
	workPredicateFun := predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			// We only care about the EndpointSlice work from provider clusters
			return util.GetLabelValue(createEvent.Object.GetLabels(), util.MultiClusterServiceNameLabel) != "" &&
				util.GetAnnotationValue(createEvent.Object.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// We only care about the EndpointSlice work from provider clusters
			// TBD: We care about the work with label util.ServiceNameLabel because now service-export-controller and endpointslice-collect-controller
			// will manage the work together, should delete this after the conflict is fixed
			return (util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.MultiClusterServiceNameLabel) != "" ||
				util.GetLabelValue(updateEvent.ObjectNew.GetLabels(), util.ServiceNameLabel) != "") &&
				util.GetAnnotationValue(updateEvent.ObjectNew.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			// We only care about the EndpointSlice work from provider clusters
			return util.GetLabelValue(deleteEvent.Object.GetLabels(), util.MultiClusterServiceNameLabel) != "" &&
				util.GetAnnotationValue(deleteEvent.Object.GetAnnotations(), util.EndpointSliceProvisionClusterAnnotation) == ""
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(EndpointsliceDispatchControllerName).
		For(&workv1alpha1.Work{}, builder.WithPredicates(workPredicateFun)).
		Watches(&networkingv1alpha1.MultiClusterService{}, handler.EnqueueRequestsFromMapFunc(c.newMultiClusterServiceFunc())).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(c.newClusterFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *EndpointsliceDispatchController) newClusterFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		var clusterName string
		switch t := a.(type) {
		case *clusterv1alpha1.Cluster:
			clusterName = t.Name
		default:
			return nil
		}

		mcsList := &networkingv1alpha1.MultiClusterServiceList{}
		if err := c.Client.List(ctx, mcsList, &client.ListOptions{}); err != nil {
			klog.ErrorS(err, "Failed to list MultiClusterService")
			return nil
		}

		var requests []reconcile.Request
		for _, mcs := range mcsList.Items {
			clusterSet, err := helper.GetConsumerClusters(c.Client, mcs.DeepCopy())
			if err != nil {
				klog.ErrorS(err, "Failed to get provider clusters")
				continue
			}

			if !clusterSet.Has(clusterName) {
				continue
			}

			workList, err := c.getClusterEndpointSliceWorks(ctx, mcs.Namespace, mcs.Name)
			if err != nil {
				klog.ErrorS(err, "Failed to list work")
				continue
			}
			for _, work := range workList {
				// This annotation is only added to the EndpointSlice work in consumer clusters' execution namespace
				// Here, when new cluster joins to Karmada and it's in the consumer cluster set, we need to re-sync the EndpointSlice work
				// from provider cluster to the newly joined cluster
				if util.GetLabelValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) != "" {
					continue
				}

				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: work.Namespace, Name: work.Name}})
			}
		}
		return requests
	}
}

func (c *EndpointsliceDispatchController) getClusterEndpointSliceWorks(ctx context.Context, mcsNamespace, mcsName string) ([]workv1alpha1.Work, error) {
	workList := &workv1alpha1.WorkList{}
	if err := c.Client.List(ctx, workList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			util.MultiClusterServiceNameLabel:      mcsName,
			util.MultiClusterServiceNamespaceLabel: mcsNamespace,
		}),
	}); err != nil {
		klog.ErrorS(err, "Failed to list work")
		return nil, err
	}

	return workList.Items, nil
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

		workList, err := c.getClusterEndpointSliceWorks(ctx, mcsNamespace, mcsName)
		if err != nil {
			klog.ErrorS(err, "Failed to list work")
			return nil
		}

		var requests []reconcile.Request
		for _, work := range workList {
			// We only care about the EndpointSlice work from provider clusters
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
		klog.ErrorS(err, "Failed to list works")
		return err
	}

	for _, work := range workList.Items {
		// We only care about the EndpointSlice work in consumer clusters
		if util.GetAnnotationValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) == "" {
			continue
		}

		consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
		if err != nil {
			klog.ErrorS(err, "Failed to get consumer clusters")
			return err
		}

		cluster, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.ErrorS(err, "Failed to get cluster name for work", "namespace", work.Namespace, "name", work.Name)
			return err
		}

		if consumerClusters.Has(cluster) {
			continue
		}

		if err = c.Client.Delete(ctx, work.DeepCopy()); err != nil {
			klog.ErrorS(err, "Failed to delete work", "namespace", work.Namespace, "name", work.Name)
			return err
		}
	}

	return nil
}

func (c *EndpointsliceDispatchController) dispatchEndpointSlice(ctx context.Context, work *workv1alpha1.Work, mcs *networkingv1alpha1.MultiClusterService) error {
	epsSourceCluster, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get EndpointSlice source cluster name for work", "namespace", work.Namespace, "name", work.Name)
		return err
	}

	consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
	if err != nil {
		klog.ErrorS(err, "Failed to get consumer clusters")
		return err
	}
	for clusterName := range consumerClusters {
		if clusterName == epsSourceCluster {
			continue
		}
		clusterObj, err := util.GetCluster(c.Client, clusterName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonClusterNotFound, "Consumer cluster %s is not found", clusterName)
				continue
			}
			klog.ErrorS(err, "Failed to get cluster", "cluster", clusterName)
			return err
		}
		if !util.IsClusterReady(&clusterObj.Status) {
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
				"Consumer cluster %s is not ready, skip to propagate EndpointSlice", clusterName)
			continue
		}
		if !helper.IsAPIEnabled(clusterObj.Status.APIEnablements, util.EndpointSliceGVK.GroupVersion().String(), util.EndpointSliceGVK.Kind) {
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonAPIIncompatible, "Consumer cluster %s does not support EndpointSlice", clusterName)
			continue
		}

		if err = c.ensureEndpointSliceWork(ctx, mcs, work, epsSourceCluster, clusterName); err != nil {
			return err
		}
	}
	return nil
}

func (c *EndpointsliceDispatchController) ensureEndpointSliceWork(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService,
	work *workv1alpha1.Work, providerCluster, consumerCluster string) error {
	// It couldn't happen here
	if len(work.Spec.Workload.Manifests) == 0 {
		return nil
	}

	// There should be only one manifest in the work, let's use the first one.
	manifest := work.Spec.Workload.Manifests[0]
	unstructuredObj := &unstructured.Unstructured{}
	if err := unstructuredObj.UnmarshalJSON(manifest.Raw); err != nil {
		klog.ErrorS(err, "Failed to unmarshal work manifest")
		return err
	}

	endpointSlice := &discoveryv1.EndpointSlice{}
	if err := helper.ConvertToTypedObject(unstructuredObj, endpointSlice); err != nil {
		klog.ErrorS(err, "Failed to convert unstructured object to typed object")
		return err
	}

	// Use this name to avoid naming conflicts and locate the EPS source cluster.
	endpointSlice.Name = providerCluster + "-" + endpointSlice.Name
	clusterNamespace := names.GenerateExecutionSpaceName(consumerCluster)
	endpointSlice.Labels = map[string]string{
		discoveryv1.LabelServiceName: mcs.Name,
		discoveryv1.LabelManagedBy:   util.EndpointSliceDispatchControllerLabelValue,
	}
	endpointSlice.Annotations = map[string]string{
		// This annotation is used to identify the source cluster of EndpointSlice and whether the eps are the newest version
		util.EndpointSliceProvisionClusterAnnotation: providerCluster,
	}

	workMeta := metav1.ObjectMeta{
		Name:       work.Name,
		Namespace:  clusterNamespace,
		Finalizers: []string{util.ExecutionControllerFinalizer},
		Annotations: map[string]string{
			util.EndpointSliceProvisionClusterAnnotation: providerCluster,
		},
		Labels: map[string]string{
			util.MultiClusterServiceNameLabel:      mcs.Name,
			util.MultiClusterServiceNamespaceLabel: mcs.Namespace,
		},
	}
	unstructuredEPS, err := helper.ToUnstructured(endpointSlice)
	if err != nil {
		klog.ErrorS(err, "Failed to convert typed object to unstructured object")
		return err
	}
	if err := ctrlutil.CreateOrUpdateWork(ctx, c.Client, workMeta, unstructuredEPS); err != nil {
		klog.ErrorS(err, "Failed to dispatch EndpointSlice",
			"namespace", work.GetNamespace(), "name", work.GetName(), "providerCluster", providerCluster, "consumerCluster", consumerCluster)
		return err
	}

	return nil
}

func (c *EndpointsliceDispatchController) cleanupEndpointSliceFromConsumerClusters(ctx context.Context, work *workv1alpha1.Work) error {
	// TBD: There may be a better way without listing all works.
	workList := &workv1alpha1.WorkList{}
	err := c.Client.List(ctx, workList)
	if err != nil {
		klog.ErrorS(err, "Failed to list works")
		return err
	}

	epsSourceCluster, err := names.GetClusterName(work.Namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to get EndpointSlice provider cluster name for work", "namespace", work.Namespace, "name", work.Name)
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
			klog.ErrorS(err, "Failed to remove finalizer for work", "finalizer", util.MCSEndpointSliceDispatchControllerFinalizer, "namespace", work.Namespace, "name", work.Name)
			return err
		}
	}

	return nil
}
