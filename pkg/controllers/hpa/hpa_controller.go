package hpa

import (
	"context"
	"math"
	"time"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "hpa-controller"

// HorizontalPodAutoscalerController is to sync HorizontalPodAutoscaler.
type HorizontalPodAutoscalerController struct {
	client.Client                   // used to operate HorizontalPodAutoscaler resources.
	DynamicClient dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder record.EventRecorder
	RESTMapper    meta.RESTMapper
	Interval      time.Duration // the interval to process hpa schedule.
}

// HorizontalPodAutoscalerReplicas records replicas for each member cluster.
type HorizontalPodAutoscalerReplicas struct {
	Cluster     string
	MaxReplicas float64
	MinReplicas float64
}

// scheduleHPA change max or min replicas in hpa according to work status.
func (c *HorizontalPodAutoscalerController) scheduleHPA() {
	HPAList := &autoscalingv1.HorizontalPodAutoscalerList{}
	if err := c.Client.List(context.TODO(), HPAList, &client.ListOptions{}); err != nil {
		klog.Errorf("Failed to list hpa objects. Error: %v.", err)
		return
	}
	for _, HPAObject := range HPAList.Items {
		HPAWorkList := &v1alpha1.PropagationWorkList{}
		labelRequirement, err := labels.NewRequirement(util.OwnerLabel, selection.Equals,
			[]string{names.GenerateOwnerLabelValue(HPAObject.GetNamespace(), HPAObject.GetName())})
		if err != nil {
			klog.Errorf("Failed to new a requirement. Error: %v", err)
			return
		}
		selector := labels.NewSelector().Add(*labelRequirement)
		if err := c.Client.List(context.TODO(), HPAWorkList, &client.ListOptions{LabelSelector: selector}); err != nil {
			klog.Errorf("Failed to list works by hpa %s/%s. Error: %v.", HPAObject.GetNamespace(),
				HPAObject.GetName(), err)
			return
		}
		for _, workObject := range HPAWorkList.Items {
			klog.V(2).Infof("process work %s/%s", workObject.GetNamespace(), workObject.GetName())
			// TODO: change max or min replicas in hpa according to work status.
		}
	}
}

// RunWorker start a scheduled task to schedule hpa object periodical.
func (c *HorizontalPodAutoscalerController) RunWorker(stopChan <-chan struct{}) {
	go wait.Until(c.scheduleHPA, c.Interval, stopChan)
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *HorizontalPodAutoscalerController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling HorizontalPodAutoscaler %s.", req.NamespacedName.String())

	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, hpa); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !hpa.DeletionTimestamp.IsZero() {
		// Do nothing, just return.
		return controllerruntime.Result{}, nil
	}

	return c.syncHPA(hpa)
}

