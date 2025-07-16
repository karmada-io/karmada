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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/ctrlutil"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ControllerName is the controller name that will be used when reporting events and metrics.
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
		klog.ErrorS(err, "Failed to get MultiClusterService object", "namespacedName", req.NamespacedName)
		return controllerruntime.Result{}, err
	}

	if !mcs.DeletionTimestamp.IsZero() {
		return c.handleMultiClusterServiceDelete(ctx, mcs.DeepCopy())
	}

	var err error
	defer func() {
		if err != nil {
			_ = c.updateMultiClusterServiceStatus(ctx, mcs, metav1.ConditionFalse, "ServiceAppliedFailed", err.Error())
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed, err.Error())
			return
		}
		_ = c.updateMultiClusterServiceStatus(ctx, mcs, metav1.ConditionTrue, "ServiceAppliedSucceed", "Service is propagated to target clusters.")
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventReasonSyncServiceSucceed, "Service is propagated to target clusters.")
	}()

	if err = c.handleMultiClusterServiceCreateOrUpdate(ctx, mcs.DeepCopy()); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (c *MCSController) handleMultiClusterServiceDelete(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Begin to handle MultiClusterService delete event", "namespace", mcs.Namespace, "name", mcs.Name)

	if err := c.retrieveService(ctx, mcs); err != nil {
		c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
			fmt.Sprintf("failed to delete service work :%v", err))
		return controllerruntime.Result{}, err
	}

	if err := c.retrieveMultiClusterService(ctx, mcs, nil); err != nil {
		c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
			fmt.Sprintf("failed to delete MultiClusterService work :%v", err))
		return controllerruntime.Result{}, err
	}

	if controllerutil.RemoveFinalizer(mcs, util.MCSControllerFinalizer) {
		err := c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "Failed to update MultiClusterService with finalizer", "namespace", mcs.Namespace, "name", mcs.Name)
			return controllerruntime.Result{}, err
		}
	}

	klog.V(4).InfoS("Success to delete MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
	return controllerruntime.Result{}, nil
}

func (c *MCSController) retrieveMultiClusterService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, providerClusters sets.Set[string]) error {
	mcsID := util.GetLabelValue(mcs.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel)
	workList, err := helper.GetWorksByLabelsSet(ctx, c, labels.Set{
		networkingv1alpha1.MultiClusterServicePermanentIDLabel: mcsID,
	})
	if err != nil {
		klog.ErrorS(err, "Failed to list work by MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	for _, work := range workList.Items {
		if !util.IsWorkContains(work.Spec.Workload.Manifests, multiClusterServiceGVK) {
			continue
		}
		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.ErrorS(err, "Failed to get member cluster name for work", "namespace", work.Namespace, "name", work.Name)
			continue
		}

		if providerClusters.Has(clusterName) {
			continue
		}

		if err = c.cleanProviderEndpointSliceWork(ctx, work.DeepCopy()); err != nil {
			klog.ErrorS(err, "Failed to clean provider EndpointSlice work", "namespace", work.Namespace, "name", work.Name)
			return err
		}

		if err = c.Client.Delete(ctx, work.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Error while deleting work", "namespace", work.Namespace, "name", work.Name)
			return err
		}
	}

	klog.V(4).InfoS("Success to clean up MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
	return nil
}

func (c *MCSController) cleanProviderEndpointSliceWork(ctx context.Context, work *workv1alpha1.Work) error {
	workList := &workv1alpha1.WorkList{}
	if err := c.List(ctx, workList, &client.ListOptions{
		Namespace: work.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.MultiClusterServiceNameLabel:      util.GetLabelValue(work.Labels, util.MultiClusterServiceNameLabel),
			util.MultiClusterServiceNamespaceLabel: util.GetLabelValue(work.Labels, util.MultiClusterServiceNamespaceLabel),
		}),
	}); err != nil {
		klog.ErrorS(err, "Failed to list workList reported by work(MultiClusterService)", "namespace", work.Namespace, "name", work.Name)
		return err
	}

	var errs []error
	for _, work := range workList.Items {
		if !util.IsWorkContains(work.Spec.Workload.Manifests, util.EndpointSliceGVK) {
			continue
		}
		// This annotation is only added to the EndpointSlice work in consumer clusters' execution namespace
		// Here we only care about the EndpointSlice work in provider clusters' execution namespace
		if util.GetAnnotationValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) != "" {
			continue
		}

		if err := cleanProviderClustersEndpointSliceWork(ctx, c.Client, work.DeepCopy()); err != nil {
			errs = append(errs, err)
		}
	}
	if err := utilerrors.NewAggregate(errs); err != nil {
		return err
	}

	return nil
}

