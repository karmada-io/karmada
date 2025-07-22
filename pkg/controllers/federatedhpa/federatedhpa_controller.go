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

package federatedhpa

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	listcorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/federatedhpa/monitor"
	"github.com/karmada-io/karmada/pkg/metrics"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/typedmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/lifted/selectors"
)

// FederatedHPA-controller is borrowed from the HPA controller of Kubernetes.
// The referenced code has been marked in the comment.

// ControllerName is the controller name that will be used when reporting events and metrics.
const ControllerName = "federatedHPA-controller"

var (
	podGVR              = corev1.SchemeGroupVersion.WithResource("pods")
	scaleUpLimitFactor  = 2.0
	scaleUpLimitMinimum = 4.0
)

var (
	// errSpec is used to determine if the error comes from the spec of HPA object in reconcileAutoscaler.
	// All such errors should have this error as a root error so that the upstream function can distinguish spec errors from internal errors.
	// e.g., fmt.Errorf("invalid spec%w", errSpec)
	errSpec = errors.New("")
)

// FHPAController is to sync FederatedHPA.
type FHPAController struct {
	client.Client
	ReplicaCalc               *ReplicaCalculator
	ClusterScaleClientSetFunc func(string, client.Client) (*util.ClusterScaleClient, error)
	RESTMapper                meta.RESTMapper
	EventRecorder             record.EventRecorder
	TypedInformerManager      typedmanager.MultiClusterInformerManager
	ClusterCacheSyncTimeout   metav1.Duration

	monitor monitor.Monitor

	HorizontalPodAutoscalerSyncPeriod time.Duration
	DownscaleStabilisationWindow      time.Duration
	// Latest unstabilized recommendations for each autoscaler.
	recommendations     map[string][]timestampedRecommendation
	recommendationsLock sync.Mutex

	// Latest autoscaler events
	scaleUpEvents       map[string][]timestampedScaleEvent
	scaleUpEventsLock   sync.RWMutex
	scaleDownEvents     map[string][]timestampedScaleEvent
	scaleDownEventsLock sync.RWMutex

	// Storage of HPAs and their selectors.
	hpaSelectors    *selectors.BiMultimap
	hpaSelectorsMux sync.Mutex

	RateLimiterOptions ratelimiterflag.Options
}

type timestampedScaleEvent struct {
	replicaChange int32 // positive for scaleUp, negative for scaleDown
	timestamp     time.Time
	outdated      bool
}

type timestampedRecommendation struct {
	recommendation int32
	timestamp      time.Time
}

// SetupWithManager creates a controller and register to controller manager.
func (c *FHPAController) SetupWithManager(mgr controllerruntime.Manager) error {
	c.recommendations = map[string][]timestampedRecommendation{}
	c.scaleUpEvents = map[string][]timestampedScaleEvent{}
	c.scaleDownEvents = map[string][]timestampedScaleEvent{}
	c.hpaSelectors = selectors.NewBiMultimap()
	c.monitor = monitor.New()
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&autoscalingv1alpha1.FederatedHPA{}).
		WithOptions(controller.Options{RateLimiter: ratelimiterflag.DefaultControllerRateLimiter[controllerruntime.Request](c.RateLimiterOptions)}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(c)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *FHPAController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).InfoS("Reconciling FederatedHPA", "namespacedName", req.NamespacedName.String())

	hpa := &autoscalingv1alpha1.FederatedHPA{}
	key := req.NamespacedName.String()
	if err := c.Client.Get(ctx, req.NamespacedName, hpa); err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("FederatedHPA has been deleted in namespace", "hpaName", req.Name, "namespace", req.Namespace)
			c.recommendationsLock.Lock()
			delete(c.recommendations, key)
			c.recommendationsLock.Unlock()

			c.scaleUpEventsLock.Lock()
			delete(c.scaleUpEvents, key)
			c.scaleUpEventsLock.Unlock()

			c.scaleDownEventsLock.Lock()
			delete(c.scaleDownEvents, key)
			c.scaleDownEventsLock.Unlock()

			c.hpaSelectorsMux.Lock()
			c.hpaSelectors.DeleteSelector(selectors.Parse(key))
			c.hpaSelectorsMux.Unlock()
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{}, err
	}

	c.hpaSelectorsMux.Lock()
	if hpaKey := selectors.Parse(key); !c.hpaSelectors.SelectorExists(hpaKey) {
		c.hpaSelectors.PutSelector(hpaKey, labels.Nothing())
	}
	c.hpaSelectorsMux.Unlock()

	// observe process FederatedHPA latency
	var err error
	startTime := time.Now()
	defer func() {
		metrics.ObserveProcessFederatedHPALatency(err, startTime)
	}()

	err = c.reconcileAutoscaler(ctx, hpa)
	if err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{RequeueAfter: c.HorizontalPodAutoscalerSyncPeriod}, nil
}

