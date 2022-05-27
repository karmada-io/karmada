package storage

import (
	"encoding/json"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"
)

type errorResponse struct {
	Error string `json:"error"`
}

type reqResponse struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []runtime.Object `json:"items"`
}

func (r *SearchREST) newCacheHandler(info *genericrequest.RequestInfo, responder rest.Responder) (http.Handler, error) {
	resourceGVR := schema.GroupVersionResource{
		Group:    info.APIGroup,
		Version:  info.APIVersion,
		Resource: info.Resource,
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		enc := json.NewEncoder(rw)

		clusters, err := r.clusterLister.List(labels.Everything())
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_ = enc.Encode(errorResponse{Error: fmt.Sprintf("Failed to list clusters: %v", err)})
			return
		}

		items := make([]runtime.Object, 0)
		for _, cluster := range clusters {
			singleClusterManger := r.multiClusterInformerManager.GetSingleClusterManager(cluster.Name)
			if singleClusterManger == nil {
				klog.Warningf("SingleClusterInformerManager for cluster(%s) is nil.", cluster.Name)
				continue
			}

			switch {
			case len(info.Namespace) > 0 && len(info.Name) > 0:
				resourceObject, err := singleClusterManger.Lister(resourceGVR).ByNamespace(info.Namespace).Get(info.Name)
				if err != nil {
					klog.Errorf("Failed to get %s resource(%s/%s) from cluster(%s)'s informer cache.",
						resourceGVR, info.Namespace, info.Name, cluster.Name)
				}
				items = append(items, addAnnotationWithClusterName([]runtime.Object{resourceObject}, cluster.Name)...)
			case len(info.Namespace) > 0:
				resourceObjects, err := singleClusterManger.Lister(resourceGVR).ByNamespace(info.Namespace).List(labels.Everything())
				if err != nil {
					klog.Errorf("Failed to list %s resource under namespace(%s) from cluster(%s)'s informer cache.",
						resourceGVR, info.Namespace, cluster.Name)
				}
				items = append(items, addAnnotationWithClusterName(resourceObjects, cluster.Name)...)
			case len(info.Name) > 0:
				resourceObject, err := singleClusterManger.Lister(resourceGVR).Get(info.Name)
				if err != nil {
					klog.Errorf("Failed to get %s resource(%s) from cluster(%s)'s informer cache.",
						resourceGVR, info.Name, cluster.Name)
				}
				items = append(items, addAnnotationWithClusterName([]runtime.Object{resourceObject}, cluster.Name)...)
			default:
				resourceObjects, err := singleClusterManger.Lister(resourceGVR).List(labels.Everything())
				if err != nil {
					klog.Errorf("Failed to list %s resource from cluster(%s)'s informer cache.",
						resourceGVR, cluster.Name)
				}
				items = append(items, addAnnotationWithClusterName(resourceObjects, cluster.Name)...)
			}
		}

		rr := reqResponse{}
		rr.APIVersion = fmt.Sprintf("%s/%s", info.APIGroup, info.APIVersion)
		rr.Kind = "List"
		rr.Items = items
		_ = enc.Encode(rr)
	}), nil
}

func addAnnotationWithClusterName(resourceObjects []runtime.Object, clusterName string) []runtime.Object {
	resources := make([]runtime.Object, 0)
	for index := range resourceObjects {
		resource := resourceObjects[index].(*unstructured.Unstructured)

		annotations := resource.GetAnnotations()
		annotations["cluster.karmada.io/name"] = clusterName

		resource.SetAnnotations(annotations)
		resources = append(resources, resource)
	}

	return resources
}
