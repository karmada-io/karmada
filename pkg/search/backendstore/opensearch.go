package backendstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

var defaultPrefix = "kubernetes"

var mapping = `
{
    "settings":{
        "index":{
            "number_of_shards":1,
            "number_of_replicas":0
        }
    },
    "mappings":{
        "properties":{
            "apiVersion":{
                "type":"text"
            },
            "kind":{
                "type":"text"
            },
            "metadata":{
                "properties":{
                    "annotations":{
                        "type":"object",
                        "enabled":false
                    },
                    "creationTimestamp":{
                        "type":"text"
                    },
                    "deletionTimestamp":{
                        "type":"text"
                    },
                    "labels":{
                        "type":"object",
                        "enabled":false
                    },
                    "name":{
                        "type":"text",
                        "fields":{
                            "keyword":{
                                "type":"keyword",
                                "ignore_above":256
                            }
                        }
                    },
                    "namespace":{
                        "type":"text",
                        "fields":{
                            "keyword":{
                                "type":"keyword",
                                "ignore_above":256
                            }
                        }
                    },
                    "ownerReferences":{
                        "type":"text"
                    },
                    "resourceVersion":{
                        "type":"text",
                        "fields":{
                            "keyword":{
                                "type":"keyword",
                                "ignore_above":256
                            }
                        }
                    }
                }
            },
            "spec":{
                "type":"object",
                "enabled":false
            },
            "status":{
                "type":"object",
                "enabled":false
            }
        }
    }
}
`

// OpenSearch implements backendstore.BackendStore
type OpenSearch struct {
	cluster string
	client  *opensearch.Client
	indices map[string]struct{}
	l       sync.Mutex
}

// NewOpenSearch returns a new OpenSearch
func NewOpenSearch(cluster string, cfg *searchv1alpha1.BackendStoreConfig) (*OpenSearch, error) {
	klog.Infof("create openserch backend store: %s", cluster)
	os := &OpenSearch{
		cluster: cluster,
		indices: make(map[string]struct{})}

	if err := os.initClient(cfg); err != nil {
		return nil, fmt.Errorf("cannot init client: %v", err)
	}

	return os, nil
}

// ResourceEventHandlerFuncs implements cache.ResourceEventHandler
func (os *OpenSearch) ResourceEventHandlerFuncs() cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			os.upsert(obj)
		},
		UpdateFunc: func(oldObj, curObj interface{}) {
			os.upsert(curObj)
		},
		DeleteFunc: func(obj interface{}) {
			os.delete(obj)
		},
	}
}

// Close the client
func (os *OpenSearch) Close() {}

// TODO: bulk delete
func (os *OpenSearch) delete(obj interface{}) {
	us, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("unexpected type %T", obj)
		return
	}

	indexName, err := os.indexName(us)
	if err != nil {
		klog.Errorf("cannot get index name: %v", err)
		return
	}

	deleteRequest := opensearchapi.DeleteRequest{
		Index:      indexName,
		DocumentID: string(us.GetUID()),
	}

	resp, err := deleteRequest.Do(context.Background(), os.client)
	if err != nil {
		klog.Errorf("cannot delete: %v", err)
		return
	}
	klog.V(4).Infof("Delete response: %v", resp.String())
}

// TODO: bulk upsert
func (os *OpenSearch) upsert(obj interface{}) {
	us, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("unexpected type %T", obj)
		return
	}

	us = us.DeepCopy()
	annotations := us.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[clusterv1alpha1.CacheSourceAnnotationKey] = os.cluster
	us.SetAnnotations(annotations)

	doc := map[string]interface{}{
		"apiVersion": us.GetAPIVersion(),
		"kind":       us.GetKind(),
		"metadata": map[string]interface{}{
			"name":              us.GetName(),
			"namespace":         us.GetNamespace(),
			"creationTimestamp": us.GetCreationTimestamp().Format(time.RFC3339),
			"labels":            us.GetLabels(),
			"annotations":       us.GetAnnotations(),
			"deletionTimestamp": us.GetDeletionTimestamp(),
		},
	}

	spec, _ := json.Marshal(us.Object["spec"])
	status, _ := json.Marshal(us.Object["status"])
	doc["spec"] = string(spec)
	doc["status"] = string(status)

	body, err := json.Marshal(doc)
	if err != nil {
		klog.Errorf("Cannot marshal to json: %v", err)
		return
	}

	indexName, err := os.indexName(us)
	if err != nil {
		klog.Errorf("Cannot get index name: %v", err)
		return
	}

	req := opensearchapi.IndexRequest{
		Index:      indexName,
		DocumentID: string(us.GetUID()),
		Body:       strings.NewReader(string(body)),
	}
	resp, err := req.Do(context.Background(), os.client)
	if err != nil {
		klog.Errorf("Cannot upsert: %v", err)
		return
	}
	if resp.IsError() {
		klog.Errorf("Upsert error: %s", resp.String())
		return
	}
	klog.V(4).Infof("Upsert response: %s", resp.String())
}

// TODO: apply mapping
func (os *OpenSearch) indexName(us *unstructured.Unstructured) (string, error) {
	name := fmt.Sprintf("%s-%s", defaultPrefix, strings.ToLower(us.GetKind()))
	os.l.Lock()
	defer os.l.Unlock()

	if _, ok := os.indices[name]; ok {
		return name, nil
	}

	klog.Infof("Try to create index: %s", name)
	res := opensearchapi.IndicesCreateRequest{Index: name, Body: strings.NewReader(mapping)}
	resp, err := res.Do(context.Background(), os.client)
	if err != nil {
		if strings.Contains(err.Error(), "resource_already_exists_exception") {
			klog.Info("Index already exists")
			os.indices[name] = struct{}{}
			return name, nil
		}
		return name, fmt.Errorf("cannot create index: %v", err)
	}
	if resp.IsError() {
		if strings.Contains(resp.String(), "resource_already_exists_exception") {
			klog.Info("Index already exists")
			os.indices[name] = struct{}{}
			return name, nil
		}
		return name, fmt.Errorf("cannot create index (resp): %v", resp.String())
	}

	klog.Infof("create index response: %s", resp.String())
	os.indices[name] = struct{}{}

	return name, nil
}

func (os *OpenSearch) initClient(bsc *searchv1alpha1.BackendStoreConfig) error {
	if bsc == nil || bsc.OpenSearch == nil {
		return errors.New("opensearch config is nil")
	}

	if len(bsc.OpenSearch.Addresses) == 0 {
		return errors.New("not found opensearch address")
	}
	cfg := opensearch.Config{Addresses: bsc.OpenSearch.Addresses}

	user, pwd := func(secretRef clusterv1alpha1.LocalSecretReference) (user, pwd string) {
		if secretRef.Namespace == "" || secretRef.Name == "" {
			klog.Warningf("Not found secret for opensearch, try to without auth")
			return
		}

		secret, err := k8sClient.CoreV1().Secrets(secretRef.Namespace).Get(context.TODO(), secretRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Can not get secret %s/%s: %v, try to without auth", secretRef.Namespace, secretRef.Name, err)
			return
		}

		return string(secret.Data["username"]), string(secret.Data["password"])
	}(bsc.OpenSearch.SecretRef)

	if user != "" {
		cfg.Username = user
		cfg.Password = pwd
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("cannot create opensearch client: %v", err)
	}

	info, err := client.Info()
	if err != nil {
		return fmt.Errorf("cannot get opensearch info: %v", err)
	}

	klog.V(4).Infof("Opensearch client: %v", info)
	os.client = client
	return nil
}
