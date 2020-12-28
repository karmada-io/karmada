package status

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// PropagationWorkStatusController is to sync status of PropagationWork.
type PropagationWorkStatusController struct {
	client.Client                     // used to operate PropagationWork resources.
	DynamicClient   dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder   record.EventRecorder
	RESTMapper      meta.RESTMapper
	KubeClientSet   kubernetes.Interface // used to get kubernetes resources.
	InformerManager informermanager.MultiClusterInformerManager
	eventHandler    cache.ResourceEventHandler // eventHandler knows how to handle events from the member cluster.
	StopChan        <-chan struct{}
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *PropagationWorkStatusController) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Reconciling status of PropagationWork %s.", req.NamespacedName.String())

	work := &v1alpha1.PropagationWork{}
	if err := c.Client.Get(context.TODO(), req.NamespacedName, work); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !work.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	return c.buildResourceInformers(work)
}

// buildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant PropagationWork object.
func (c *PropagationWorkStatusController) buildResourceInformers(work *v1alpha1.PropagationWork) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(work)
	if err != nil {
		klog.Errorf("Failed to register informer for propagationWork %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// getEventHandler return callback function that knows how to handle events from the member cluster.
func (c *PropagationWorkStatusController) getEventHandler() cache.ResourceEventHandler {
	if c.eventHandler == nil {
		c.eventHandler = informermanager.NewHandlerOnAllEvents(c.syncPropagationWorkStatus)
	}
	return c.eventHandler
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *PropagationWorkStatusController) registerInformersAndStart(work *v1alpha1.PropagationWork) error {
	memberClusterName, err := names.GetMemberClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return err
	}

	singleClusterInformerManager, err := c.getSingleClusterManager(memberClusterName)
	if err != nil {
		return err
	}

	gvrTargets, err := c.getGVRsFromPropagationWork(work)
	if err != nil {
		return err
	}

	for gvr := range gvrTargets {
		singleClusterInformerManager.ForResource(gvr, c.getEventHandler())
	}

	c.InformerManager.Start(memberClusterName, c.StopChan)
	synced := c.InformerManager.WaitForCacheSync(memberClusterName, c.StopChan)
	if synced == nil {
		klog.Errorf("No informerFactory for cluster %s exist.", memberClusterName)
		return fmt.Errorf("no informerFactory for cluster %s exist", memberClusterName)
	}
	for gvr := range gvrTargets {
		if !synced[gvr] {
			klog.Errorf("Informer for %s hasn't synced.", gvr)
			return fmt.Errorf("informer for %s hasn't synced", gvr)
		}
	}
	return nil
}

// getGVRsFromPropagationWork traverses the manifests in propagationWork to find groupVersionResource list.
func (c *PropagationWorkStatusController) getGVRsFromPropagationWork(work *v1alpha1.PropagationWork) (map[schema.GroupVersionResource]bool, error) {
	gvrTargets := map[schema.GroupVersionResource]bool{}
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload. Error: %v.", err)
			return nil, err
		}
		gvr, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
		if err != nil {
			klog.Errorf("Failed to get GVR from GVK for resource %s/%s. Error: %v.", workload.GetNamespace(), workload.GetName(), err)
			return nil, err
		}
		gvrTargets[gvr] = true
	}
	return gvrTargets, nil
}

// getSingleClusterManager gets singleClusterInformerManager with clusterName.
// If manager is not exist, create it, otherwise gets it from map.
func (c *PropagationWorkStatusController) getSingleClusterManager(memberClusterName string) (informermanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.GetSingleClusterManager(memberClusterName)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := util.BuildDynamicClusterClient(c.Client, c.KubeClientSet, memberClusterName)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", memberClusterName)
			return nil, err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}
	return singleClusterInformerManager, nil
}

// syncPropagationWorkStatus will find propagationWork by label in workload, then update resource status to propagationWork status.
// label example: "karmada.io/created-by: karmada-es-member-cluster-1.default-deployment-nginx"
// TODO(chenxianpao): sync workload status to propagationWork status.
func (c *PropagationWorkStatusController) syncPropagationWorkStatus(obj runtime.Object) error {
	resource := obj.(*unstructured.Unstructured)
	klog.Infof("sync obj is %s/%s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationWorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationWork{}).Complete(c)
}
