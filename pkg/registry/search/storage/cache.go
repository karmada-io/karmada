package storage

import (
	"encoding/json"
	"fmt"
	"net/http"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metainternalversionvalidation "k8s.io/apimachinery/pkg/apis/meta/internalversion/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchscheme "github.com/karmada-io/karmada/pkg/apis/search/scheme"
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

		opts := metainternalversion.ListOptions{}
		if err := searchscheme.ParameterCodec.DecodeParameters(req.URL.Query(), metav1.SchemeGroupVersion, &opts); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			klog.Errorf("Failed to decode parameters from req.URL.Query(): %v", err)
			_ = enc.Encode(errorResponse{Error: err.Error()})
			return
		}

		if errs := metainternalversionvalidation.ValidateListOptions(&opts); len(errs) > 0 {
			rw.WriteHeader(http.StatusBadRequest)
			klog.Errorf("Invalid decoded ListOptions: %v.", errs)
			_ = enc.Encode(errorResponse{Error: errs.ToAggregate().Error()})
			return
		}

		label := labels.Everything()
		if opts.LabelSelector != nil {
			label = opts.LabelSelector
		}

		clusters, err := r.clusterLister.List(labels.Everything())
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_ = enc.Encode(errorResponse{Error: fmt.Sprintf("Failed to list clusters: %v", err)})
			return
		}

		// TODO: process opts.Limit to prevent the client from being unable to process the response
		// due to the large size of the response body.
		objItems := r.getObjectItemsFromClusters(clusters, resourceGVR, info.Namespace, info.Name, label)
		rr := reqResponse{
			TypeMeta: metav1.TypeMeta{
				APIVersion: resourceGVR.GroupVersion().String(),
				Kind:       "List", // TODO: obtains the kind type of the actual resource list.
			},
			Items: objItems,
		}
		_ = enc.Encode(rr)
	}), nil
}

func (r *SearchREST) getObjectItemsFromClusters(
	clusters []*clusterv1alpha1.Cluster,
	objGVR schema.GroupVersionResource,
	namespace, name string,
	label labels.Selector) []runtime.Object {
	items := make([]runtime.Object, 0)

	// TODO: encapsulating the Interface for Obtaining resource from the Multi-Cluster Cache.
	for _, cluster := range clusters {
		singleClusterManger := r.multiClusterInformerManager.GetSingleClusterManager(cluster.Name)
		if singleClusterManger == nil {
			klog.Warningf("SingleClusterInformerManager for cluster(%s) is nil.", cluster.Name)
			continue
		}

		var err error
		objLister := singleClusterManger.Lister(objGVR)
		if len(name) > 0 {
			var resourceObject runtime.Object
			if len(namespace) > 0 {
				resourceObject, err = objLister.ByNamespace(namespace).Get(name)
			} else {
				resourceObject, err = objLister.Get(name)
			}
			if err != nil {
				klog.Errorf("Failed to get %s resource(%s/%s) from cluster(%s)'s informer cache: %v",
					objGVR, namespace, name, cluster.Name, err)
				continue
			}
			items = append(items, addAnnotationWithClusterName([]runtime.Object{resourceObject}, cluster.Name)...)
		} else {
			var resourceObjects []runtime.Object
			if len(namespace) > 0 {
				resourceObjects, err = objLister.ByNamespace(namespace).List(label)
			} else {
				resourceObjects, err = objLister.List(label)
			}
			if err != nil {
				klog.Errorf("Failed to list %s resource from cluster(%s)'s informer cache: %v",
					objGVR, cluster.Name, err)
				continue
			}
			items = append(items, addAnnotationWithClusterName(resourceObjects, cluster.Name)...)
		}
	}

	return items
}

func addAnnotationWithClusterName(resourceObjects []runtime.Object, clusterName string) []runtime.Object {
	resources := make([]runtime.Object, 0)
	for index := range resourceObjects {
		resource := resourceObjects[index].(*unstructured.Unstructured)

		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		// TODO: move this annotation key `cluster.karmada.io/name` to the Cluster API.
		annotations["cluster.karmada.io/name"] = clusterName

		resource.SetAnnotations(annotations)
		resources = append(resources, resource)
	}

	return resources
}
