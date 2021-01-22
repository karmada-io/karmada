package util

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// maxRetries is the number of times a resource will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a resource is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

// AsyncWorker is a worker to process resources periodic with a rateLimitingQueue.
type AsyncWorker interface {
	EnqueueRateLimited(obj runtime.Object)
	Run(workerNumber int, stopChan <-chan struct{})
}

// ReconcileHandler is a callback function for process resources.
type ReconcileHandler func(key string) error

type asyncWorker struct {
	// reconcile is callback function to process object in the queue.
	reconcile ReconcileHandler
	// queue allowing parallel processing of resources.
	queue workqueue.RateLimitingInterface
	// interval is the interval for process object in the queue.
	interval time.Duration
}

// NewAsyncWorker returns a asyncWorker which can process resource periodic.
func NewAsyncWorker(reconcile ReconcileHandler, name string, interval time.Duration) AsyncWorker {
	return &asyncWorker{
		reconcile: reconcile,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		interval:  interval,
	}
}

// ClusterWorkload is the thumbnail of cluster workload, it contains GVK, cluster, namespace and name.
type ClusterWorkload struct {
	GVK       schema.GroupVersionKind
	Cluster   string
	Namespace string
	Name      string
}

// GetListerKey returns the key that can be used to query full object information by GenericLister
func (w *ClusterWorkload) GetListerKey() string {
	if w.Namespace == "" {
		return w.Name
	}
	return w.Namespace + "/" + w.Name
}

// GenerateKey generates a key from obj, the key contains cluster, GVK, namespace and name.
func GenerateKey(obj runtime.Object) (string, error) {
	resource := obj.(*unstructured.Unstructured)
	gvk := schema.FromAPIVersionAndKind(resource.GetAPIVersion(), resource.GetKind())
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v.", obj, err)
		return "", err
	}
	cluster, err := getClusterNameFromLabel(resource)
	if err != nil {
		return "", err
	}
	if cluster == "" {
		return "", nil
	}
	return cluster + "/" + gvk.Group + "/" + gvk.Version + "/" + gvk.Kind + "/" + key, nil
}

// getClusterNameFromLabel gets cluster name from ownerLabel, if label not exist, means resource is not created by karmada.
func getClusterNameFromLabel(resource *unstructured.Unstructured) (string, error) {
	workloadLabels := resource.GetLabels()
	if workloadLabels == nil {
		klog.V(2).Infof("Resource %s/%s/%s is not created by karmada.", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
		return "", nil
	}
	value, exist := workloadLabels[OwnerLabel]
	if !exist {
		klog.V(2).Infof("Resource %s/%s/%s is not created by karmada.", resource.GetKind(),
			resource.GetNamespace(), resource.GetName())
		return "", nil
	}
	executionNamespace, _, err := names.GetNamespaceAndName(value)
	if err != nil {
		klog.Errorf("Failed to get executionNamespace from label %s", value)
		return "", err
	}
	cluster, err := names.GetClusterName(executionNamespace)
	if err != nil {
		klog.Errorf("Failed to get member cluster name by %s. Error: %v.", value, err)
		return "", err
	}
	return cluster, nil
}

// SplitMetaKey transforms key to struct ClusterWorkload, struct ClusterWorkload contains cluster, GVK, namespace and name.
func SplitMetaKey(key string) (ClusterWorkload, error) {
	var clusterWorkload ClusterWorkload
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 5:
		// name only, no namespace
		clusterWorkload.Name = parts[4]
	case 6:
		// namespace and name
		clusterWorkload.Namespace = parts[4]
		clusterWorkload.Name = parts[5]
	default:
		return clusterWorkload, fmt.Errorf("unexpected key format: %q", key)
	}
	clusterWorkload.Cluster = parts[0]
	clusterWorkload.GVK.Group = parts[1]
	clusterWorkload.GVK.Version = parts[2]
	clusterWorkload.GVK.Kind = parts[3]
	return clusterWorkload, nil
}

func (w *asyncWorker) processKey(obj runtime.Object) string {
	key, err := GenerateKey(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v.", obj, err)
		return ""
	}
	if key == "" {
		klog.V(2).Infof("The key is empty, object is not created by karmada.")
		return ""
	}
	return key
}

func (w *asyncWorker) EnqueueRateLimited(obj runtime.Object) {
	key := w.processKey(obj)
	if key == "" {
		return
	}
	w.queue.AddRateLimited(key)
}

func (w *asyncWorker) handleError(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		w.queue.Forget(key)
		return
	}

	_, keyErr := SplitMetaKey(key.(string))
	if keyErr != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "key", key)
	}

	if w.queue.NumRequeues(key) < maxRetries {
		w.queue.AddRateLimited(key)
		return
	}

	klog.V(2).Infof("Dropping resource %q out of the queue: %v", key, err)
	w.queue.Forget(key)
}

func (w *asyncWorker) worker() {
	key, quit := w.queue.Get()
	if quit {
		return
	}
	defer w.queue.Done(key)

	err := w.reconcile(key.(string))
	w.handleError(err, key)
}

func (w *asyncWorker) Run(workerNumber int, stopChan <-chan struct{}) {
	for i := 0; i < workerNumber; i++ {
		go wait.Until(w.worker, w.interval, stopChan)
	}
	// Ensure all goroutines are cleaned up when the stop channel closes
	go func() {
		<-stopChan
		w.queue.ShutDown()
	}()
}