func (c *MCSController) handleMultiClusterServiceCreateOrUpdate(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	klog.V(4).InfoS("Begin to handle MultiClusterService create or update event", "namespace", mcs.Namespace, "name", mcs.Name)

	providerClusters, err := helper.GetProviderClusters(c.Client, mcs)
	if err != nil {
		klog.ErrorS(err, "Failed to get provider clusters by MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}
	consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
	if err != nil {
		klog.ErrorS(err, "Failed to get consumer clusters by MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	// 1. if mcs not contain CrossCluster type, delete service work if needed
	if !helper.MultiClusterServiceCrossClusterEnabled(mcs) {
		if err = c.retrieveService(ctx, mcs); err != nil {
			return err
		}
		if err = c.retrieveMultiClusterService(ctx, mcs, providerClusters); err != nil {
			return err
		}
		if controllerutil.RemoveFinalizer(mcs, util.MCSControllerFinalizer) {
			err := c.Client.Update(ctx, mcs)
			if err != nil {
				klog.ErrorS(err, "Failed to remove finalizer from MultiClusterService", "finalizer", util.MCSControllerFinalizer, "namespace", mcs.Namespace, "name", mcs.Name)
				return err
			}
		}
		return nil
	}

	// 2. add finalizer if needed
	if controllerutil.AddFinalizer(mcs, util.MCSControllerFinalizer) {
		err = c.Client.Update(ctx, mcs)
		if err != nil {
			klog.ErrorS(err, "Failed to add finalizer to MultiClusterService", "finalizer", util.MCSControllerFinalizer, "namespace", mcs.Namespace, "name", mcs.Name)
			return err
		}
	}

	// 3. Generate the MCS work in target clusters' namespace
	if err = c.propagateMultiClusterService(ctx, mcs, providerClusters); err != nil {
		return err
	}

	// 4. delete MultiClusterService work not in provider clusters and in the unready clusters
	if err = c.retrieveMultiClusterService(ctx, mcs, providerClusters); err != nil {
		return err
	}

	// 5. make sure service exist
	svc := &corev1.Service{}
	err = c.Client.Get(ctx, types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}, svc)
	// If the Service is deleted, the Service's ResourceBinding will be cleaned by GC
	if err != nil {
		klog.ErrorS(err, "Failed to get service", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	// 6. if service exists, create or update corresponding ResourceBinding
	if err = c.propagateService(ctx, mcs, svc, providerClusters, consumerClusters); err != nil {
		return err
	}

	klog.V(4).InfoS("Success to reconcile MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
	return nil
}

func (c *MCSController) propagateMultiClusterService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, providerClusters sets.Set[string]) error {
	for clusterName := range providerClusters {
		clusterObj, err := util.GetCluster(c.Client, clusterName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonClusterNotFound, "Provider cluster %s is not found", clusterName)
				continue
			}
			klog.ErrorS(err, "Failed to get cluster", "cluster", clusterName)
			return err
		}
		if !util.IsClusterReady(&clusterObj.Status) {
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
				"Provider cluster %s is not ready, skip to propagate MultiClusterService", clusterName)
			continue
		}
		if !helper.IsAPIEnabled(clusterObj.Status.APIEnablements, util.EndpointSliceGVK.GroupVersion().String(), util.EndpointSliceGVK.Kind) {
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonAPIIncompatible, "Provider cluster %s does not support EndpointSlice", clusterName)
			continue
		}

		workMeta := metav1.ObjectMeta{
			Name:      names.GenerateWorkName(mcs.Kind, mcs.Name, mcs.Namespace),
			Namespace: names.GenerateExecutionSpaceName(clusterName),
			Labels: map[string]string{
				// We add this id in mutating webhook, let's just use it
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: util.GetLabelValue(mcs.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel),
				util.MultiClusterServiceNamespaceLabel:                 mcs.Namespace,
				util.MultiClusterServiceNameLabel:                      mcs.Name,
			},
		}

		mcsObj, err := helper.ToUnstructured(mcs)
		if err != nil {
			klog.ErrorS(err, "Failed to convert MultiClusterService to unstructured object", "namespace", mcs.Namespace, "name", mcs.Name)
			return err
		}
		if err = ctrlutil.CreateOrUpdateWork(ctx, c, workMeta, mcsObj, ctrlutil.WithSuspendDispatching(true)); err != nil {
			klog.ErrorS(err, "Failed to create or update MultiClusterService work in the given member cluster",
				"namespace", mcs.Namespace, "name", mcs.Name, "cluster", clusterName)
			return err
		}
	}

	return nil
}

func (c *MCSController) retrieveService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService) error {
	svc := &corev1.Service{}
	err := c.Client.Get(ctx, types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}, svc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.ErrorS(err, "Failed to get service", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	if apierrors.IsNotFound(err) {
		return nil
	}

	svcCopy := svc.DeepCopy()
	if svcCopy.Labels != nil {
		delete(svcCopy.Labels, util.ResourceTemplateClaimedByLabel)
		delete(svcCopy.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel)
	}

	if err = c.Client.Update(ctx, svcCopy); err != nil {
		klog.ErrorS(err, "Failed to update service", "namespace", mcs.Namespace, "name", mcs.Name)
		return err
	}

	rb := &workv1alpha2.ResourceBinding{}
	err = c.Client.Get(ctx, types.NamespacedName{Namespace: mcs.Namespace, Name: names.GenerateBindingName(svc.Kind, svc.Name)}, rb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.ErrorS(err, "Failed to get ResourceBinding", "namespace", mcs.Namespace, "name", names.GenerateBindingName(svc.Kind, svc.Name))
		return err
	}

	// MultiClusterService is a specialized instance of PropagationPolicy, and when deleting PropagationPolicy, the resource ResourceBinding
	// will not be removed to ensure service stability. MultiClusterService should also adhere to this design.
	rbCopy := rb.DeepCopy()
	if rbCopy.Annotations != nil {
		delete(rbCopy.Annotations, networkingv1alpha1.MultiClusterServiceNameAnnotation)
		delete(rbCopy.Annotations, networkingv1alpha1.MultiClusterServiceNamespaceAnnotation)
	}
	if rbCopy.Labels != nil {
		delete(rbCopy.Labels, workv1alpha2.BindingManagedByLabel)
		delete(rbCopy.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel)
	}
	if err := c.Client.Update(ctx, rbCopy); err != nil {
		klog.ErrorS(err, "Failed to update ResourceBinding", "namespace", mcs.Namespace, "name", names.GenerateBindingName(svc.Kind, svc.Name))
		return err
	}

	return nil
}

func (c *MCSController) propagateService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, svc *corev1.Service,
	providerClusters, consumerClusters sets.Set[string]) error {
	if err := c.claimMultiClusterServiceForService(ctx, svc, mcs); err != nil {
		klog.ErrorS(err, "Failed to claim for Service", "namespace", svc.Namespace, "name", svc.Name)
		return err
	}

	binding, err := c.buildResourceBinding(svc, mcs, providerClusters, consumerClusters)
	if err != nil {
		klog.ErrorS(err, "Failed to build ResourceBinding for Service", "namespace", svc.Namespace, "name", svc.Name)
		return err
	}

	var operationResult controllerutil.OperationResult
	bindingCopy := binding.DeepCopy()
	err = retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		operationResult, err = controllerutil.CreateOrUpdate(ctx, c.Client, bindingCopy, func() error {
			// If this binding exists and its owner is not the input object, return error and let garbage collector
			// delete this binding and try again later. See https://github.com/karmada-io/karmada/issues/2090.
			if ownerRef := metav1.GetControllerOfNoCopy(bindingCopy); ownerRef != nil && ownerRef.UID != svc.GetUID() {
				return fmt.Errorf("failed to update binding due to different owner reference UID, will " +
					"try again later after binding is garbage collected, see https://github.com/karmada-io/karmada/issues/2090")
			}

			// Just update necessary fields, especially avoid modifying Spec.Clusters which is scheduling result, if already exists.
			bindingCopy.Annotations = util.DedupeAndMergeAnnotations(bindingCopy.Annotations, binding.Annotations)
			bindingCopy.Labels = util.DedupeAndMergeLabels(bindingCopy.Labels, binding.Labels)
			bindingCopy.OwnerReferences = binding.OwnerReferences
			bindingCopy.Finalizers = binding.Finalizers
			bindingCopy.Spec.Placement = binding.Spec.Placement
			bindingCopy.Spec.Resource = binding.Spec.Resource
			bindingCopy.Spec.ConflictResolution = binding.Spec.ConflictResolution
			if binding.Spec.Suspension != nil {
				if bindingCopy.Spec.Suspension == nil {
					bindingCopy.Spec.Suspension = &workv1alpha2.Suspension{}
				}
				bindingCopy.Spec.Suspension.Suspension = binding.Spec.Suspension.Suspension
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.ErrorS(err, "Failed to create/update ResourceBinding", "namespace", bindingCopy.Namespace, "name", bindingCopy.Name)
		return err
	}

	switch operationResult {
	case controllerutil.OperationResultCreated:
		klog.InfoS("Create ResourceBinding successfully.", "namespace", binding.GetNamespace(), "name", binding.GetName())
	case controllerutil.OperationResultUpdated:
		klog.InfoS("Update ResourceBinding successfully.", "namespace", binding.GetNamespace(), "name", binding.GetName())
	default:
		klog.V(2).InfoS("ResourceBinding is up to date.", "namespace", binding.GetNamespace(), "name", binding.GetName())
	}

	return nil
}

func (c *MCSController) buildResourceBinding(svc *corev1.Service, mcs *networkingv1alpha1.MultiClusterService,
	providerClusters, consumerClusters sets.Set[string]) (*workv1alpha2.ResourceBinding, error) {
	propagateClusters := providerClusters.Clone().Insert(consumerClusters.Clone().UnsortedList()...)
	placement := &policyv1alpha1.Placement{
		ClusterAffinity: &policyv1alpha1.ClusterAffinity{
			ClusterNames: propagateClusters.UnsortedList(),
		},
	}

	propagationBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.GenerateBindingName(svc.Kind, svc.Name),
			Namespace: svc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: svc.APIVersion, Kind: svc.Kind, Name: svc.Name, UID: svc.UID},
			},
			Annotations: map[string]string{
				networkingv1alpha1.MultiClusterServiceNameAnnotation:      mcs.Name,
				networkingv1alpha1.MultiClusterServiceNamespaceAnnotation: mcs.Namespace,
			},
			Labels: map[string]string{
				workv1alpha2.BindingManagedByLabel:                     util.MultiClusterServiceKind,
				networkingv1alpha1.MultiClusterServicePermanentIDLabel: util.GetLabelValue(mcs.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel),
			},
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Placement:          placement,
			ConflictResolution: policyv1alpha1.ConflictOverwrite,
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      svc.APIVersion,
				Kind:            svc.Kind,
				Namespace:       svc.Namespace,
				Name:            svc.Name,
				UID:             svc.UID,
				ResourceVersion: svc.ResourceVersion,
			},
		},
	}

	return propagationBinding, nil
}