//nolint:gocyclo
func (c *FHPAController) reconcileAutoscaler(ctx context.Context, hpa *autoscalingv1alpha1.FederatedHPA) (retErr error) {
	// actionLabel is used to report which actions this reconciliation has taken.
	actionLabel := monitor.ActionLabelNone
	start := time.Now()
	defer func() {
		errorLabel := monitor.ErrorLabelNone
		if retErr != nil {
			// In case of error, set "internal" as default.
			errorLabel = monitor.ErrorLabelInternal
		}
		if errors.Is(retErr, errSpec) {
			errorLabel = monitor.ErrorLabelSpec
		}

		c.monitor.ObserveReconciliationResult(actionLabel, errorLabel, time.Since(start))
	}()

	hpaStatusOriginal := hpa.Status.DeepCopy()

	reference := fmt.Sprintf("%s/%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)

	targetGV, err := schema.ParseGroupVersion(hpa.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetScale", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return fmt.Errorf("invalid API version in scale target reference: %v", err)
	}

	// Get the matched binding from resource template
	targetGVK := schema.GroupVersionKind{
		Group:   targetGV.Group,
		Kind:    hpa.Spec.ScaleTargetRef.Kind,
		Version: targetGV.Version,
	}
	targetResource := &unstructured.Unstructured{}
	targetResource.SetGroupVersionKind(targetGVK)
	err = c.Get(ctx, types.NamespacedName{Name: hpa.Spec.ScaleTargetRef.Name, Namespace: hpa.Namespace}, targetResource)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetScaleTargetRef", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetScaleTargetRef", "the HPA controller was unable to get the target reference object: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return fmt.Errorf("failed to get scale target reference: %v ", err)
	}

	binding, err := c.getBindingByLabel(ctx, targetResource.GetLabels(), hpa.Spec.ScaleTargetRef)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetBindings", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetBinding", "the HPA controller was unable to get the binding by scaleTargetRef: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return err
	}

	allClusters, err := c.getTargetCluster(ctx, binding)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetTargetClusters", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetTargetClusters", "the HPA controller was unable to get the target clusters from binding: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return err
	}

	mapping, err := c.RESTMapper.RESTMapping(targetGVK.GroupKind(), targetGVK.Version)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetScale", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return fmt.Errorf("unable to determine resource for scale target reference: %v", err)
	}

	scale, podList, err := c.scaleForTargetCluster(ctx, allClusters, hpa, mapping)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedGetScale", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
			utilruntime.HandleError(err)
		}
		return fmt.Errorf("failed to query scale subresource for %s: %v", reference, err)
	}

	templateScaleObj := &unstructured.Unstructured{}
	err = c.Client.SubResource("scale").Get(ctx, targetResource, templateScaleObj)
	if err != nil {
		return fmt.Errorf("failed to get scale subresource for resource template %s: %v", reference, err)
	}

	templateScale := &autoscalingv1.Scale{}
	err = helper.ConvertToTypedObject(templateScaleObj, templateScale)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured.Unstructured to scale: %v", err)
	}

	setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, "SucceededGetScale", "the HPA controller was able to get the target's current scale")
	currentReplicas := templateScale.Spec.Replicas

	key := types.NamespacedName{Namespace: hpa.Namespace, Name: hpa.Name}.String()
	c.recordInitialRecommendation(currentReplicas, key)

	var (
		metricStatuses        []autoscalingv2.MetricStatus
		metricDesiredReplicas int32
		metricName            string
	)

	desiredReplicas := int32(0)
	rescaleReason := ""

	var minReplicas int32

	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	} else {
		// Default value
		minReplicas = 1
	}

	rescale := true
	if scale.Spec.Replicas == 0 && minReplicas != 0 {
		// Autoscaling is disabled for this resource
		desiredReplicas = 0
		rescale = false
		setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionFalse, "ScalingDisabled", "scaling is disabled since the replica count of the target is zero")
	} else if currentReplicas > hpa.Spec.MaxReplicas {
		rescaleReason = "Current number of replicas above Spec.MaxReplicas"
		desiredReplicas = hpa.Spec.MaxReplicas
	} else if currentReplicas < minReplicas {
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = minReplicas
	} else {
		var metricTimestamp time.Time
		metricDesiredReplicas, metricName, metricStatuses, metricTimestamp, err = c.computeReplicasForMetrics(ctx, hpa, scale, hpa.Spec.Metrics, templateScale.Spec.Replicas, podList)
		// computeReplicasForMetrics may return both non-zero metricDesiredReplicas and an error.
		// That means some metrics still work and HPA should perform scaling based on them.
		if err != nil && metricDesiredReplicas == -1 {
			c.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedComputeMetricsReplicas", err.Error())
			return fmt.Errorf("failed to compute desired number of replicas based on listed metrics for %s: %v", reference, err)
		}
		if err != nil {
			// We proceed to scaling, but return this error from reconcileAutoscaler() finally.
			retErr = err
		}

		klog.V(4).InfoS("proposing desired replicas for resource", "desiredReplicas", metricDesiredReplicas, "metricName", metricName, "metricTimestamp", metricTimestamp, "resource", reference)

		rescaleMetric := ""
		if metricDesiredReplicas > desiredReplicas {
			desiredReplicas = metricDesiredReplicas
			rescaleMetric = metricName
		}
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}
		if hpa.Spec.Behavior == nil {
			desiredReplicas = c.normalizeDesiredReplicas(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		} else {
			desiredReplicas = c.normalizeDesiredReplicasWithBehaviors(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		}
		rescale = desiredReplicas != currentReplicas
	}

	if rescale {
		if err = helper.ApplyReplica(templateScaleObj, int64(desiredReplicas), util.ReplicasField); err != nil {
			return err
		}
		err = c.Client.SubResource("scale").Update(ctx, targetResource, client.WithSubResourceBody(templateScaleObj))
		if err != nil {
			c.EventRecorder.Eventf(hpa, corev1.EventTypeWarning, "FailedRescale", "New size: %d; reason: %s; error: %v", desiredReplicas, rescaleReason, err.Error())
			setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionFalse, "FailedUpdateScale", "the HPA controller was unable to update the target scale: %v", err)
			c.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			return fmt.Errorf("failed to rescale %s: %v", reference, err)
		}
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, "SucceededRescale", "the HPA controller was able to update the target scale to %d", desiredReplicas)
		c.EventRecorder.Eventf(hpa, corev1.EventTypeNormal, "SuccessfulRescale", "New size: %d; reason: %s", desiredReplicas, rescaleReason)
		c.storeScaleEvent(hpa.Spec.Behavior, key, currentReplicas, desiredReplicas)
		klog.InfoS("Successfully rescaled FederatedHPA",
			"hpaName", hpa.Name, "currentReplicas", currentReplicas, "desiredReplicas", desiredReplicas, "rescaleReason", rescaleReason)

		if desiredReplicas > currentReplicas {
			actionLabel = monitor.ActionLabelScaleUp
		} else {
			actionLabel = monitor.ActionLabelScaleDown
		}
	} else {
		klog.V(4).InfoS("decided not to scale resource", "resource", reference, "desiredReplicas", desiredReplicas, "hpaLastScaleTime", hpa.Status.LastScaleTime)
		desiredReplicas = currentReplicas
	}

	c.setStatus(hpa, currentReplicas, desiredReplicas, metricStatuses, rescale)
	err = c.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa)
	if err != nil {
		// we can overwrite retErr in this case because it's an internal error.
		return err
	}

	return retErr
}

func (c *FHPAController) getBindingByLabel(ctx context.Context, resourceLabel map[string]string, resourceRef autoscalingv2.CrossVersionObjectReference) (*workv1alpha2.ResourceBinding, error) {
	if len(resourceLabel) == 0 {
		return nil, errors.New("target resource has no label")
	}

	var selector labels.Selector
	if policyID, ok := resourceLabel[policyv1alpha1.PropagationPolicyPermanentIDLabel]; ok {
		selector = labels.SelectorFromSet(labels.Set{
			policyv1alpha1.PropagationPolicyPermanentIDLabel: policyID,
		})
	} else if policyID, ok = resourceLabel[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]; ok {
		selector = labels.SelectorFromSet(labels.Set{
			policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: policyID,
		})
	} else {
		return nil, errors.New("no label of policy permanent-id found")
	}

	binding := &workv1alpha2.ResourceBinding{}
	bindingList := &workv1alpha2.ResourceBindingList{}
	err := c.Client.List(ctx, bindingList, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(bindingList.Items) == 0 {
		return nil, errors.New("length of binding list is zero")
	}

	found := false
	for i, b := range bindingList.Items {
		if b.Spec.Resource.Name == resourceRef.Name && b.Spec.Resource.APIVersion == resourceRef.APIVersion && b.Spec.Resource.Kind == resourceRef.Kind {
			found = true
			binding = &bindingList.Items[i]
			break
		}
	}
	if !found {
		return nil, errors.New("no binding matches the target resource")
	}

	return binding, nil
}

func (c *FHPAController) getTargetCluster(ctx context.Context, binding *workv1alpha2.ResourceBinding) ([]string, error) {
	if len(binding.Spec.Clusters) == 0 {
		return nil, errors.New("binding has no schedulable clusters")
	}

	var allClusters []string
	cluster := &clusterv1alpha1.Cluster{}
	for _, targetCluster := range binding.Spec.Clusters {
		err := c.Client.Get(ctx, types.NamespacedName{Name: targetCluster.Name}, cluster)
		if err != nil {
			return nil, err
		}
		if util.IsClusterReady(&cluster.Status) {
			allClusters = append(allClusters, targetCluster.Name)
		}
	}

	return allClusters, nil
}

func (c *FHPAController) scaleForTargetCluster(ctx context.Context, clusters []string, hpa *autoscalingv1alpha1.FederatedHPA, mapping *meta.RESTMapping) (*autoscalingv1.Scale, []*corev1.Pod, error) {
	multiClusterScale := &autoscalingv1.Scale{
		Spec: autoscalingv1.ScaleSpec{
			Replicas: 0,
		},
		Status: autoscalingv1.ScaleStatus{
			Replicas: 0,
		},
	}

	var multiClusterPodList []*corev1.Pod

	targetGR := mapping.Resource.GroupResource()
	for _, cluster := range clusters {
		clusterClient, err := c.ClusterScaleClientSetFunc(cluster, c.Client)
		if err != nil {
			klog.ErrorS(err, "Failed to get cluster client of cluster", "cluster", cluster)
			continue
		}

		clusterInformerManager, err := c.buildPodInformerForCluster(clusterClient)
		if err != nil {
			klog.ErrorS(err, "Failed to get or create informer for cluster", "cluster", cluster)
			continue
		}

		scale, err := clusterClient.ScaleClient.Scales(hpa.Namespace).Get(ctx, targetGR, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to get scale subResource of resource in cluster", "resource", hpa.Spec.ScaleTargetRef.Name, "cluster", cluster)
			continue
		}

		multiClusterScale.Spec.Replicas += scale.Spec.Replicas
		multiClusterScale.Status.Replicas += scale.Status.Replicas
		if multiClusterScale.Status.Selector == "" {
			multiClusterScale.Status.Selector = scale.Status.Selector
		}

		if scale.Status.Selector == "" {
			errMsg := "selector is required"
			c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "SelectorRequired", errMsg)
			setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionFalse, "InvalidSelector", "the HPA target's scale is missing a selector")
			continue
		}

		selector, err := labels.Parse(scale.Status.Selector)
		if err != nil {
			errMsg := fmt.Sprintf("couldn't convert selector into a corresponding internal selector object: %v", err)
			c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "InvalidSelector", errMsg)
			setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionFalse, "InvalidSelector", "%s", errMsg)
			continue
		}

		podInterface, err := clusterInformerManager.Lister(podGVR)
		if err != nil {
			klog.ErrorS(err, "Failed to get podInterface for cluster", "cluster", cluster)
			continue
		}

		podLister, ok := podInterface.(listcorev1.PodLister)
		if !ok {
			klog.ErrorS(nil, "Failed to convert interface to PodLister for cluster", "cluster", cluster)
			continue
		}

		podList, err := podLister.Pods(hpa.Namespace).List(selector)
		if err != nil {
			klog.ErrorS(err, "Failed to get podList for cluster", "cluster", cluster)
			continue
		}

		multiClusterPodList = append(multiClusterPodList, podList...)
	}

	if multiClusterScale.Spec.Replicas == 0 {
		return nil, nil, fmt.Errorf("failed to get replicas in any of clusters, possibly because all of clusters are not ready")
	}

	return multiClusterScale, multiClusterPodList, nil
}

