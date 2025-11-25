/*
Copyright 2022 The Karmada Authors.

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
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

func newTestAsyncWorker(reconcileFunc ReconcileFunc, usePriorityQueue bool) *asyncWorker {
	options := Options{
		Name:          "test_async_worker",
		KeyFunc:       MetaNamespaceKeyFunc,
		ReconcileFunc: reconcileFunc,
		RateLimiterOptions: ratelimiterflag.Options{
			RateLimiterBaseDelay:  time.Millisecond,
			RateLimiterMaxDelay:   10 * time.Millisecond,
			RateLimiterQPS:        5000,
			RateLimiterBucketSize: 100,
		},
		UsePriorityQueue: usePriorityQueue,
	}
	worker := NewAsyncWorker(options)

	return worker.(*asyncWorker)
}

func Test_asyncWorker_Enqueue(t *testing.T) {
	const name = "fake_node"

	worker := newTestAsyncWorker(nil, false)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	worker.Enqueue(node)

	item, _ := worker.queue.Get()

	if name != item {
		t.Errorf("Added Item: %v, want: %v", item, name)
	}
}

func Test_asyncPriorityWorker_Enqueue(t *testing.T) {
	const name = "fake_node"

	worker := newTestAsyncWorker(nil, true)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}

	worker.Enqueue(node)

	item, _ := worker.queue.Get()

	if name != item {
		t.Errorf("Added Item: %v, want: %v", item, name)
	}
}

func Test_asyncWorker_AddAfter(t *testing.T) {
	const name = "fake_node"
	const duration = 1 * time.Second

	worker := newTestAsyncWorker(nil, false)

	start := time.Now()
	worker.AddAfter(name, duration)

	item, _ := worker.queue.Get()
	end := time.Now()

	if name != item {
		t.Errorf("Added Item: %v, want: %v", item, name)
	}

	elapsed := end.Sub(start)
	if elapsed < duration {
		t.Errorf("Added Item should be dequeued after %v, but the actually elapsed time is %v.",
			duration.String(), elapsed.String())
	}
}

func Test_asyncPriorityWorker_AddAfter(t *testing.T) {
	const name = "fake_node"
	const duration = 1 * time.Second

	worker := newTestAsyncWorker(nil, true)

	start := time.Now()
	worker.AddAfter(name, duration)

	item, _ := worker.queue.Get()
	end := time.Now()

	if name != item {
		t.Errorf("Added Item: %v, want: %v", item, name)
	}

	elapsed := end.Sub(start)
	if elapsed < duration {
		t.Errorf("Added Item should be dequeued after %v, but the actually elapsed time is %v.",
			duration.String(), elapsed.String())
	}
}

type asyncWorkerReconciler struct {
	lock sync.Mutex
	// guarded by lock
	shouldPass map[int]struct{}
}

func newAsyncWorkerReconciler() *asyncWorkerReconciler {
	return &asyncWorkerReconciler{
		shouldPass: make(map[int]struct{}),
	}
}

func (a *asyncWorkerReconciler) ReconcileFunc(key QueueKey) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	v := key.(int)
	if _, ok := a.shouldPass[v]; !ok {
		// every item retry once
		a.shouldPass[v] = struct{}{}
		return errors.New("fail once")
	}

	return nil
}

func (a *asyncWorkerReconciler) ProcessedItem() map[int]struct{} {
	a.lock.Lock()
	defer a.lock.Unlock()

	ret := make(map[int]struct{}, len(a.shouldPass))
	for k, v := range a.shouldPass {
		ret[k] = v
	}

	return ret
}

func Test_asyncWorker_Run(t *testing.T) {
	const cnt = 2000

	reconcile := newAsyncWorkerReconciler()
	worker := newTestAsyncWorker(reconcile.ReconcileFunc, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Run(ctx, 5)

	for i := 0; i < cnt; i++ {
		worker.Add(i)
	}

	err := assertUntil(20*time.Second, func() error {
		processed := reconcile.ProcessedItem()
		if len(processed) < cnt {
			return fmt.Errorf("processed item not equal to input, len() = %v, processed item is %v",
				len(processed), processed)
		}

		for i := 0; i < cnt; i++ {
			if _, ok := processed[i]; !ok {
				return fmt.Errorf("expected item not processed, expected: %v, all processed item: %v",
					i, processed)
			}
		}

		return nil
	})

	if err != nil {
		t.Error(err.Error())
	}
}

func Test_asyncPriorityWorker_Run(t *testing.T) {
	const cnt = 2000

	reconcile := newAsyncWorkerReconciler()
	worker := newTestAsyncWorker(reconcile.ReconcileFunc, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Run(ctx, 5)

	for i := 0; i < cnt; i++ {
		worker.Add(i)
	}

	err := assertUntil(20*time.Second, func() error {
		processed := reconcile.ProcessedItem()
		if len(processed) < cnt {
			return fmt.Errorf("processed item not equal to input, len() = %v, processed item is %v",
				len(processed), processed)
		}

		for i := 0; i < cnt; i++ {
			if _, ok := processed[i]; !ok {
				return fmt.Errorf("expected item not processed, expected: %v, all processed item: %v",
					i, processed)
			}
		}

		return nil
	})

	if err != nil {
		t.Error(err.Error())
	}
}

// Block running assertion func periodically to check if condition match.
// Fail if: maxDuration is reached
// Success if: assertion return nil error
// return: non-nil error if failed. nil error if succeed
func assertUntil(maxDuration time.Duration, assertion func() error) error {
	start := time.Now()
	var lastErr error

	for {
		lastErr = assertion()
		if lastErr == nil {
			return nil
		}

		// Make sure assertion() is called at least one time.
		if time.Since(start) >= maxDuration {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	return lastErr
}

func Test_asyncWorker_AddWithOpts(t *testing.T) {
	t.Run("AddAfter, pq not enabled", func(t *testing.T) {
		const name = "fake_node"
		const duration = 1 * time.Second

		worker := newTestAsyncWorker(nil, false)

		start := time.Now()
		worker.AddWithOpts(AddOpts{After: duration}, name)

		item, _ := worker.queue.Get()
		end := time.Now()

		if name != item {
			t.Errorf("Added Item: %v, want: %v", item, name)
		}

		elapsed := end.Sub(start)
		if elapsed < duration {
			t.Errorf("Added Item should be dequeued after %v, but the actually elapsed time is %v.",
				duration.String(), elapsed.String())
		}
	})

	t.Run("addAfter, pq enabled", func(t *testing.T) {
		const name = "fake_node"
		const duration = 1 * time.Second

		worker := newTestAsyncWorker(nil, true)

		start := time.Now()
		worker.AddWithOpts(AddOpts{After: duration}, name)

		item, _ := worker.queue.Get()
		end := time.Now()

		if name != item {
			t.Errorf("Added Item: %v, want: %v", item, name)
		}

		elapsed := end.Sub(start)
		if elapsed < duration {
			t.Errorf("Added Item should be dequeued after %v, but the actually elapsed time is %v.",
				duration.String(), elapsed.String())
		}
	})

	t.Run("test FIFO with priority", func(t *testing.T) {
		const node1 = "fake_node1"
		const node2 = "fake_node2"
		const node3 = "fake_node3"
		const node4 = "fake_node4"
		const node5 = "fake_node5"

		worker := newTestAsyncWorker(nil, true)

		worker.AddWithOpts(AddOpts{Priority: ptr.To(LowPriority)}, node1)
		worker.AddWithOpts(AddOpts{}, node2)
		worker.AddWithOpts(AddOpts{Priority: ptr.To(LowPriority)}, node3)
		worker.AddWithOpts(AddOpts{}, node3)
		worker.AddWithOpts(AddOpts{Priority: ptr.To(LowPriority)}, node4)
		worker.AddWithOpts(AddOpts{}, node5)

		item, _ := worker.queue.Get()
		if node2 != item {
			t.Errorf("Added Item: %v, want: %v", item, node2)
		}

		item, _ = worker.queue.Get()
		if node3 != item {
			t.Errorf("Added Item: %v, want: %v", item, node3)
		}

		item, _ = worker.queue.Get()
		if node5 != item {
			t.Errorf("Added Item: %v, want: %v", item, node5)
		}

		item, _ = worker.queue.Get()
		if node1 != item {
			t.Errorf("Added Item: %v, want: %v", item, node1)
		}

		item, _ = worker.queue.Get()
		if node4 != item {
			t.Errorf("Added Item: %v, want: %v", item, node4)
		}
	})
}

func Test_asyncWorker_EnqueueWithOpts(t *testing.T) {
	t.Run("low priority", func(t *testing.T) {
		const nodeName1 = "fake_node1"
		const nodeName2 = "fake_node2"

		worker := newTestAsyncWorker(nil, true)

		node1 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName1},
		}
		worker.EnqueueWithOpts(AddOpts{Priority: ptr.To(LowPriority)}, node1)
		node2 := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: nodeName2},
		}
		worker.EnqueueWithOpts(AddOpts{}, node2)

		item, _ := worker.queue.Get()

		if nodeName2 != item {
			t.Errorf("Added Item: %v, want: %v", item, nodeName2)
		}
		item, _ = worker.queue.Get()
		if nodeName1 != item {
			t.Errorf("Added Item: %v, want: %v", item, nodeName1)
		}
	})
}