func (c *MCSController) claimMultiClusterServiceForService(ctx context.Context, svc *corev1.Service, mcs *networkingv1alpha1.MultiClusterService) error {
	svcCopy := svc.DeepCopy()
	if svcCopy.Labels == nil {
		svcCopy.Labels = map[string]string{}
	}
	if svcCopy.Annotations == nil {
		svcCopy.Annotations = map[string]string{}
	}

	delete(svcCopy.Labels, policyv1alpha1.PropagationPolicyPermanentIDLabel)
	delete(svcCopy.Labels, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel)

	svcCopy.Labels[util.ResourceTemplateClaimedByLabel] = util.MultiClusterServiceKind
	svcCopy.Labels[networkingv1alpha1.MultiClusterServicePermanentIDLabel] = util.GetLabelValue(mcs.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel)

	// cleanup policy annotations
	delete(svcCopy.Annotations, policyv1alpha1.PropagationPolicyNameAnnotation)
	delete(svcCopy.Annotations, policyv1alpha1.PropagationPolicyNamespaceAnnotation)
	delete(svcCopy.Annotations, policyv1alpha1.ClusterPropagationPolicyAnnotation)

	svcCopy.Annotations[networkingv1alpha1.MultiClusterServiceNameAnnotation] = mcs.Name
	svcCopy.Annotations[networkingv1alpha1.MultiClusterServiceNamespaceAnnotation] = mcs.Namespace

	if err := c.Client.Update(ctx, svcCopy); err != nil {
		klog.ErrorS(err, "Failed to update service", "namespace", svc.Namespace, "name", svc.Name)
		return err
	}

	return nil
}

