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
	// AddRateLimited adds item to queue.
	AddRateLimited(item interface{})
	// EnqueueRateLimited generates the key for objects then adds the key as an item to queue.
	EnqueueRateLimited(obj runtime.Object)
	Run(workerNumber int, stopChan <-chan struct{})
}

// QueueKey is the item key that stores in queue.
// The key could be arbitrary types.
//
// In some cases, people would like store different resources in a same queue, the traditional full-qualified key,
// such as '<namespace>/<name>', can't distinguish which resource the key belongs to, the key might carry more information
// of a resource, such as GVK(Group Version Kind), in that cases people need to use self-defined key, e.g. a struct.
type QueueKey interface{}

// KeyFunc knows how to make a key from an object. Implementations should be deterministic.
type KeyFunc func(obj interface{}) (QueueKey, error)

// ReconcileFunc knows how to consume items(key) from the queue.
type ReconcileFunc func(key QueueKey) error

type asyncWorker struct {
	// keyFunc is the function that make keys for API objects.
	keyFunc KeyFunc
	// reconcileFunc is the function that process keys from the queue.
	reconcileFunc ReconcileFunc
	// queue allowing parallel processing of resources.
	queue workqueue.RateLimitingInterface
	// interval is the interval for process object in the queue.
	interval time.Duration
}

// NewAsyncWorker returns a asyncWorker which can process resource periodic.
func NewAsyncWorker(name string, interval time.Duration, keyFunc KeyFunc, reconcileFunc ReconcileFunc) AsyncWorker {
	return &asyncWorker{
		keyFunc:       keyFunc,
		reconcileFunc: reconcileFunc,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		interval:      interval,
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
func GenerateKey(obj interface{}) (QueueKey, error) {
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
	// it happens when the obj not managed by Karmada.
	if cluster == "" {
		return nil, nil
	}
	return cluster + "/" + gvk.Group + "/" + gvk.Version + "/" + gvk.Kind + "/" + key, nil
}

// getClusterNameFromLabel gets cluster name from ownerLabel, if label not exist, means resource is not created by karmada.
func getClusterNameFromLabel(resource *unstructured.Unstructured) (string, error) {
	workNamespace := GetLabelValue(resource.GetLabels(), WorkNamespaceLabel)
	if len(workNamespace) == 0 {
		klog.V(4).Infof("Ignore resource(%s/%s/%s) which not managed by karmada", resource.GetKind(), resource.GetNamespace(), resource.GetName())
		return "", nil
	}

	cluster, err := names.GetClusterName(workNamespace)
	if err != nil {
		klog.Errorf("Failed to get cluster name from work namespace: %s, error: %v.", workNamespace, err)
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

func (w *asyncWorker) EnqueueRateLimited(obj runtime.Object) {
	key, err := w.keyFunc(obj)
	if err != nil {
		klog.Warningf("Failed to generate key for obj: %s", obj.GetObjectKind().GroupVersionKind())
		return
	}

	w.AddRateLimited(key)
}

func (w *asyncWorker) AddRateLimited(item interface{}) {
	if item == nil {
		klog.Warningf("Ignore nil item from queue")
		return
	}

	w.queue.AddRateLimited(item)
}

func (w *asyncWorker) handleError(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		w.queue.Forget(key)
		return
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

	err := w.reconcileFunc(key)
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