// buildPodInformerForCluster builds informer manager for cluster if it doesn't exist, then constructs informers for pod and start it. If the informer manager exist, return it.
func (c *FHPAController) buildPodInformerForCluster(clusterScaleClient *util.ClusterScaleClient) (typedmanager.SingleClusterInformerManager, error) {
	singleClusterInformerManager := c.TypedInformerManager.GetSingleClusterManager(clusterScaleClient.ClusterName)
	if singleClusterInformerManager == nil {
		singleClusterInformerManager = c.TypedInformerManager.ForCluster(clusterScaleClient.ClusterName, clusterScaleClient.KubeClient, 0)
	}

	if singleClusterInformerManager.IsInformerSynced(podGVR) {
		return singleClusterInformerManager, nil
	}

	if _, err := singleClusterInformerManager.Lister(podGVR); err != nil {
		klog.ErrorS(err, "Failed to get the lister for pods")
	}

	c.TypedInformerManager.Start(clusterScaleClient.ClusterName)

	if err := func() error {
		synced := c.TypedInformerManager.WaitForCacheSyncWithTimeout(clusterScaleClient.ClusterName, c.ClusterCacheSyncTimeout.Duration)
		if synced == nil {
			return fmt.Errorf("no informerFactory for cluster %s exist", clusterScaleClient.ClusterName)
		}
		if !synced[podGVR] {
			return fmt.Errorf("informer for pods hasn't synced")
		}
		return nil
	}(); err != nil {
		klog.ErrorS(err, "Failed to sync cache for cluster", "cluster", clusterScaleClient.ClusterName)
		c.TypedInformerManager.Stop(clusterScaleClient.ClusterName)
		return nil, err
	}

	return singleClusterInformerManager, nil
}

// L565-L1410 is partly lifted from https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/controller/podautoscaler/horizontal.go.
// computeReplicasForMetrics computes the desired number of replicas for the metric specifications listed in the HPA,
// returning the maximum of the computed replica counts, a description of the associated metric, and the statuses of
// all metrics computed.
// It may return both valid metricDesiredReplicas and an error,
// when some metrics still work and HPA should perform scaling based on them.
// If HPA cannot do anything due to error, it returns -1 in metricDesiredReplicas as a failure signal.
func (c *FHPAController) computeReplicasForMetrics(ctx context.Context, hpa *autoscalingv1alpha1.FederatedHPA, scale *autoscalingv1.Scale,
	metricSpecs []autoscalingv2.MetricSpec, templateReplicas int32, podList []*corev1.Pod) (replicas int32, metric string, statuses []autoscalingv2.MetricStatus, timestamp time.Time, err error) {
	selector, err := c.validateAndParseSelector(hpa, scale.Status.Selector, podList)
	if err != nil {
		return -1, "", nil, time.Time{}, err
	}

	specReplicas := templateReplicas
	statusReplicas := scale.Status.Replicas
	calibration := float64(scale.Spec.Replicas) / float64(specReplicas)
	statuses = make([]autoscalingv2.MetricStatus, len(metricSpecs))

	invalidMetricsCount := 0
	var invalidMetricError error
	var invalidMetricCondition autoscalingv2.HorizontalPodAutoscalerCondition

	for i, metricSpec := range metricSpecs {
		replicaCountProposal, metricNameProposal, timestampProposal, condition, err := c.computeReplicasForMetric(ctx, hpa, metricSpec, specReplicas, statusReplicas, selector, &statuses[i], podList, calibration)

		if err != nil {
			if invalidMetricsCount <= 0 {
				invalidMetricCondition = condition
				invalidMetricError = err
			}
			invalidMetricsCount++
			continue
		}
		if replicas == 0 || replicaCountProposal > replicas {
			timestamp = timestampProposal
			replicas = replicaCountProposal
			metric = metricNameProposal
		}
	}

	if invalidMetricError != nil {
		invalidMetricError = fmt.Errorf("invalid metrics (%v invalid out of %v), first error is: %v", invalidMetricsCount, len(metricSpecs), invalidMetricError)
	}

	// If all metrics are invalid or some are invalid and we would scale down,
	// return an error and set the condition of the hpa based on the first invalid metric.
	// Otherwise set the condition as scaling active as we're going to scale
	if invalidMetricsCount >= len(metricSpecs) || (invalidMetricsCount > 0 && replicas < specReplicas) {
		setCondition(hpa, invalidMetricCondition.Type, invalidMetricCondition.Status, invalidMetricCondition.Reason, "%s", invalidMetricCondition.Message)
		return -1, "", statuses, time.Time{}, invalidMetricError
	}
	setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionTrue, "ValidMetricFound", "the HPA was able to successfully calculate a replica count from %s", metric)
	return replicas, metric, statuses, timestamp, invalidMetricError
}

// hpasControllingPodsUnderSelector returns a list of keys of all HPAs that control a given list of pods.
func (c *FHPAController) hpasControllingPodsUnderSelector(pods []*corev1.Pod) []selectors.Key {
	c.hpaSelectorsMux.Lock()
	defer c.hpaSelectorsMux.Unlock()

	hpas := map[selectors.Key]struct{}{}
	for _, p := range pods {
		podKey := selectors.Key{Name: p.Name, Namespace: p.Namespace}
		c.hpaSelectors.Put(podKey, p.Labels)

		selectingHpas, ok := c.hpaSelectors.ReverseSelect(podKey)
		if !ok {
			continue
		}
		for _, hpa := range selectingHpas {
			hpas[hpa] = struct{}{}
		}
	}
	// Clean up all added pods.
	c.hpaSelectors.KeepOnly([]selectors.Key{})

	hpaList := []selectors.Key{}
	for hpa := range hpas {
		hpaList = append(hpaList, hpa)
	}
	return hpaList
}