func (c *MCSController) updateMultiClusterServiceStatus(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, status metav1.ConditionStatus, reason, message string) error {
	serviceAppliedCondition := metav1.Condition{
		Type:               networkingv1alpha1.MCSServiceAppliedConditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(ctx, c.Client, mcs, func() error {
			meta.SetStatusCondition(&mcs.Status.Conditions, serviceAppliedCondition)
			return nil
		})
		return err
	})
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
			svc := e.Object.(*corev1.Service)

			return c.serviceHasCrossClusterMultiClusterService(svc)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			svcOld := e.ObjectOld.(*corev1.Service)
			svcNew := e.ObjectNew.(*corev1.Service)

			// We only care about the update events below and do not care about the status updating
			if equality.Semantic.DeepEqual(svcOld.Annotations, svcNew.Annotations) &&
				equality.Semantic.DeepEqual(svcOld.Labels, svcNew.Labels) &&
				equality.Semantic.DeepEqual(svcOld.Spec, svcNew.Spec) {
				return false
			}
			return c.serviceHasCrossClusterMultiClusterService(svcNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			svc := e.Object.(*corev1.Service)
			return c.serviceHasCrossClusterMultiClusterService(svc)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}

	svcMapFunc := handler.MapFunc(
		func(_ context.Context, svcObj client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: svcObj.GetNamespace(),
					Name:      svcObj.GetName(),
				},
			}}
		})

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&networkingv1alpha1.MultiClusterService{}, builder.WithPredicates(mcsPredicateFunc)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(svcMapFunc), builder.WithPredicates(svcPredicateFunc)).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(c.clusterMapFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		Complete(c)
}

