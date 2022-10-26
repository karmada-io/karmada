package util

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
)

func newTestAsyncWorker(reconcileFunc ReconcileFunc) *asyncWorker {
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
	}
	worker := NewAsyncWorker(options)

	return worker.(*asyncWorker)
}

func Test_asyncWorker_Enqueue(t *testing.T) {
	const name = "fake_node"

	worker := newTestAsyncWorker(nil)

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

	worker := newTestAsyncWorker(nil)

	start := time.Now()
	worker.AddAfter(name, duration)

	item, _ := worker.queue.Get()
	end := time.Now()

	if name != item {
		t.Errorf("Added Item: %v, want: %v", item, name)
	}

	elapsed := end.Sub(start)
	if elapsed < duration {
		t.Errorf("Added Item should dequeued after %v, but the actually elapsed time is %v.",
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
	worker := newTestAsyncWorker(reconcile.ReconcileFunc)

	stopChan := make(chan struct{})
	defer close(stopChan)

	worker.Run(5, stopChan)

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

type asyncWorkerReconciler2 struct {
	receivedTimes atomic.Int64
}

func (a *asyncWorkerReconciler2) ReconcileFunc(key QueueKey) error {
	a.receivedTimes.Inc()
	return errors.New("always fail")
}

func Test_asyncWorker_drop_resource(t *testing.T) {
	const name = "fake_node"
	const wantReceivedTimes = maxRetries + 1

	reconcile := new(asyncWorkerReconciler2)
	worker := newTestAsyncWorker(reconcile.ReconcileFunc)

	stopChan := make(chan struct{})
	defer close(stopChan)

	worker.Run(5, stopChan)

	worker.Add(name)

	err := assertUntil(20*time.Second, func() error {
		receivedTimes := reconcile.receivedTimes.Load()

		if receivedTimes != wantReceivedTimes {
			return fmt.Errorf("receivedTimes = %v, want = %v", receivedTimes, wantReceivedTimes)
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