// validateAndParseSelector verifies that:
// - selector format is valid;
// - all pods by current selector are controlled by only one HPA.
// Returns an error if the check has failed or the parsed selector if succeeded.
// In case of an error the ScalingActive is set to false with the corresponding reason.
func (c *FHPAController) validateAndParseSelector(hpa *autoscalingv1alpha1.FederatedHPA, selector string, podList []*corev1.Pod) (labels.Selector, error) {
	parsedSelector, err := labels.Parse(selector)
	if err != nil {
		errMsg := fmt.Sprintf("couldn't convert selector into a corresponding internal selector object: %v", err)
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "InvalidSelector", errMsg)
		setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionFalse, "InvalidSelector", "%s", errMsg)
		return nil, errors.New(errMsg)
	}

	hpaKey := selectors.Key{Name: hpa.Name, Namespace: hpa.Namespace}
	c.hpaSelectorsMux.Lock()
	if c.hpaSelectors.SelectorExists(hpaKey) {
		c.hpaSelectors.PutSelector(hpaKey, parsedSelector)
	}
	c.hpaSelectorsMux.Unlock()

	selectingHpas := c.hpasControllingPodsUnderSelector(podList)
	if len(selectingHpas) > 1 {
		errMsg := fmt.Sprintf("pods by selector %v are controlled by multiple HPAs: %v", selector, selectingHpas)
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "AmbiguousSelector", errMsg)
		setCondition(hpa, autoscalingv2.ScalingActive, corev1.ConditionFalse, "AmbiguousSelector", "%s", errMsg)
		return nil, errors.New(errMsg)
	}

	return parsedSelector, nil
}

// Computes the desired number of replicas for a specific hpa and metric specification,
// returning the metric status and a proposed condition to be set on the HPA object.
//
//nolint:gocyclo
func (c *FHPAController) computeReplicasForMetric(ctx context.Context, hpa *autoscalingv1alpha1.FederatedHPA, spec autoscalingv2.MetricSpec,
	specReplicas, statusReplicas int32, selector labels.Selector, status *autoscalingv2.MetricStatus, podList []*corev1.Pod, calibration float64) (replicaCountProposal int32, metricNameProposal string,
	timestampProposal time.Time, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	// actionLabel is used to report which actions this reconciliation has taken.
	start := time.Now()
	defer func() {
		actionLabel := monitor.ActionLabelNone
		switch {
		case replicaCountProposal > hpa.Status.CurrentReplicas:
			actionLabel = monitor.ActionLabelScaleUp
		case replicaCountProposal < hpa.Status.CurrentReplicas:
			actionLabel = monitor.ActionLabelScaleDown
		}

		errorLabel := monitor.ErrorLabelNone
		if err != nil {
			// In case of error, set "internal" as default.
			errorLabel = monitor.ErrorLabelInternal
			actionLabel = monitor.ActionLabelNone
		}
		if errors.Is(err, errSpec) {
			errorLabel = monitor.ErrorLabelSpec
		}

		c.monitor.ObserveMetricComputationResult(actionLabel, errorLabel, time.Since(start), spec.Type)
	}()

	switch spec.Type {
	case autoscalingv2.ObjectMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Object.Metric.Selector)
		if err != nil {
			condition := c.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = c.computeStatusForObjectMetric(specReplicas, statusReplicas, spec, hpa, status, metricSelector, podList, calibration)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
	case autoscalingv2.PodsMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Pods.Metric.Selector)
		if err != nil {
			condition := c.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = c.computeStatusForPodsMetric(specReplicas, spec, hpa, selector, status, metricSelector, podList, calibration)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
	case autoscalingv2.ResourceMetricSourceType:
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = c.computeStatusForResourceMetric(ctx, specReplicas, spec, hpa, selector, status, podList, calibration)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get %s resource metric value: %v", spec.Resource.Name, err)
		}
	case autoscalingv2.ContainerResourceMetricSourceType:
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = c.computeStatusForContainerResourceMetric(ctx, specReplicas, spec, hpa, selector, status, podList, calibration)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get %s container metric value: %v", spec.ContainerResource.Container, err)
		}
	default:
		// It shouldn't reach here as invalid metric source type is filtered out in the api-server's validation.
		err = fmt.Errorf("unknown metric source type %q%w", string(spec.Type), errSpec)
		condition := c.getUnableComputeReplicaCountCondition(hpa, "InvalidMetricSourceType", err)
		return 0, "", time.Time{}, condition, err
	}
	return replicaCountProposal, metricNameProposal, timestampProposal, autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
}