func (c *MCSController) serviceHasCrossClusterMultiClusterService(svc *corev1.Service) bool {
	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(context.Background(),
		types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, mcs); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Failed to get MultiClusterService", "namespace", svc.Namespace, "name", svc.Name)
		}
		return false
	}

	return helper.MultiClusterServiceCrossClusterEnabled(mcs)
}

func (c *MCSController) clusterMapFunc() handler.MapFunc {
	return func(ctx context.Context, a client.Object) []reconcile.Request {
		var clusterName string
		switch t := a.(type) {
		case *clusterv1alpha1.Cluster:
			clusterName = t.Name
		default:
			return nil
		}

		klog.V(4).InfoS("Begin to sync mcs with cluster", "cluster", clusterName)
		mcsList := &networkingv1alpha1.MultiClusterServiceList{}
		if err := c.Client.List(ctx, mcsList, &client.ListOptions{}); err != nil {
			klog.ErrorS(err, "Failed to list MultiClusterService")
			return nil
		}

		var requests []reconcile.Request
		for index := range mcsList.Items {
			if need, err := c.needSyncMultiClusterService(&mcsList.Items[index], clusterName); err != nil || !need {
				continue
			}

			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mcsList.Items[index].Namespace,
				Name: mcsList.Items[index].Name}})
		}

		return requests
	}
}

func (c *MCSController) needSyncMultiClusterService(mcs *networkingv1alpha1.MultiClusterService, clusterName string) (bool, error) {
	if !helper.MultiClusterServiceCrossClusterEnabled(mcs) {
		return false, nil
	}

	if len(mcs.Spec.ProviderClusters) == 0 || len(mcs.Spec.ConsumerClusters) == 0 {
		return true, nil
	}

	providerClusters, err := helper.GetProviderClusters(c.Client, mcs)
	if err != nil {
		klog.ErrorS(err, "Failed to get provider clusters by MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
		return false, err
	}
	if providerClusters.Has(clusterName) {
		return true, nil
	}

	consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
	if err != nil {
		klog.ErrorS(err, "Failed to get consumer clusters by MultiClusterService", "namespace", mcs.Namespace, "name", mcs.Name)
		return false, err
	}
	if consumerClusters.Has(clusterName) {
		return true, nil
	}
	return false, nil
}
