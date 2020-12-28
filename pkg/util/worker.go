package util

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// maxRetries is the number of times a resource will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a resource is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

// ReconcileWorker is a worker to process resources periodic with a rateLimitingQueue.
type ReconcileWorker interface {
	Enqueue(obj runtime.Object)
	EnqueueRateLimited(obj runtime.Object)
	EnqueueAfter(obj runtime.Object, after time.Duration)
	Run(stopChan <-chan struct{})
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

// NewReconcileWorker returns a asyncWorker which can process resource periodic.
func NewReconcileWorker(reconcile ReconcileHandler, name string, internal time.Duration) ReconcileWorker {
	return &asyncWorker{
		reconcile: reconcile,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		interval:  internal,
	}
}

func (w *asyncWorker) Enqueue(obj runtime.Object) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v.", obj, err)
		return
	}
	w.queue.Add(key)
}

func (w *asyncWorker) EnqueueRateLimited(obj runtime.Object) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v.", obj, err)
		return
	}

	w.queue.AddRateLimited(key)
}

func (w *asyncWorker) EnqueueAfter(obj runtime.Object, after time.Duration) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %#v: %v.", obj, err)
		return
	}

	w.queue.AddAfter(key, after)
}

func (w *asyncWorker) handleError(err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
		w.queue.Forget(key)
		return
	}

	_, _, keyErr := cache.SplitMetaNamespaceKey(key.(string))
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

func (w *asyncWorker) Run(stopChan <-chan struct{}) {
	go wait.Until(w.worker, w.interval, stopChan)
	// Ensure all goroutines are cleaned up when the stop channel closes
	go func() {
		<-stopChan
		w.queue.ShutDown()
	}()
}