// computeStatusForObjectMetric computes the desired number of replicas for the specified metric of type ObjectMetricSourceType.
func (c *FHPAController) computeStatusForObjectMetric(specReplicas, statusReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *autoscalingv1alpha1.FederatedHPA, status *autoscalingv2.MetricStatus, metricSelector labels.Selector, podList []*corev1.Pod, calibration float64) (replicas int32, timestamp time.Time, metricName string, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	switch metricSpec.Object.Target.Type {
	case autoscalingv2.ValueMetricType:
		replicaCountProposal, usageProposal, timestampProposal, err := c.ReplicaCalc.GetObjectMetricReplicas(specReplicas, metricSpec.Object.Target.Value.MilliValue(), metricSpec.Object.Metric.Name, hpa.Namespace, &metricSpec.Object.DescribedObject, metricSelector, podList, calibration)
		if err != nil {
			condition := c.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, timestampProposal, "", condition, err
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ObjectMetricSourceType,
			Object: &autoscalingv2.ObjectMetricStatus{
				DescribedObject: metricSpec.Object.DescribedObject,
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.Object.Metric.Name,
					Selector: metricSpec.Object.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					Value: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("%s metric %s", metricSpec.Object.DescribedObject.Kind, metricSpec.Object.Metric.Name), autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
	case autoscalingv2.AverageValueMetricType:
		replicaCountProposal, usageProposal, timestampProposal, err := c.ReplicaCalc.GetObjectPerPodMetricReplicas(statusReplicas, metricSpec.Object.Target.AverageValue.MilliValue(), metricSpec.Object.Metric.Name, hpa.Namespace, &metricSpec.Object.DescribedObject, metricSelector, calibration)
		if err != nil {
			condition := c.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, time.Time{}, "", condition, fmt.Errorf("failed to get %s object metric: %v", metricSpec.Object.Metric.Name, err)
		}
		*status = autoscalingv2.MetricStatus{
			Type: autoscalingv2.ObjectMetricSourceType,
			Object: &autoscalingv2.ObjectMetricStatus{
				Metric: autoscalingv2.MetricIdentifier{
					Name:     metricSpec.Object.Metric.Name,
					Selector: metricSpec.Object.Metric.Selector,
				},
				Current: autoscalingv2.MetricValueStatus{
					AverageValue: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
				},
			},
		}
		return replicaCountProposal, timestampProposal, fmt.Sprintf("external metric %s(%+v)", metricSpec.Object.Metric.Name, metricSpec.Object.Metric.Selector), autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
	}
	errMsg := "invalid object metric source: neither a value target nor an average value target was set"
	err = errors.New(errMsg)
	condition = c.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
	return 0, time.Time{}, "", condition, err
}

// computeStatusForPodsMetric computes the desired number of replicas for the specified metric of type PodsMetricSourceType.
func (c *FHPAController) computeStatusForPodsMetric(currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *autoscalingv1alpha1.FederatedHPA, selector labels.Selector, status *autoscalingv2.MetricStatus, metricSelector labels.Selector, podList []*corev1.Pod, calibration float64) (replicaCountProposal int32, timestampProposal time.Time, metricNameProposal string, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	replicaCountProposal, usageProposal, timestampProposal, err := c.ReplicaCalc.GetMetricReplicas(currentReplicas, metricSpec.Pods.Target.AverageValue.MilliValue(), metricSpec.Pods.Metric.Name, hpa.Namespace, selector, metricSelector, podList, calibration)
	if err != nil {
		condition = c.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
		return 0, timestampProposal, "", condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.PodsMetricSourceType,
		Pods: &autoscalingv2.PodsMetricStatus{
			Metric: autoscalingv2.MetricIdentifier{
				Name:     metricSpec.Pods.Metric.Name,
				Selector: metricSpec.Pods.Metric.Selector,
			},
			Current: autoscalingv2.MetricValueStatus{
				AverageValue: resource.NewMilliQuantity(usageProposal, resource.DecimalSI),
			},
		},
	}

	return replicaCountProposal, timestampProposal, fmt.Sprintf("pods metric %s", metricSpec.Pods.Metric.Name), autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
}

func (c *FHPAController) computeStatusForResourceMetricGeneric(ctx context.Context, currentReplicas int32, target autoscalingv2.MetricTarget,
	resourceName corev1.ResourceName, namespace string, container string, selector labels.Selector, sourceType autoscalingv2.MetricSourceType, podList []*corev1.Pod, calibration float64) (replicaCountProposal int32,
	metricStatus *autoscalingv2.MetricValueStatus, timestampProposal time.Time, metricNameProposal string,
	condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	if target.AverageValue != nil {
		var rawProposal int64
		replicaCountProposal, rawProposal, timestampProposal, err := c.ReplicaCalc.GetRawResourceReplicas(ctx, currentReplicas, target.AverageValue.MilliValue(), resourceName, namespace, selector, container, podList, calibration)
		if err != nil {
			return 0, nil, time.Time{}, "", condition, fmt.Errorf("failed to get %s utilization: %v", resourceName, err)
		}
		metricNameProposal = fmt.Sprintf("%s resource", resourceName.String())
		status := autoscalingv2.MetricValueStatus{
			AverageValue: resource.NewMilliQuantity(rawProposal, resource.DecimalSI),
		}
		return replicaCountProposal, &status, timestampProposal, metricNameProposal, autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
	}

	if target.AverageUtilization == nil {
		errMsg := "invalid resource metric source: neither an average utilization target nor an average value target was set"
		return 0, nil, time.Time{}, "", condition, errors.New(errMsg)
	}

	targetUtilization := *target.AverageUtilization
	replicaCountProposal, percentageProposal, rawProposal, timestampProposal, err := c.ReplicaCalc.GetResourceReplicas(ctx, currentReplicas, targetUtilization, resourceName, namespace, selector, container, podList, calibration)
	if err != nil {
		return 0, nil, time.Time{}, "", condition, fmt.Errorf("failed to get %s utilization: %v", resourceName, err)
	}

	metricNameProposal = fmt.Sprintf("%s resource utilization (percentage of request)", resourceName)
	if sourceType == autoscalingv2.ContainerResourceMetricSourceType {
		metricNameProposal = fmt.Sprintf("%s container resource utilization (percentage of request)", resourceName)
	}
	status := autoscalingv2.MetricValueStatus{
		AverageUtilization: &percentageProposal,
		AverageValue:       resource.NewMilliQuantity(rawProposal, resource.DecimalSI),
	}
	return replicaCountProposal, &status, timestampProposal, metricNameProposal, autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
}

// computeStatusForResourceMetric computes the desired number of replicas for the specified metric of type ResourceMetricSourceType.
func (c *FHPAController) computeStatusForResourceMetric(ctx context.Context, currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *autoscalingv1alpha1.FederatedHPA,
	selector labels.Selector, status *autoscalingv2.MetricStatus, podList []*corev1.Pod, calibration float64) (replicaCountProposal int32, timestampProposal time.Time,
	metricNameProposal string, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	replicaCountProposal, metricValueStatus, timestampProposal, metricNameProposal, condition, err := c.computeStatusForResourceMetricGeneric(ctx, currentReplicas, metricSpec.Resource.Target, metricSpec.Resource.Name, hpa.Namespace, "", selector, autoscalingv2.ResourceMetricSourceType, podList, calibration)
	if err != nil {
		condition = c.getUnableComputeReplicaCountCondition(hpa, "FailedGetResourceMetric", err)
		return replicaCountProposal, timestampProposal, metricNameProposal, condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.ResourceMetricSourceType,
		Resource: &autoscalingv2.ResourceMetricStatus{
			Name:    metricSpec.Resource.Name,
			Current: *metricValueStatus,
		},
	}
	return replicaCountProposal, timestampProposal, metricNameProposal, condition, nil
}

// computeStatusForContainerResourceMetric computes the desired number of replicas for the specified metric of type ResourceMetricSourceType.
func (c *FHPAController) computeStatusForContainerResourceMetric(ctx context.Context, currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *autoscalingv1alpha1.FederatedHPA,
	selector labels.Selector, status *autoscalingv2.MetricStatus, podList []*corev1.Pod, calibration float64) (replicaCountProposal int32, timestampProposal time.Time,
	metricNameProposal string, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	replicaCountProposal, metricValueStatus, timestampProposal, metricNameProposal, condition, err := c.computeStatusForResourceMetricGeneric(ctx, currentReplicas, metricSpec.ContainerResource.Target, metricSpec.ContainerResource.Name, hpa.Namespace, metricSpec.ContainerResource.Container, selector, autoscalingv2.ContainerResourceMetricSourceType, podList, calibration)
	if err != nil {
		condition = c.getUnableComputeReplicaCountCondition(hpa, "FailedGetContainerResourceMetric", err)
		return replicaCountProposal, timestampProposal, metricNameProposal, condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.ContainerResourceMetricSourceType,
		ContainerResource: &autoscalingv2.ContainerResourceMetricStatus{
			Name:      metricSpec.ContainerResource.Name,
			Container: metricSpec.ContainerResource.Container,
			Current:   *metricValueStatus,
		},
	}
	return replicaCountProposal, timestampProposal, metricNameProposal, condition, nil
}

func (c *FHPAController) recordInitialRecommendation(currentReplicas int32, key string) {
	c.recommendationsLock.Lock()
	defer c.recommendationsLock.Unlock()
	if c.recommendations[key] == nil {
		c.recommendations[key] = []timestampedRecommendation{{currentReplicas, time.Now()}}
	}
}

// stabilizeRecommendation:
// - replaces old recommendation with the newest recommendation,
// - returns max of recommendations that are not older than DownscaleStabilisationWindow.
func (c *FHPAController) stabilizeRecommendation(key string, prenormalizedDesiredReplicas int32) int32 {
	maxRecommendation := prenormalizedDesiredReplicas
	foundOldSample := false
	oldSampleIndex := 0
	cutoff := time.Now().Add(-c.DownscaleStabilisationWindow)

	c.recommendationsLock.Lock()
	defer c.recommendationsLock.Unlock()
	for i, rec := range c.recommendations[key] {
		if rec.timestamp.Before(cutoff) {
			foundOldSample = true
			oldSampleIndex = i
		} else if rec.recommendation > maxRecommendation {
			maxRecommendation = rec.recommendation
		}
	}
	if foundOldSample {
		c.recommendations[key][oldSampleIndex] = timestampedRecommendation{prenormalizedDesiredReplicas, time.Now()}
	} else {
		c.recommendations[key] = append(c.recommendations[key], timestampedRecommendation{prenormalizedDesiredReplicas, time.Now()})
	}
	return maxRecommendation
}

// normalizeDesiredReplicas takes the metrics desired replicas value and normalizes it based on the appropriate conditions (i.e. < maxReplicas, >
// minReplicas, etc...)
func (c *FHPAController) normalizeDesiredReplicas(hpa *autoscalingv1alpha1.FederatedHPA, key string, currentReplicas int32, prenormalizedDesiredReplicas int32, minReplicas int32) int32 {
	stabilizedRecommendation := c.stabilizeRecommendation(key, prenormalizedDesiredReplicas)
	if stabilizedRecommendation != prenormalizedDesiredReplicas {
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, "ScaleDownStabilized", "recent recommendations were higher than current one, applying the highest recent recommendation")
	} else {
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, "ReadyForNewScale", "recommended size matches current size")
	}

	desiredReplicas, condition, reason := convertDesiredReplicasWithRules(currentReplicas, stabilizedRecommendation, minReplicas, hpa.Spec.MaxReplicas)

	if desiredReplicas == stabilizedRecommendation {
		setCondition(hpa, autoscalingv2.ScalingLimited, corev1.ConditionFalse, condition, "%s", reason)
	} else {
		setCondition(hpa, autoscalingv2.ScalingLimited, corev1.ConditionTrue, condition, "%s", reason)
	}

	return desiredReplicas
}

