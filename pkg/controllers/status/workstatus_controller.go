package status

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// WorkStatusControllerName is the controller name that will be used when reporting events.
const WorkStatusControllerName = "work-status-controller"

// TODO(chenxianpao): informer should be closed if need.
var stopCh = make(chan struct{})

// PropagationWorkStatusController is to sync status of PropagationWork.
type PropagationWorkStatusController struct {
	client.Client                       // used to operate PropagationWork resources.
	DynamicClient     dynamic.Interface // used to fetch arbitrary resources.
	EventRecorder     record.EventRecorder
	RESTMapper        meta.RESTMapper
	KarmadaClient     karmadaclientset.Interface // used to get MemberCluster resources.
	KubeClientSet     kubernetes.Interface       // used to get kubernetes resources.
	InformerManager   informermanager.MultiClusterInformerManager
	EventHandlerFuncs *cache.ResourceEventHandlerFuncs
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

// buildResourceInformers builds informers for kubernetes resources and CRDs, the callback function of informers will
// update status to propagationWork.
func (c *PropagationWorkStatusController) buildResourceInformers(work *v1alpha1.PropagationWork) (controllerruntime.Result, error) {
	err := c.registerInformersAndStart(work)
	if err != nil {
		klog.Errorf("Failed to register informer for propagationWork %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return controllerruntime.Result{Requeue: true}, err
	}
	return controllerruntime.Result{}, nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (c *PropagationWorkStatusController) registerInformersAndStart(work *v1alpha1.PropagationWork) error {
	memberClusterName, err := names.GetMemberClusterName(work.GetNamespace())
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", work.GetNamespace(), err)
		return err
	}

	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := c.InformerManager.ForExistCluster(memberClusterName)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := c.buildDynamicClusterClient(memberClusterName)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", memberClusterName)
			return err
		}
		singleClusterInformerManager = c.InformerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}

	var gvrTargets []schema.GroupVersionResource
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload. Error: %v.", err)
			return err
		}

		dynamicResource, err := restmapper.GetGroupVersionResource(c.RESTMapper, workload.GroupVersionKind())
		if err != nil {
			klog.Errorf("Failed to get GVR from GVK for resource %s/%s. Error: %v.", workload.GetNamespace(), workload.GetName(), err)
			return err
		}
		gvrTargets = append(gvrTargets, dynamicResource)
		if c.EventHandlerFuncs == nil {
			c.EventHandlerFuncs = informermanager.NewTriggerOnAllChanges(c.syncPropagationWorkStatus)
		}
		singleClusterInformerManager.ForResource(dynamicResource, c.EventHandlerFuncs)
	}
	c.InformerManager.Start(memberClusterName, stopCh)
	synced := c.InformerManager.WaitForCacheSync(memberClusterName, stopCh)
	if synced == nil {
		klog.Errorf("No informerFactory for cluster %s exist.", memberClusterName)
		return fmt.Errorf("no informerFactory for cluster %s exist", memberClusterName)
	}
	for _, gvr := range gvrTargets {
		if !synced[gvr] {
			klog.Errorf("Informer for %s hasn't synced.", gvr)
			return fmt.Errorf("informer for %s hasn't synced", gvr)
		}
	}
	return nil
}

// buildDynamicClusterClient builds dynamic client for informerFactory by clusterName,
// it will build kubeconfig from memberCluster resource and construct dynamic client.
func (c *PropagationWorkStatusController) buildDynamicClusterClient(cluster string) (*util.DynamicClusterClient, error) {
	// TODO(RainbowMango): retrieve member cluster from the local cache instead of a real request to API server.
	memberCluster, err := c.KarmadaClient.MemberclusterV1alpha1().MemberClusters().Get(context.TODO(), cluster, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to the get given member cluster %s", cluster)
		return nil, err
	}

	if !util.IsMemberClusterReady(&memberCluster.Status) {
		klog.Errorf("The status of the given member cluster %s is unready", memberCluster.GetName())
		return nil, fmt.Errorf("cluster %s is not ready", memberCluster.Name)
	}

	dynamicClusterClient, err := util.NewClusterDynamicClientSet(memberCluster, c.KubeClientSet)
	if err != nil {
		klog.Errorf("Failed to get dynamic client for cluster %s.", memberCluster.GetName())
		return nil, err
	}
	return dynamicClusterClient, nil
}

