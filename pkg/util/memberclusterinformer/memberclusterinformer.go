package memberclusterinformer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var _ MemberClusterInformer = &memberClusterInformerImpl{}

type MemberClusterInformer interface {
	// BuildResourceInformers builds informer dynamically for managed resources in member cluster.
	// The created informer watches resource change and then sync to the relevant Work object.
	BuildResourceInformers(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work, handler cache.ResourceEventHandler) error

	// GetObjectFromCache returns object in cache by federated key.
	GetObjectFromCache(fedKey keys.FederatedKey) (*unstructured.Unstructured, error)
}

func NewMemberClusterInformer(
	c client.Client,
	restMapper meta.RESTMapper,
	informerManager genericmanager.MultiClusterInformerManager,
	clusterCacheSyncTimeout metav1.Duration,
	clusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error),
) MemberClusterInformer {
	return &memberClusterInformerImpl{
		Client:                      c,
		restMapper:                  restMapper,
		informerManager:             informerManager,
		clusterDynamicClientSetFunc: clusterDynamicClientSetFunc,
		clusterCacheSyncTimeout:     clusterCacheSyncTimeout,
	}
}

type memberClusterInformerImpl struct {
	client.Client               // used to get Cluster and Secret resources.
	restMapper                  meta.RESTMapper
	informerManager             genericmanager.MultiClusterInformerManager
	clusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
	clusterCacheSyncTimeout     metav1.Duration
}

// BuildResourceInformers builds informer dynamically for managed resources in member cluster.
// The created informer watches resource change and then sync to the relevant Work object.
func (m *memberClusterInformerImpl) BuildResourceInformers(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work, handler cache.ResourceEventHandler) error {
	err := m.registerInformersAndStart(cluster, work, handler)
	if err != nil {
		klog.Errorf("Failed to register informer for Work %s/%s. Error: %v.", work.GetNamespace(), work.GetName(), err)
		return err
	}
	return nil
}

// registerInformersAndStart builds informer manager for cluster if it doesn't exist, then constructs informers for gvr
// and start it.
func (m *memberClusterInformerImpl) registerInformersAndStart(cluster *clusterv1alpha1.Cluster, work *workv1alpha1.Work, handler cache.ResourceEventHandler) error {
	singleClusterInformerManager, err := m.getSingleClusterManager(cluster)
	if err != nil {
		return err
	}

	gvrTargets, err := m.getGVRsFromWork(work)
	if err != nil {
		return err
	}

	allSynced := true
	for _, gvr := range gvrTargets {
		if !singleClusterInformerManager.IsInformerSynced(gvr) || !singleClusterInformerManager.IsHandlerExist(gvr, handler) {
			allSynced = false
			singleClusterInformerManager.ForResource(gvr, handler)
		}
	}
	if allSynced {
		return nil
	}

	m.informerManager.Start(cluster.Name)

	if err := func() error {
		synced := m.informerManager.WaitForCacheSyncWithTimeout(cluster.Name, m.clusterCacheSyncTimeout.Duration)
		if synced == nil {
			return fmt.Errorf("no informerFactory for cluster %s exist", cluster.Name)
		}
		for _, gvr := range gvrTargets {
			if !synced[gvr] {
				return fmt.Errorf("informer for %s hasn't synced", gvr)
			}
		}
		return nil
	}(); err != nil {
		klog.Errorf("Failed to sync cache for cluster: %s, error: %v", cluster.Name, err)
		m.informerManager.Stop(cluster.Name)
		return err
	}

	return nil
}

// getSingleClusterManager gets singleClusterInformerManager with clusterName.
// If manager is not exist, create it, otherwise gets it from map.
func (m *memberClusterInformerImpl) getSingleClusterManager(cluster *clusterv1alpha1.Cluster) (genericmanager.SingleClusterInformerManager, error) {
	// TODO(chenxianpao): If cluster A is removed, then a new cluster that name also is A joins karmada,
	//  the cache in informer manager should be updated.
	singleClusterInformerManager := m.informerManager.GetSingleClusterManager(cluster.Name)
	if singleClusterInformerManager == nil {
		dynamicClusterClient, err := m.clusterDynamicClientSetFunc(cluster.Name, m.Client)
		if err != nil {
			klog.Errorf("Failed to build dynamic cluster client for cluster %s.", cluster.Name)
			return nil, err
		}
		singleClusterInformerManager = m.informerManager.ForCluster(dynamicClusterClient.ClusterName, dynamicClusterClient.DynamicClientSet, 0)
	}
	return singleClusterInformerManager, nil
}

// getGVRsFromWork traverses the manifests in work to find groupVersionResource list.
func (m *memberClusterInformerImpl) getGVRsFromWork(work *workv1alpha1.Work) ([]schema.GroupVersionResource, error) {
	gvrTargets := sets.New[schema.GroupVersionResource]()
	for _, manifest := range work.Spec.Workload.Manifests {
		workload := &unstructured.Unstructured{}
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload. Error: %v.", err)
			return nil, err
		}
		gvr, err := restmapper.GetGroupVersionResource(m.restMapper, workload.GroupVersionKind())
		if err != nil {
			klog.Errorf("Failed to get GVR from GVK for resource %s/%s. Error: %v.", workload.GetNamespace(), workload.GetName(), err)
			return nil, err
		}
		gvrTargets.Insert(gvr)
	}
	return gvrTargets.UnsortedList(), nil
}

// GetObjectFromCache returns object in cache by federated key.
func (m *memberClusterInformerImpl) GetObjectFromCache(fedKey keys.FederatedKey) (*unstructured.Unstructured, error) {
	return helper.GetObjectFromCache(m.restMapper, m.informerManager, fedKey)
}