// NormalizationArg is used to pass all needed information between functions as one structure
type NormalizationArg struct {
	Key               string
	ScaleUpBehavior   *autoscalingv2.HPAScalingRules
	ScaleDownBehavior *autoscalingv2.HPAScalingRules
	MinReplicas       int32
	MaxReplicas       int32
	CurrentReplicas   int32
	DesiredReplicas   int32
}

// normalizeDesiredReplicasWithBehaviors takes the metrics desired replicas value and normalizes it:
// 1. Apply the basic conditions (i.e. < maxReplicas, > minReplicas, etc...)
// 2. Apply the scale up/down limits from the hpaSpec.Behaviors (i.e. add no more than 4 pods)
// 3. Apply the constraints period (i.e. add no more than 4 pods per minute)
// 4. Apply the stabilization (i.e. add no more than 4 pods per minute, and pick the smallest recommendation during last 5 minutes)
func (c *FHPAController) normalizeDesiredReplicasWithBehaviors(hpa *autoscalingv1alpha1.FederatedHPA, key string, currentReplicas, prenormalizedDesiredReplicas, minReplicas int32) int32 {
	c.maybeInitScaleDownStabilizationWindow(hpa)
	normalizationArg := NormalizationArg{
		Key:               key,
		ScaleUpBehavior:   hpa.Spec.Behavior.ScaleUp,
		ScaleDownBehavior: hpa.Spec.Behavior.ScaleDown,
		MinReplicas:       minReplicas,
		MaxReplicas:       hpa.Spec.MaxReplicas,
		CurrentReplicas:   currentReplicas,
		DesiredReplicas:   prenormalizedDesiredReplicas}
	stabilizedRecommendation, reason, message := c.stabilizeRecommendationWithBehaviors(normalizationArg)
	normalizationArg.DesiredReplicas = stabilizedRecommendation
	if stabilizedRecommendation != prenormalizedDesiredReplicas {
		// "ScaleUpStabilized" || "ScaleDownStabilized"
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, reason, "%s", message)
	} else {
		setCondition(hpa, autoscalingv2.AbleToScale, corev1.ConditionTrue, "ReadyForNewScale", "recommended size matches current size")
	}
	desiredReplicas, reason, message := c.convertDesiredReplicasWithBehaviorRate(normalizationArg)
	if desiredReplicas == stabilizedRecommendation {
		setCondition(hpa, autoscalingv2.ScalingLimited, corev1.ConditionFalse, reason, "%s", message)
	} else {
		setCondition(hpa, autoscalingv2.ScalingLimited, corev1.ConditionTrue, reason, "%s", message)
	}

	return desiredReplicas
}

func (c *FHPAController) maybeInitScaleDownStabilizationWindow(hpa *autoscalingv1alpha1.FederatedHPA) {
	behavior := hpa.Spec.Behavior
	if behavior != nil && behavior.ScaleDown != nil && behavior.ScaleDown.StabilizationWindowSeconds == nil {
		stabilizationWindowSeconds := (int32)(c.DownscaleStabilisationWindow.Seconds())
		hpa.Spec.Behavior.ScaleDown.StabilizationWindowSeconds = &stabilizationWindowSeconds
	}
}

// getReplicasChangePerPeriod function find all the replica changes per period
func getReplicasChangePerPeriod(periodSeconds int32, scaleEvents []timestampedScaleEvent) int32 {
	period := time.Second * time.Duration(periodSeconds)
	cutoff := time.Now().Add(-period)
	var replicas int32
	for _, rec := range scaleEvents {
		if rec.timestamp.After(cutoff) {
			replicas += rec.replicaChange
		}
	}
	return replicas
}

func (c *FHPAController) getUnableComputeReplicaCountCondition(hpa runtime.Object, reason string, err error) (condition autoscalingv2.HorizontalPodAutoscalerCondition) {
	c.EventRecorder.Event(hpa, corev1.EventTypeWarning, reason, err.Error())
	return autoscalingv2.HorizontalPodAutoscalerCondition{
		Type:    autoscalingv2.ScalingActive,
		Status:  corev1.ConditionFalse,
		Reason:  reason,
		Message: fmt.Sprintf("the HPA was unable to compute the replica count: %v", err),
	}
}

// storeScaleEvent stores (adds or replaces outdated) scale event.
// outdated events to be replaced were marked as outdated in the `markScaleEventsOutdated` function
func (c *FHPAController) storeScaleEvent(behavior *autoscalingv2.HorizontalPodAutoscalerBehavior, key string, prevReplicas, newReplicas int32) {
	if behavior == nil {
		return // we should not store any event as they will not be used
	}
	var oldSampleIndex int
	var longestPolicyPeriod int32
	foundOldSample := false
	if newReplicas > prevReplicas {
		longestPolicyPeriod = getLongestPolicyPeriod(behavior.ScaleUp)

		c.scaleUpEventsLock.Lock()
		defer c.scaleUpEventsLock.Unlock()
		markScaleEventsOutdated(c.scaleUpEvents[key], longestPolicyPeriod)
		replicaChange := newReplicas - prevReplicas
		for i, event := range c.scaleUpEvents[key] {
			if event.outdated {
				foundOldSample = true
				oldSampleIndex = i
			}
		}
		newEvent := timestampedScaleEvent{replicaChange, time.Now(), false}
		if foundOldSample {
			c.scaleUpEvents[key][oldSampleIndex] = newEvent
		} else {
			c.scaleUpEvents[key] = append(c.scaleUpEvents[key], newEvent)
		}
	} else {
		longestPolicyPeriod = getLongestPolicyPeriod(behavior.ScaleDown)

		c.scaleDownEventsLock.Lock()
		defer c.scaleDownEventsLock.Unlock()
		markScaleEventsOutdated(c.scaleDownEvents[key], longestPolicyPeriod)
		replicaChange := prevReplicas - newReplicas
		for i, event := range c.scaleDownEvents[key] {
			if event.outdated {
				foundOldSample = true
				oldSampleIndex = i
			}
		}
		newEvent := timestampedScaleEvent{replicaChange, time.Now(), false}
		if foundOldSample {
			c.scaleDownEvents[key][oldSampleIndex] = newEvent
		} else {
			c.scaleDownEvents[key] = append(c.scaleDownEvents[key], newEvent)
		}
	}
}