// syncPropagationWorkStatus will find propagationWork by label in workload, then update resource status to propagationWork status.
// label example: "karmada.io/created-by: karmada-es-member-cluster-1.default-deployment-nginx"
func (c *PropagationWorkStatusController) syncPropagationWorkStatus(obj runtime.Object) error {
	resource := obj.(*unstructured.Unstructured)

	// Ignore resources that don't match labels.
	propagationWork, find, err := c.locatePropagationWork(resource)
	if err != nil && find {
		return err
	} else if err != nil && !find {
		return nil
	}

	statusRawExtension, err := c.generateStatusRawExtension(resource)
	if err != nil {
		klog.Errorf("Failed to generate status rawExtension. Error: %v.", err)
		return err
	}

	err = c.updatePropagationWorkStatus(propagationWork, resource, statusRawExtension)
	if err != nil {
		klog.Errorf("Failed to update status of propagationWork %s/%s. Error: %v.", propagationWork.GetNamespace(), propagationWork.GetName(), err)
		return err
	}
	return nil
}

// updatePropagationWorkStatus will find the index of manifest for specified resource, then update resource status to
// corresponding location in propagationWork.Status
// TODO(chenxianpao): If delete one manifest in propagationWork, how to delete resource status in propagationWork status array.
func (c *PropagationWorkStatusController) updatePropagationWorkStatus(propagationWork *v1alpha1.PropagationWork, resource *unstructured.Unstructured, statusRawExtension *runtime.RawExtension) error {
	hitManifestIndex, err := c.matchManifest(propagationWork, resource)
	if err != nil {
		klog.Errorf("Failed to match resource in propagationWork %s/%s. Error: %v.",
			propagationWork.GetNamespace(), propagationWork.GetName(), err)
		return err
	}

	hitManifestStatusIndex, err := c.matchManifestStatus(propagationWork, resource)
	if err != nil {
		klog.Errorf("Failed to match manifest status in propagationWork %s/%s. Error: %v.",
			propagationWork.GetNamespace(), propagationWork.GetName(), err)
		return err
	}

	if hitManifestStatusIndex == util.NotFound {
		gv, err := schema.ParseGroupVersion(resource.GetAPIVersion())
		if err != nil {
			klog.Errorf("Failed to parse group version by %s. Error: %v.", resource.GetAPIVersion(), err)
			return err
		}
		propagationWork.Status.ManifestStatuses = append(propagationWork.Status.ManifestStatuses, v1alpha1.ManifestStatus{
			Identifier: v1alpha1.ResourceIdentifier{
				Ordinal:   hitManifestIndex,
				Group:     gv.Group,
				Version:   gv.Version,
				Kind:      resource.GetKind(),
				Namespace: resource.GetNamespace(),
				Name:      resource.GetName(),
			},
			Status: *statusRawExtension,
		})
	} else {
		propagationWork.Status.ManifestStatuses[hitManifestStatusIndex].Status = *statusRawExtension
	}
	// TODO(chenxianpao): how to ensure update status successfully? We should add a worker queue.
	_, err = c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(propagationWork.GetNamespace()).UpdateStatus(context.TODO(), propagationWork, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update status of propagationWork %s/%s. Error: %v.", propagationWork.GetNamespace(), propagationWork.GetName(), err)
		return err
	}
	return nil
}

