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
	klog.V(4).Infof("Reconciling MultiClusterService(%s/%s)", req.Namespace, req.Name)

	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(ctx, req.NamespacedName, mcs); err != nil {
		if apierrors.IsNotFound(err) {
			// The mcs no longer exist, in which case we stop processing.
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("Failed to get MultiClusterService object(%s):%v", req.NamespacedName, err)
		return controllerruntime.Result{}, err
	}

	if !mcs.DeletionTimestamp.IsZero() {
		return c.handleMultiClusterServiceDelete(mcs.DeepCopy())
	}

	var err error
	defer func() {
		if err != nil {
			_ = c.updateMultiClusterServiceStatus(mcs, metav1.ConditionFalse, "ServiceAppliedFailed", err.Error())
			c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed, err.Error())
			return
		}
		_ = c.updateMultiClusterServiceStatus(mcs, metav1.ConditionTrue, "ServiceAppliedSucceed", "Service is propagated to target clusters.")
		c.EventRecorder.Eventf(mcs, corev1.EventTypeNormal, events.EventReasonSyncServiceSucceed, "Service is propagated to target clusters.")
	}()

	if err = c.handleMultiClusterServiceCreateOrUpdate(mcs.DeepCopy()); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (c *MCSController) handleMultiClusterServiceDelete(mcs *networkingv1alpha1.MultiClusterService) (controllerruntime.Result, error) {
	klog.V(4).Infof("Begin to handle MultiClusterService(%s/%s) delete event", mcs.Namespace, mcs.Name)

	if err := c.retrieveService(mcs); err != nil {
		c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
			fmt.Sprintf("failed to delete service work :%v", err))
		return controllerruntime.Result{}, err
	}

	if err := c.retrieveMultiClusterService(mcs, nil); err != nil {
		c.EventRecorder.Event(mcs, corev1.EventTypeWarning, events.EventReasonSyncServiceFailed,
			fmt.Sprintf("failed to delete MultiClusterService work :%v", err))
		return controllerruntime.Result{}, err
	}

	finalizersUpdated := controllerutil.RemoveFinalizer(mcs, util.MCSControllerFinalizer)
	if finalizersUpdated {
		err := c.Client.Update(context.Background(), mcs)
		if err != nil {
			klog.Errorf("Failed to update MultiClusterService(%s/%s) with finalizer:%v", mcs.Namespace, mcs.Name, err)
			return controllerruntime.Result{}, err
		}
	}

	klog.V(4).Infof("Success to delete MultiClusterService(%s/%s)", mcs.Namespace, mcs.Name)
	return controllerruntime.Result{}, nil
}