// stabilizeRecommendationWithBehaviors:
// - replaces old recommendation with the newest recommendation,
// - returns {max,min} of recommendations that are not older than constraints.Scale{Up,Down}.DelaySeconds
func (c *FHPAController) stabilizeRecommendationWithBehaviors(args NormalizationArg) (int32, string, string) {
	now := time.Now()

	foundOldSample := false
	oldSampleIndex := 0

	upRecommendation := args.DesiredReplicas
	upDelaySeconds := *args.ScaleUpBehavior.StabilizationWindowSeconds
	upCutoff := now.Add(-time.Second * time.Duration(upDelaySeconds))

	downRecommendation := args.DesiredReplicas
	downDelaySeconds := *args.ScaleDownBehavior.StabilizationWindowSeconds
	downCutoff := now.Add(-time.Second * time.Duration(downDelaySeconds))

	// Calculate the upper and lower stabilization limits.
	c.recommendationsLock.Lock()
	defer c.recommendationsLock.Unlock()
	for i, rec := range c.recommendations[args.Key] {
		if rec.timestamp.After(upCutoff) {
			upRecommendation = min(rec.recommendation, upRecommendation)
		}
		if rec.timestamp.After(downCutoff) {
			downRecommendation = max(rec.recommendation, downRecommendation)
		}
		if rec.timestamp.Before(upCutoff) && rec.timestamp.Before(downCutoff) {
			foundOldSample = true
			oldSampleIndex = i
		}
	}

	// Bring the recommendation to within the upper and lower limits (stabilize).
	recommendation := args.CurrentReplicas
	if recommendation < upRecommendation {
		recommendation = upRecommendation
	}
	if recommendation > downRecommendation {
		recommendation = downRecommendation
	}

	// Record the unstabilized recommendation.
	if foundOldSample {
		c.recommendations[args.Key][oldSampleIndex] = timestampedRecommendation{args.DesiredReplicas, time.Now()}
	} else {
		c.recommendations[args.Key] = append(c.recommendations[args.Key], timestampedRecommendation{args.DesiredReplicas, time.Now()})
	}

	// Determine a human-friendly message.
	var reason, message string
	if args.DesiredReplicas >= args.CurrentReplicas {
		reason = "ScaleUpStabilized"
		message = "recent recommendations were lower than current one, applying the lowest recent recommendation"
	} else {
		reason = "ScaleDownStabilized"
		message = "recent recommendations were higher than current one, applying the highest recent recommendation"
	}
	return recommendation, reason, message
}

// convertDesiredReplicasWithBehaviorRate performs the actual normalization, given the constraint rate
// It doesn't consider the stabilizationWindow, it is done separately
func (c *FHPAController) convertDesiredReplicasWithBehaviorRate(args NormalizationArg) (int32, string, string) {
	var possibleLimitingReason, possibleLimitingMessage string

	if args.DesiredReplicas > args.CurrentReplicas {
		c.scaleUpEventsLock.RLock()
		defer c.scaleUpEventsLock.RUnlock()
		c.scaleDownEventsLock.RLock()
		defer c.scaleDownEventsLock.RUnlock()
		scaleUpLimit := calculateScaleUpLimitWithScalingRules(args.CurrentReplicas, c.scaleUpEvents[args.Key], c.scaleDownEvents[args.Key], args.ScaleUpBehavior)

		if scaleUpLimit < args.CurrentReplicas {
			// We shouldn't scale up further until the scaleUpEvents will be cleaned up
			scaleUpLimit = args.CurrentReplicas
		}
		maximumAllowedReplicas := args.MaxReplicas
		if maximumAllowedReplicas > scaleUpLimit {
			maximumAllowedReplicas = scaleUpLimit
			possibleLimitingReason = "ScaleUpLimit"
			possibleLimitingMessage = "the desired replica count is increasing faster than the maximum scale rate"
		} else {
			possibleLimitingReason = "TooManyReplicas"
			possibleLimitingMessage = "the desired replica count is more than the maximum replica count"
		}
		if args.DesiredReplicas > maximumAllowedReplicas {
			return maximumAllowedReplicas, possibleLimitingReason, possibleLimitingMessage
		}
	} else if args.DesiredReplicas < args.CurrentReplicas {
		c.scaleUpEventsLock.RLock()
		defer c.scaleUpEventsLock.RUnlock()
		c.scaleDownEventsLock.RLock()
		defer c.scaleDownEventsLock.RUnlock()
		scaleDownLimit := calculateScaleDownLimitWithBehaviors(args.CurrentReplicas, c.scaleUpEvents[args.Key], c.scaleDownEvents[args.Key], args.ScaleDownBehavior)

		if scaleDownLimit > args.CurrentReplicas {
			// We shouldn't scale down further until the scaleDownEvents will be cleaned up
			scaleDownLimit = args.CurrentReplicas
		}
		minimumAllowedReplicas := args.MinReplicas
		if minimumAllowedReplicas < scaleDownLimit {
			minimumAllowedReplicas = scaleDownLimit
			possibleLimitingReason = "ScaleDownLimit"
			possibleLimitingMessage = "the desired replica count is decreasing faster than the maximum scale rate"
		} else {
			possibleLimitingMessage = "the desired replica count is less than the minimum replica count"
			possibleLimitingReason = "TooFewReplicas"
		}
		if args.DesiredReplicas < minimumAllowedReplicas {
			return minimumAllowedReplicas, possibleLimitingReason, possibleLimitingMessage
		}
	}
	return args.DesiredReplicas, "DesiredWithinRange", "the desired count is within the acceptable range"
}

// convertDesiredReplicas performs the actual normalization, without depending on `HorizontalController` or `HorizontalPodAutoscaler`
func convertDesiredReplicasWithRules(currentReplicas, desiredReplicas, hpaMinReplicas, hpaMaxReplicas int32) (int32, string, string) {
	var minimumAllowedReplicas int32
	var maximumAllowedReplicas int32

	var possibleLimitingCondition string
	var possibleLimitingReason string

	minimumAllowedReplicas = hpaMinReplicas

	// Do not scaleup too much to prevent incorrect rapid increase of the number of master replicas caused by
	// bogus CPU usage report from heapster/kubelet (like in issue #32304).
	scaleUpLimit := calculateScaleUpLimit(currentReplicas)

	if hpaMaxReplicas > scaleUpLimit {
		maximumAllowedReplicas = scaleUpLimit
		possibleLimitingCondition = "ScaleUpLimit"
		possibleLimitingReason = "the desired replica count is increasing faster than the maximum scale rate"
	} else {
		maximumAllowedReplicas = hpaMaxReplicas
		possibleLimitingCondition = "TooManyReplicas"
		possibleLimitingReason = "the desired replica count is more than the maximum replica count"
	}

	if desiredReplicas < minimumAllowedReplicas {
		possibleLimitingCondition = "TooFewReplicas"
		possibleLimitingReason = "the desired replica count is less than the minimum replica count"

		return minimumAllowedReplicas, possibleLimitingCondition, possibleLimitingReason
	} else if desiredReplicas > maximumAllowedReplicas {
		return maximumAllowedReplicas, possibleLimitingCondition, possibleLimitingReason
	}

	return desiredReplicas, "DesiredWithinRange", "the desired count is within the acceptable range"
}

func calculateScaleUpLimit(currentReplicas int32) int32 {
	return int32(math.Max(scaleUpLimitFactor*float64(currentReplicas), scaleUpLimitMinimum))
}

// markScaleEventsOutdated set 'outdated=true' flag for all scale events that are not used by any HPA object
func markScaleEventsOutdated(scaleEvents []timestampedScaleEvent, longestPolicyPeriod int32) {
	period := time.Second * time.Duration(longestPolicyPeriod)
	cutoff := time.Now().Add(-period)
	for i, event := range scaleEvents {
		if event.timestamp.Before(cutoff) {
			// outdated scale event are marked for later reuse
			scaleEvents[i].outdated = true
		}
	}
}

