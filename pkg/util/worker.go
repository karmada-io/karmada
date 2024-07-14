/*
Copyright 2020 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

// AsyncWorker maintains a rate limiting queue and the items in the queue will be reconciled by a "ReconcileFunc".
// The item will be re-queued if "ReconcileFunc" returns an error, maximum re-queue times defined by "maxRetries" above,
// after that the item will be discarded from the queue.
type AsyncWorker interface {
	// Add adds the 'item' to queue immediately(without any delay).
	Add(item interface{})

	// AddAfter adds an item to the workqueue after the indicated duration has passed
	AddAfter(item interface{}, duration time.Duration)

	// Enqueue generates the key of 'obj' according to a 'KeyFunc' then adds the key as an item to queue by 'Add'.
	Enqueue(obj interface{})

	// Run starts a certain number of concurrent workers to reconcile the items and will never stop until 'stopChan'
	// is closed.
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
}

// Options are the arguments for creating a new AsyncWorker.
type Options struct {
	// Name is the queue's name that will be used to emit metrics.
	// Defaults to "", which means disable metrics.
	Name               string
	KeyFunc            KeyFunc
	ReconcileFunc      ReconcileFunc
	RateLimiterOptions ratelimiterflag.Options
}

// NewAsyncWorker returns a asyncWorker which can process resource periodic.
func NewAsyncWorker(opt Options) AsyncWorker {
	return &asyncWorker{
		keyFunc:       opt.KeyFunc,
		reconcileFunc: opt.ReconcileFunc,
		queue: workqueue.NewRateLimitingQueueWithConfig(ratelimiterflag.DefaultControllerRateLimiter(opt.RateLimiterOptions), workqueue.RateLimitingQueueConfig{
			Name: opt.Name,
		}),
	}
}

func (w *asyncWorker) Enqueue(obj interface{}) {
	key, err := w.keyFunc(obj)
	if err != nil {
		klog.Errorf("Failed to generate key for obj: %+v, err: %v", obj, err)
		return
	}

	if key == nil {
		return
	}

	w.Add(key)
}

func (w *asyncWorker) Add(item interface{}) {
	if item == nil {
		klog.Warningf("Ignore nil item from queue")
		return
	}

	w.queue.Add(item)
}

func (w *asyncWorker) AddAfter(item interface{}, duration time.Duration) {
	if item == nil {
		klog.Warningf("Ignore nil item from queue")
		return
	}

	w.queue.AddAfter(item, duration)
}

func (w *asyncWorker) worker() {
	key, quit := w.queue.Get()
	if quit {
		return
	}
	defer w.queue.Done(key)

	err := w.reconcileFunc(key)
	if err != nil {
		w.queue.AddRateLimited(key)
		return
	}

	w.queue.Forget(key)
}

func (w *asyncWorker) Run(workerNumber int, stopChan <-chan struct{}) {
	for i := 0; i < workerNumber; i++ {
		go wait.Until(w.worker, 0, stopChan)
	}
	// Ensure all goroutines are cleaned up when the stop channel closes
	go func() {
		<-stopChan
		w.queue.ShutDown()
	}()
}

// MetaNamespaceKeyFunc generates a namespaced key for object.
func MetaNamespaceKeyFunc(obj interface{}) (QueueKey, error) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		return nil, err
	}
	return key, nil
}
