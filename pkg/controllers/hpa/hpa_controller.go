package hpa

import (
	"context"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ControllerName is the controller name that will be used when reporting events.
const ControllerName = "hpa-controller"

// HorizontalPodAutoscalerController is to sync HorizontalPodAutoscaler.
type HorizontalPodAutoscalerController struct {
	client.Client                                               // used to operate HorizontalPodAutoscaler resources.
	DynamicClient   dynamic.Interface                           // used to fetch arbitrary resources from api server.
	InformerManager genericmanager.SingleClusterInformerManager // used to fetch arbitrary resources from cache.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *HorizontalPodAutoscalerController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling HorizontalPodAutoscaler %s.", req.NamespacedName.String())

	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := c.Client.Get(ctx, req.NamespacedName, hpa); err != nil {
		// The resource may no longer exist, in which case we delete related works.
		if apierrors.IsNotFound(err) {
			if err := c.deleteWorks(names.GenerateWorkName(util.HorizontalPodAutoscalerKind, req.Name, req.Namespace)); err != nil {
				return controllerruntime.Result{Requeue: true}, err
			}
			return controllerruntime.Result{}, err
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !hpa.DeletionTimestamp.IsZero() {
		// Do nothing, just return.
		return controllerruntime.Result{}, nil
	}

	return c.syncHPA(hpa)
}

// syncHPA gets placement from propagationBinding according to targetRef in hpa, then builds works in target execution namespaces.
func (c *HorizontalPodAutoscalerController) syncHPA(hpa *autoscalingv1.HorizontalPodAutoscaler) (controllerruntime.Result, error) {
	clusters, err := c.getTargetPlacement(hpa.Spec.ScaleTargetRef, hpa.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get target placement by hpa %s/%s. Error: %v.", hpa.GetNamespace(), hpa.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	err = c.buildWorks(hpa, clusters)
	if err != nil {
		klog.Errorf("Failed to build work for hpa %s/%s. Error: %v.", hpa.GetNamespace(), hpa.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// buildWorks transforms hpa obj to unstructured, creates or updates Works in the target execution namespaces.
func (c *HorizontalPodAutoscalerController) buildWorks(hpa *autoscalingv1.HorizontalPodAutoscaler, clusters []string) error {
	hpaObj, err := helper.ToUnstructured(hpa)
	if err != nil {
		klog.Errorf("Failed to transform hpa %s/%s. Error: %v", hpa.GetNamespace(), hpa.GetName(), err)
		return nil
	}
	for _, clusterName := range clusters {
		workNamespace := names.GenerateExecutionSpaceName(clusterName)
		workName := names.GenerateWorkName(hpaObj.GetKind(), hpaObj.GetName(), hpa.GetNamespace())
		objectMeta := metav1.ObjectMeta{
			Name:       workName,
			Namespace:  workNamespace,
			Finalizers: []string{util.ExecutionControllerFinalizer},
		}

		util.MergeLabel(hpaObj, workv1alpha1.WorkNamespaceLabel, workNamespace)
		util.MergeLabel(hpaObj, workv1alpha1.WorkNameLabel, workName)

		if err = helper.CreateOrUpdateWork(c.Client, objectMeta, hpaObj); err != nil {
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
	workload, err := c.InformerManager.Lister(dynamicResource).ByNamespace(namespace).Get(objRef.Name)
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get workload from cache, kind: %s, namespace: %s, name: %s. Error: %v. Fall back to call api server",
			objRef.Kind, namespace, objRef.Name, err)
		workload, err = c.DynamicClient.Resource(dynamicResource).Namespace(namespace).Get(context.TODO(),
			objRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get workload from api server, kind: %s, namespace: %s, name: %s. Error: %v",
				objRef.Kind, namespace, objRef.Name, err)
			return nil, err
		}
	}
	unstructuredWorkLoad, err := helper.ToUnstructured(workload)
	if err != nil {
		klog.Errorf("Failed to transform object(%s/%s): %v", namespace, objRef.Name, err)
		return nil, err
	}
	bindingName := names.GenerateBindingName(unstructuredWorkLoad.GetKind(), unstructuredWorkLoad.GetName())
	binding := &workv1alpha2.ResourceBinding{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      bindingName,
	}
	if err := c.Client.Get(context.TODO(), namespacedName, binding); err != nil {
		return nil, err
	}
	return util.GetBindingClusterNames(&binding.Spec), nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *HorizontalPodAutoscalerController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&autoscalingv1.HorizontalPodAutoscaler{}).Complete(c)
}

func (c *HorizontalPodAutoscalerController) deleteWorks(workName string) error {
	workList := &workv1alpha1.WorkList{}
	var errs []error
	if err := c.List(context.TODO(), workList); err != nil {
		klog.Errorf("Failed to list works: %v.", err)
		return err
	}

	for i := range workList.Items {
		work := &workList.Items[i]
		if workName == work.Name {
			if err := c.Client.Delete(context.TODO(), work); err != nil && !apierrors.IsNotFound(err) {
				klog.Errorf("Failed to delete work %s/%s: %v.", work.Namespace, work.Name, err)
				errs = append(errs, err)
			}
		}
	}
	return utilerrors.NewAggregate(errs)
}
