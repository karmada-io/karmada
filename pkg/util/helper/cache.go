package helper

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

type clusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)

// GetObjectFromCache gets full object information from cache by key in worker queue.
func GetObjectFromCache(restMapper meta.RESTMapper, manager informermanager.MultiClusterInformerManager, fedKey keys.FederatedKey,
	client client.Client, clientSetFunc clusterDynamicClientSetFunc) (*unstructured.Unstructured, error) {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, fedKey.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s. Error: %v", fedKey.GroupVersionKind(), err)
		return nil, err
	}

	singleClusterManager := manager.GetSingleClusterManager(fedKey.Cluster)
	if singleClusterManager == nil {
		// That may happen in case of multi-controllers sharing one informer. For example:
		// controller-A takes responsibility of initialize informer for clusters, but controller-B consumes
		// the informer before the initialization.
		// Usually this error will be eliminated during the controller reconciling loop.
		return nil, fmt.Errorf("the informer of cluster(%s) has not been initialized", fedKey.Cluster)
	}

	if !singleClusterManager.IsInformerSynced(gvr) {
		// fall back to call api server in case the cache has not been synchronized yet
		return getObjectFromMemberCluster(gvr, fedKey, client, clientSetFunc)
	}

	var obj runtime.Object
	lister := singleClusterManager.Lister(gvr)
	obj, err = lister.Get(fedKey.NamespaceKey())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}

		// print logs only for real error.
		klog.Errorf("Failed to get obj %s. error: %v.", fedKey.String(), err)
		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}

// getObjectFromMemberCluster will try to get resource from member cluster by DynamicClientSet.
func getObjectFromMemberCluster(gvr schema.GroupVersionResource, fedKey keys.FederatedKey, client client.Client,
	clientSetFunc clusterDynamicClientSetFunc) (*unstructured.Unstructured, error) {
	dynamicClusterClient, err := clientSetFunc(fedKey.Cluster, client)
	if err != nil {
		klog.Errorf("Failed to build dynamic cluster client for cluster %s.", fedKey.Cluster)
		return nil, err
	}

	existObj, err := dynamicClusterClient.DynamicClientSet.Resource(gvr).Namespace(fedKey.Namespace).Get(context.TODO(),
		fedKey.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return existObj, nil
}