// syncHPA gets placement from propagationBinding according to targetRef in hpa, then builds propagationWorks in target execution namespaces.
func (c *HorizontalPodAutoscalerController) syncHPA(hpa *autoscalingv1.HorizontalPodAutoscaler) (controllerruntime.Result, error) {
	clusters, err := c.getTargetPlacement(hpa.Spec.ScaleTargetRef, hpa.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get target placement by hpa %s/%s. Error: %v.", hpa.GetNamespace(), hpa.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	err = c.buildPropagationWorks(hpa, clusters)
	if err != nil {
		klog.Errorf("Failed to build propagationWork for hpa %s/%s. Error: %v.", hpa.GetNamespace(), hpa.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// calculateReplicas calculates the replicas for hpa in each member cluster. It will two methods, average or weight.
// TODO: this function just implements even distribution. Another way to change HPA replicas by cluster weight need to be implemented.
// Average: If averageReplica is less than 1, averageReplica will be assigned by 1, so the total replicas may more than replica in hpa object.
// If MaxReplica or MinReplica can't be divisible, the last cluster will get the remaining replicas.
// Weight: not currently implemented
func (c *HorizontalPodAutoscalerController) calculateReplicas(hpa *autoscalingv1.HorizontalPodAutoscaler, clusters []string) []HorizontalPodAutoscalerReplicas {
	clusterLength := len(clusters)
	maxReplicas := hpa.Spec.MaxReplicas
	minReplicas := hpa.Spec.MinReplicas
	if hpa.Spec.MinReplicas == nil {
		defaultMinReplicas := int32(1)
		minReplicas = &defaultMinReplicas
	}
	averageMaxReplicas := math.Floor(float64(maxReplicas) / float64(clusterLength))
	averageMinReplicas := math.Floor(float64(*minReplicas) / float64(clusterLength))
	maxReplicaIsNotEnough := false
	minReplicaIsNotEnough := false
	if averageMaxReplicas <= 0 {
		maxReplicaIsNotEnough = true
		averageMaxReplicas = 1
	}
	if averageMinReplicas <= 0 {
		minReplicaIsNotEnough = true
		averageMinReplicas = 1
	}
	var clustersReplica []HorizontalPodAutoscalerReplicas
	for index, cluster := range clusters {
		hpaReplica := HorizontalPodAutoscalerReplicas{
			Cluster:     cluster,
			MaxReplicas: averageMaxReplicas,
			MinReplicas: averageMinReplicas,
		}
		if index == clusterLength-1 {
			if !maxReplicaIsNotEnough && minReplicaIsNotEnough {
				hpaReplica.MaxReplicas = float64(maxReplicas) - float64(index)*averageMaxReplicas
				hpaReplica.MinReplicas = averageMinReplicas
			}
			if !maxReplicaIsNotEnough && !minReplicaIsNotEnough {
				hpaReplica.MaxReplicas = float64(maxReplicas) - float64(index)*averageMaxReplicas
				hpaReplica.MinReplicas = float64(*minReplicas) - float64(index)*averageMinReplicas
			}
			if !minReplicaIsNotEnough && maxReplicaIsNotEnough {
				hpaReplica.MaxReplicas = averageMaxReplicas
				hpaReplica.MinReplicas = float64(*minReplicas) - float64(index)*averageMinReplicas
			}
		}
		clustersReplica = append(clustersReplica, hpaReplica)
	}
	return clustersReplica
}

// buildPropagationWorks transforms hpa obj to unstructured, creates or updates propagationWorks in the target execution namespaces.
func (c *HorizontalPodAutoscalerController) buildPropagationWorks(hpa *autoscalingv1.HorizontalPodAutoscaler, clusters []string) error {
	clustersReplica := c.calculateReplicas(hpa, clusters)
	for _, clusterHPA := range clustersReplica {
		hpaCopy := hpa.DeepCopy()
		minReplica := int32(clusterHPA.MinReplicas)
		hpaCopy.Spec.MaxReplicas = int32(clusterHPA.MaxReplicas)
		hpaCopy.Spec.MinReplicas = &minReplica
		uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hpaCopy)
		if err != nil {
			klog.Errorf("Failed to transform hpa %s/%s. Error: %v", hpaCopy.GetNamespace(), hpaCopy.GetName(), err)
			return nil
		}
		hpaObj := &unstructured.Unstructured{Object: uncastObj}
		util.RemoveIrrelevantField(hpaObj)
		hpaJSON, err := hpaObj.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal hpa %s/%s. Error: %v",
				hpaObj.GetNamespace(), hpaObj.GetName(), err)
			return err
		}

		executionSpace, err := names.GenerateExecutionSpaceName(clusterHPA.Cluster)
		if err != nil {
			klog.Errorf("Failed to ensure PropagationWork for cluster: %s. Error: %v.", clusterHPA.Cluster, err)
			return err
		}

		hpaName := names.GenerateBindingName(hpaObj.GetNamespace(), hpaObj.GetKind(), hpaObj.GetName())
		objectMeta := metav1.ObjectMeta{
			Name:       hpaName,
			Namespace:  executionSpace,
			Finalizers: []string{util.ExecutionControllerFinalizer},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(hpa, hpa.GroupVersionKind()),
			},
			Labels: map[string]string{util.OwnerLabel: names.GenerateOwnerLabelValue(hpa.GetNamespace(), hpa.GetName())},
		}

		err = util.CreateOrUpdatePropagationWork(c.Client, objectMeta, hpaJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

// getTargetPlacement gets target clusters by CrossVersionObjectReference in hpa object. We can find
// the propagationBinding by resource with special naming rule, then get target clusters from propagationBinding.
func (c *HorizontalPodAutoscalerController) getTargetPlacement(objRef autoscalingv1.CrossVersionObjectReference, namespace string) ([]string, error) {
	// according to targetRef, find the resource.
	dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper,
		schema.FromAPIVersionAndKind(objRef.APIVersion, objRef.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", objRef.APIVersion, objRef.Kind, err)
		return nil, err
	}

	// Kind in CrossVersionObjectReference is not equal to the kind in bindingName, need to get obj from cache.
	unstructuredWorkLoad, err := c.DynamicClient.Resource(dynamicResource).Namespace(namespace).Get(context.TODO(), objRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	bindingName := names.GenerateBindingName(unstructuredWorkLoad.GetNamespace(), unstructuredWorkLoad.GetKind(), unstructuredWorkLoad.GetName())
	binding := &v1alpha1.PropagationBinding{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      bindingName,
	}
	if err := c.Client.Get(context.TODO(), namespacedName, binding); err != nil {
		return nil, err
	}
	return util.GetBindingClusterNames(binding), nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *HorizontalPodAutoscalerController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&autoscalingv1.HorizontalPodAutoscaler{}).Complete(c)
}