func getLongestPolicyPeriod(scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var longestPolicyPeriod int32
	for _, policy := range scalingRules.Policies {
		if policy.PeriodSeconds > longestPolicyPeriod {
			longestPolicyPeriod = policy.PeriodSeconds
		}
	}
	return longestPolicyPeriod
}

// calculateScaleUpLimitWithScalingRules returns the maximum number of pods that could be added for the given HPAScalingRules
func calculateScaleUpLimitWithScalingRules(currentReplicas int32, scaleUpEvents, scaleDownEvents []timestampedScaleEvent, scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var result int32
	var proposed int32
	var selectPolicyFn func(int32, int32) int32
	if scalingRules.SelectPolicy != nil && *scalingRules.SelectPolicy == autoscalingv2.DisabledPolicySelect {
		return currentReplicas // Scaling is disabled
	} else if scalingRules.SelectPolicy != nil && *scalingRules.SelectPolicy == autoscalingv2.MinChangePolicySelect {
		result = math.MaxInt32
		selectPolicyFn = minOfInt32 // For scaling up, the lowest change ('min' policy) produces a minimum value
	} else {
		result = math.MinInt32
		selectPolicyFn = maxOfInt32 // Use the default policy otherwise to produce a highest possible change
	}
	for _, policy := range scalingRules.Policies {
		replicasAddedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleUpEvents)
		replicasDeletedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleDownEvents)
		periodStartReplicas := currentReplicas - replicasAddedInCurrentPeriod + replicasDeletedInCurrentPeriod
		switch policy.Type {
		case autoscalingv2.PodsScalingPolicy:
			proposed = periodStartReplicas + policy.Value
		case autoscalingv2.PercentScalingPolicy:
			// the proposal has to be rounded up because the proposed change might not increase the replica count causing the target to never scale up
			proposed = int32(math.Ceil(float64(periodStartReplicas) * (1 + float64(policy.Value)/100)))
		}
		result = selectPolicyFn(result, proposed)
	}
	return result
}

// calculateScaleDownLimitWithBehavior returns the maximum number of pods that could be deleted for the given HPAScalingRules
func calculateScaleDownLimitWithBehaviors(currentReplicas int32, scaleUpEvents, scaleDownEvents []timestampedScaleEvent, scalingRules *autoscalingv2.HPAScalingRules) int32 {
	var result int32
	var proposed int32
	var selectPolicyFn func(int32, int32) int32
	if scalingRules.SelectPolicy != nil && *scalingRules.SelectPolicy == autoscalingv2.DisabledPolicySelect {
		return currentReplicas // Scaling is disabled
	} else if scalingRules.SelectPolicy != nil && *scalingRules.SelectPolicy == autoscalingv2.MinChangePolicySelect {
		result = math.MinInt32
		selectPolicyFn = maxOfInt32 // For scaling down, the lowest change ('min' policy) produces a maximum value
	} else {
		result = math.MaxInt32
		selectPolicyFn = minOfInt32 // Use the default policy otherwise to produce a highest possible change
	}
	for _, policy := range scalingRules.Policies {
		replicasAddedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleUpEvents)
		replicasDeletedInCurrentPeriod := getReplicasChangePerPeriod(policy.PeriodSeconds, scaleDownEvents)
		periodStartReplicas := currentReplicas - replicasAddedInCurrentPeriod + replicasDeletedInCurrentPeriod
		switch policy.Type {
		case autoscalingv2.PodsScalingPolicy:
			proposed = periodStartReplicas - policy.Value
		case autoscalingv2.PercentScalingPolicy:
			proposed = int32(float64(periodStartReplicas) * (1 - float64(policy.Value)/100))
		}
		result = selectPolicyFn(result, proposed)
	}
	return result
}

// setCurrentReplicasInStatus sets the current replica count in the status of the HPA.
func (c *FHPAController) setCurrentReplicasInStatus(hpa *autoscalingv1alpha1.FederatedHPA, currentReplicas int32) {
	c.setStatus(hpa, currentReplicas, hpa.Status.DesiredReplicas, hpa.Status.CurrentMetrics, false)
}

// setStatus recreates the status of the given HPA, updating the current and
// desired replicas, as well as the metric statuses
func (c *FHPAController) setStatus(hpa *autoscalingv1alpha1.FederatedHPA, currentReplicas, desiredReplicas int32, metricStatuses []autoscalingv2.MetricStatus, rescale bool) {
	hpa.Status = autoscalingv2.HorizontalPodAutoscalerStatus{
		CurrentReplicas: currentReplicas,
		DesiredReplicas: desiredReplicas,
		LastScaleTime:   hpa.Status.LastScaleTime,
		CurrentMetrics:  metricStatuses,
		Conditions:      hpa.Status.Conditions,
	}

	if rescale {
		now := metav1.NewTime(time.Now())
		hpa.Status.LastScaleTime = &now
	}
}

// updateStatusIfNeeded calls updateStatus only if the status of the new HPA is not the same as the old status
func (c *FHPAController) updateStatusIfNeeded(ctx context.Context, oldStatus *autoscalingv2.HorizontalPodAutoscalerStatus, newHPA *autoscalingv1alpha1.FederatedHPA) error {
	// skip a write if we wouldn't need to update
	if apiequality.Semantic.DeepEqual(oldStatus, &newHPA.Status) {
		return nil
	}
	return c.updateStatus(ctx, newHPA)
}

// updateStatus actually does the update request for the status of the given HPA
func (c *FHPAController) updateStatus(ctx context.Context, hpa *autoscalingv1alpha1.FederatedHPA) error {
	err := c.Status().Update(ctx, hpa)
	if err != nil {
		c.EventRecorder.Event(hpa, corev1.EventTypeWarning, "FailedUpdateStatus", err.Error())
		return fmt.Errorf("failed to update status for %s: %v", hpa.Name, err)
	}
	klog.V(2).InfoS("Successfully updated status for hpa", "hpaName", hpa.Name)
	return nil
}

// setCondition sets the specific condition type on the given HPA to the specified value with the given reason
// and message.  The message and args are treated like a format string.  The condition will be added if it is
// not present.
func setCondition(hpa *autoscalingv1alpha1.FederatedHPA, conditionType autoscalingv2.HorizontalPodAutoscalerConditionType, status corev1.ConditionStatus, reason, message string, args ...interface{}) {
	hpa.Status.Conditions = setConditionInList(hpa.Status.Conditions, conditionType, status, reason, message, args...)
}

// setConditionInList sets the specific condition type on the given HPA to the specified value with the given
// reason and message.  The message and args are treated like a format string.  The condition will be added if
// it is not present.  The new list will be returned.
func setConditionInList(inputList []autoscalingv2.HorizontalPodAutoscalerCondition, conditionType autoscalingv2.HorizontalPodAutoscalerConditionType, status corev1.ConditionStatus, reason, message string, args ...interface{}) []autoscalingv2.HorizontalPodAutoscalerCondition {
	resList := inputList
	var existingCond *autoscalingv2.HorizontalPodAutoscalerCondition
	for i, condition := range resList {
		if condition.Type == conditionType {
			// can't take a pointer to an iteration variable
			existingCond = &resList[i]
			break
		}
	}

	if existingCond == nil {
		resList = append(resList, autoscalingv2.HorizontalPodAutoscalerCondition{
			Type: conditionType,
		})
		existingCond = &resList[len(resList)-1]
	}

	if existingCond.Status != status {
		existingCond.LastTransitionTime = metav1.Now()
	}

	existingCond.Status = status
	existingCond.Reason = reason
	existingCond.Message = fmt.Sprintf(message, args...)

	return resList
}

func maxOfInt32(a, b int32) int32 {
	if a >= b {
		return a
	}
	return b
}

func minOfInt32(a, b int32) int32 {
	if a <= b {
		return a
	}
	return b
}