func (c *MCSController) retrieveMultiClusterService(mcs *networkingv1alpha1.MultiClusterService, providerClusters sets.Set[string]) error {
	mcsID := util.GetLabelValue(mcs.Labels, networkingv1alpha1.MultiClusterServicePermanentIDLabel)
	workList, err := helper.GetWorksByLabelsSet(c, labels.Set{
		networkingv1alpha1.MultiClusterServicePermanentIDLabel: mcsID,
	})
	if err != nil {
		klog.Errorf("Failed to list work by MultiClusterService(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return err
	}

	for _, work := range workList.Items {
		if !helper.IsWorkContains(work.Spec.Workload.Manifests, multiClusterServiceGVK) {
			continue
		}
		clusterName, err := names.GetClusterName(work.Namespace)
		if err != nil {
			klog.Errorf("Failed to get member cluster name for work %s/%s:%v", work.Namespace, work.Name, work)
			continue
		}

		if providerClusters.Has(clusterName) && c.IsClusterReady(clusterName) {
			continue
		}

		if err := c.cleanProviderEndpointSliceWork(work.DeepCopy()); err != nil {
			klog.Errorf("Failed to clean provider EndpointSlice work(%s/%s):%v", work.Namespace, work.Name, err)
			return err
		}

		if err = c.Client.Delete(context.TODO(), work.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Error while deleting work(%s/%s): %v", work.Namespace, work.Name, err)
			return err
		}
	}

	klog.V(4).Infof("Success to delete MultiClusterService(%s/%s) work:%v", mcs.Namespace, mcs.Name, err)
	return nil
}

func (c *MCSController) cleanProviderEndpointSliceWork(work *workv1alpha1.Work) error {
	workList := &workv1alpha1.WorkList{}
	if err := c.List(context.TODO(), workList, &client.ListOptions{
		Namespace: work.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			util.MultiClusterServiceNameLabel:      util.GetLabelValue(work.Labels, util.MultiClusterServiceNameLabel),
			util.MultiClusterServiceNamespaceLabel: util.GetLabelValue(work.Labels, util.MultiClusterServiceNamespaceLabel),
		}),
	}); err != nil {
		klog.Errorf("Failed to list workList reported by work(MultiClusterService)(%s/%s): %v", work.Namespace, work.Name, err)
		return err
	}

	var errs []error
	for _, work := range workList.Items {
		if !helper.IsWorkContains(work.Spec.Workload.Manifests, util.EndpointSliceGVK) {
			continue
		}
		// This annotation is only added to the EndpointSlice work in consumer clusters' execution namespace
		// Here we only care about the EndpointSlice work in provider clusters' execution namespace
		if util.GetAnnotationValue(work.Annotations, util.EndpointSliceProvisionClusterAnnotation) != "" {
			continue
		}

		if err := cleanProviderClustersEndpointSliceWork(c.Client, work.DeepCopy()); err != nil {
			errs = append(errs, err)
		}
	}
	if err := utilerrors.NewAggregate(errs); err != nil {
		return err
	}

	// TBD: This is needed because we add this finalizer in version 1.8.0, delete this in version 1.10.0
	if controllerutil.RemoveFinalizer(work, util.MCSEndpointSliceCollectControllerFinalizer) {
		if err := c.Update(context.TODO(), work); err != nil {
			klog.Errorf("Failed to update work(%s/%s), Error: %v", work.Namespace, work.Name, err)
			return err
		}
	}

	return nil
}