// locatePropagationWork gets propagationWork by labels in the resources from member cluster, if no matched label exist, do nothing.
func (c *PropagationWorkStatusController) locatePropagationWork(resource *unstructured.Unstructured) (*v1alpha1.PropagationWork, bool, error) {
	workloadLabel := resource.GetLabels()
	if workloadLabel == nil {
		klog.V(2).Infof("can't locate propagationWork by resource %s/%s/%s, because label is empty.", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
		return nil, false, fmt.Errorf("can't locate propagationWork by resource %s/%s/%s, because label is empty", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
	}
	value, exist := workloadLabel[util.OwnerLabel]
	if !exist {
		klog.V(2).Infof("can't locate propagationWork by resource %s/%s/%s, because label is not exist.", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
		return nil, false, fmt.Errorf("can't locate propagationWork by resource %s/%s/%s, because label is not exist", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
	}

	namespace, name, err := names.GetNamespaceAndName(value)
	if err != nil {
		klog.Errorf("Failed to get namespace and name by %s. Error: %v.", value, err)
		return nil, true, err
	}

	propagationWork, err := c.KarmadaClient.PropagationstrategyV1alpha1().PropagationWorks(namespace).Get(context.TODO(), name, v1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("Failed to get propagationWork %s/%s. Error: %v.", namespace, name, err)
		return nil, true, err
	}
	if err != nil && errors.IsNotFound(err) {
		return nil, false, err
	}
	return propagationWork, true, err
}

// generateStatusRawExtension gets status from unstructured resource, return status with rawExtension format, if status
// not exist, return empty rawExtension.
func (c *PropagationWorkStatusController) generateStatusRawExtension(resource *unstructured.Unstructured) (*runtime.RawExtension, error) {
	status, find, err := unstructured.NestedMap(resource.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status information. Error: %v", err)
		return nil, err
	}
	if !find {
		klog.Infof("Status information is not exist.")
		emptyJSON, err := json.Marshal(map[string]string{})
		if err != nil {
			klog.Errorf("Failed to marshal empty status. Error: %v.", emptyJSON)
			return nil, err
		}
		return &runtime.RawExtension{
			Raw: emptyJSON,
		}, nil
	}
	statusJSON, err := json.Marshal(status)
	if err != nil {
		klog.Errorf("Failed to marshal status. Error: %v.", statusJSON)
		return nil, err
	}
	return &runtime.RawExtension{
		Raw: statusJSON,
	}, nil
}

// matchManifestStatus gets index of hit manifest status, if no matched manifest status, return util.NotFound.
func (c *PropagationWorkStatusController) matchManifestStatus(propagationWork *v1alpha1.PropagationWork, resource *unstructured.Unstructured) (int, error) {
	hitManifestStatusIndex := util.NotFound
	gv, err := schema.ParseGroupVersion(resource.GetAPIVersion())
	if err != nil {
		klog.Errorf("Failed to parse group version by %s. Error: %v.", resource.GetAPIVersion(), err)
		return util.NotFound, err
	}
	for index, manifestStatus := range propagationWork.Status.ManifestStatuses {
		// The five types(kind\namespace\name\group\version) ensure that resource status is matched.
		if manifestStatus.Identifier.Kind == resource.GetKind() &&
			manifestStatus.Identifier.Namespace == resource.GetNamespace() &&
			manifestStatus.Identifier.Name == resource.GetName() &&
			manifestStatus.Identifier.Group == gv.Group &&
			manifestStatus.Identifier.Version == gv.Version {
			hitManifestStatusIndex = index
			break
		}
	}
	return hitManifestStatusIndex, nil
}

// matchManifest gets index of hit manifest, if no matched manifest, return util.NotFound.
func (c *PropagationWorkStatusController) matchManifest(propagationWork *v1alpha1.PropagationWork, resource *unstructured.Unstructured) (int, error) {
	hitManifestIndex := util.NotFound
	for index, manifest := range propagationWork.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload. Error: %v.", err)
			return util.NotFound, err
		}

		// The four types(kind\namespace\name\apiVersion) ensure that resource is matched.
		if workload.GetKind() == resource.GetKind() && workload.GetNamespace() == resource.GetNamespace() &&
			workload.GetName() == resource.GetName() && workload.GetAPIVersion() == resource.GetAPIVersion() {
			hitManifestIndex = index
			break
		}
	}

	if hitManifestIndex == util.NotFound {
		klog.Errorf("Failed to match resource %s/%s/%s in propagationWork %s/%s.", resource.GetKind(), resource.GetNamespace(),
			resource.GetName(), propagationWork.GetNamespace(), propagationWork.GetName())
		return util.NotFound, fmt.Errorf("failed to match resource in propagationWork")
	}
	return hitManifestIndex, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *PropagationWorkStatusController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).For(&v1alpha1.PropagationWork{}).Complete(c)
}