func (c *MCSController) handleMultiClusterServiceCreateOrUpdate(mcs *networkingv1alpha1.MultiClusterService) error {
	klog.V(4).Infof("Begin to handle MultiClusterService(%s/%s) create or update event", mcs.Namespace, mcs.Name)

	providerClusters, err := helper.GetProviderClusters(c.Client, mcs)
	if err != nil {
		klog.Errorf("Failed to get provider clusters by MultiClusterService(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return err
	}
	consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
	if err != nil {
		klog.Errorf("Failed to get consumer clusters by MultiClusterService(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return err
	}

	// 1. if mcs not contain CrossCluster type, delete service work if needed
	if !helper.MultiClusterServiceCrossClusterEnabled(mcs) {
		if err := c.retrieveService(mcs); err != nil {
			return err
		}
		if err := c.retrieveMultiClusterService(mcs, providerClusters); err != nil {
			return err
		}
		finalizersUpdated := controllerutil.RemoveFinalizer(mcs, util.MCSControllerFinalizer)
		if finalizersUpdated {
			err := c.Client.Update(context.Background(), mcs)
			if err != nil {
				klog.Errorf("Failed to remove finalizer(%s) from MultiClusterService(%s/%s):%v", util.MCSControllerFinalizer, mcs.Namespace, mcs.Name, err)
				return err
			}
		}
		return nil
	}

	// 2. add finalizer if needed
	finalizersUpdated := controllerutil.AddFinalizer(mcs, util.MCSControllerFinalizer)
	if finalizersUpdated {
		err := c.Client.Update(context.Background(), mcs)
		if err != nil {
			klog.Errorf("Failed to add finalizer(%s) to MultiClusterService(%s/%s):%v ", util.MCSControllerFinalizer, mcs.Namespace, mcs.Name, err)
			return err
		}
	}

	// 3. Generate the MCS work in target clusters' namespace
	if err := c.propagateMultiClusterService(mcs, providerClusters); err != nil {
		return err
	}

	// 4. delete MultiClusterService work not in provider clusters and in the unready clusters
	if err := c.retrieveMultiClusterService(mcs, providerClusters); err != nil {
		return err
	}

	// 5. make sure service exist
	svc := &corev1.Service{}
	err = c.Client.Get(context.Background(), types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}, svc)
	// If the Serivice are deleted, the Service's ResourceBinding will be cleaned by GC
	if err != nil {
		klog.Errorf("Failed to get service(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return err
	}

	// 6. if service exists, create or update corresponding ResourceBinding
	if err := c.propagateService(context.Background(), mcs, svc, providerClusters, consumerClusters); err != nil {
		return err
	}

	klog.V(4).Infof("Success to reconcile MultiClusterService(%s/%s)", mcs.Namespace, mcs.Name)
	return nil
}

func (c *MCSController) propagateMultiClusterService(mcs *networkingv1alpha1.MultiClusterService, providerClusters sets.Set[string]) error {
	for clusterName := range providerClusters {
		clusterObj, err := util.GetCluster(c.Client, clusterName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				c.EventRecorder.Eventf(mcs, corev1.EventTypeWarning, events.EventReasonClusterNotFound, "Provider cluster %s is not found", clusterName)
				continue
			}
			klog.Errorf("Failed to get cluster %s, error is: %v", clusterName, err)
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
				util.PropagationInstruction:                            util.PropagationInstructionSuppressed,
				util.MultiClusterServiceNamespaceLabel:                 mcs.Namespace,
				util.MultiClusterServiceNameLabel:                      mcs.Name,
			},
		}

		mcsObj, err := helper.ToUnstructured(mcs)
		if err != nil {
			klog.Errorf("Failed to convert MultiClusterService(%s/%s) to unstructured object, err is %v", mcs.Namespace, mcs.Name, err)
			return err
		}
		if err = helper.CreateOrUpdateWork(c, workMeta, mcsObj); err != nil {
			klog.Errorf("Failed to create or update MultiClusterService(%s/%s) work in the given member cluster %s, err is %v",
				mcs.Namespace, mcs.Name, clusterName, err)
			return err
		}
	}

	return nil
}

func (c *MCSController) retrieveService(mcs *networkingv1alpha1.MultiClusterService) error {
	svc := &corev1.Service{}
	err := c.Client.Get(context.Background(), types.NamespacedName{Namespace: mcs.Namespace, Name: mcs.Name}, svc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Failed to get service(%s/%s):%v", mcs.Namespace, mcs.Name, err)
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

	if err = c.Client.Update(context.Background(), svcCopy); err != nil {
		klog.Errorf("Failed to update service(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return err
	}

	rb := &workv1alpha2.ResourceBinding{}
	err = c.Client.Get(context.Background(), types.NamespacedName{Namespace: mcs.Namespace, Name: names.GenerateBindingName(svc.Kind, svc.Name)}, rb)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("Failed to get ResourceBinding(%s/%s):%v", mcs.Namespace, names.GenerateBindingName(svc.Kind, svc.Name), err)
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
	if err := c.Client.Update(context.Background(), rbCopy); err != nil {
		klog.Errorf("Failed to update ResourceBinding(%s/%s):%v", mcs.Namespace, names.GenerateBindingName(svc.Kind, svc.Name), err)
		return err
	}

	return nil
}

func (c *MCSController) propagateService(ctx context.Context, mcs *networkingv1alpha1.MultiClusterService, svc *corev1.Service,
	providerClusters, consumerClusters sets.Set[string]) error {
	if err := c.claimMultiClusterServiceForService(svc, mcs); err != nil {
		klog.Errorf("Failed to claim for Service(%s/%s), err is %v", svc.Namespace, svc.Name, err)
		return err
	}

	binding, err := c.buildResourceBinding(svc, mcs, providerClusters, consumerClusters)
	if err != nil {
		klog.Errorf("Failed to build ResourceBinding for Service(%s/%s), err is %v", svc.Namespace, svc.Name, err)
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
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to create/update ResourceBinding(%s/%s):%v", bindingCopy.Namespace, bindingCopy.Name, err)
		return err
	}

	if operationResult == controllerutil.OperationResultCreated {
		klog.Infof("Create ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else if operationResult == controllerutil.OperationResultUpdated {
		klog.Infof("Update ResourceBinding(%s/%s) successfully.", binding.GetNamespace(), binding.GetName())
	} else {
		klog.V(2).Infof("ResourceBinding(%s/%s) is up to date.", binding.GetNamespace(), binding.GetName())
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

func (c *MCSController) claimMultiClusterServiceForService(svc *corev1.Service, mcs *networkingv1alpha1.MultiClusterService) error {
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

	if err := c.Client.Update(context.Background(), svcCopy); err != nil {
		klog.Errorf("Failed to update service(%s/%s):%v ", svc.Namespace, svc.Name, err)
		return err
	}

	return nil
}

func (c *MCSController) updateMultiClusterServiceStatus(mcs *networkingv1alpha1.MultiClusterService, status metav1.ConditionStatus, reason, message string) error {
	serviceAppliedCondition := metav1.Condition{
		Type:               networkingv1alpha1.MCSServiceAppliedConditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() (err error) {
		_, err = helper.UpdateStatus(context.Background(), c.Client, mcs, func() error {
			meta.SetStatusCondition(&mcs.Status.Conditions, serviceAppliedCondition)
			return nil
		})
		return err
	})
}

// IsClusterReady checks whether the cluster is ready.
func (c *MCSController) IsClusterReady(clusterName string) bool {
	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(context.TODO(), types.NamespacedName{Name: clusterName}, cluster); err != nil {
		klog.Errorf("Failed to get cluster object(%s):%v", clusterName, err)
		return false
	}

	return util.IsClusterReady(&cluster.Status)
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
		For(&networkingv1alpha1.MultiClusterService{}, builder.WithPredicates(mcsPredicateFunc)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(svcMapFunc), builder.WithPredicates(svcPredicateFunc)).
		Watches(&clusterv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(c.clusterMapFunc())).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RateLimiterOptions)}).
		Complete(c)
}

func (c *MCSController) serviceHasCrossClusterMultiClusterService(svc *corev1.Service) bool {
	mcs := &networkingv1alpha1.MultiClusterService{}
	if err := c.Client.Get(context.Background(),
		types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, mcs); err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get MultiClusterService(%s/%s):%v", svc.Namespace, svc.Name, err)
		}
		return false
	}

	return helper.MultiClusterServiceCrossClusterEnabled(mcs)
}

func (c *MCSController) clusterMapFunc() handler.MapFunc {
	return func(_ context.Context, a client.Object) []reconcile.Request {
		var clusterName string
		switch t := a.(type) {
		case *clusterv1alpha1.Cluster:
			clusterName = t.Name
		default:
			return nil
		}

		klog.V(4).Infof("Begin to sync mcs with cluster %s.", clusterName)
		mcsList := &networkingv1alpha1.MultiClusterServiceList{}
		if err := c.Client.List(context.TODO(), mcsList, &client.ListOptions{}); err != nil {
			klog.Errorf("Failed to list MultiClusterService, error: %v", err)
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
		klog.Errorf("Failed to get provider clusters by MultiClusterService(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return false, err
	}
	if providerClusters.Has(clusterName) {
		return true, nil
	}

	consumerClusters, err := helper.GetConsumerClusters(c.Client, mcs)
	if err != nil {
		klog.Errorf("Failed to get consumer clusters by MultiClusterService(%s/%s):%v", mcs.Namespace, mcs.Name, err)
		return false, err
	}
	if consumerClusters.Has(clusterName) {
		return true, nil
	}
	return false, nil
}
