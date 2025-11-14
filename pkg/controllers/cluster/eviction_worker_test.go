/*
Copyright 2025 The Karmada Authors.

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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
)

// mockQueue is a wrapper around a real workqueue that allows us to track calls to the Forget method.
type mockQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	forgetCalls atomic.Int32
}

func (q *mockQueue) Forget(item any) {
	q.forgetCalls.Add(1)
	q.TypedRateLimitingInterface.Forget(item)
}

func newMockQueue(limiter workqueue.TypedRateLimiter[any], name string) *mockQueue {
	return &mockQueue{
		TypedRateLimitingInterface: workqueue.NewTypedRateLimitingQueueWithConfig[any](
			limiter,
			workqueue.TypedRateLimitingQueueConfig[any]{Name: name},
		),
	}
}

// loggingRateLimiter is a wrapper around a real rate limiter that logs the delay assigned to each item.
type loggingRateLimiter[T comparable] struct {
	workqueue.TypedRateLimiter[T]
	t *testing.T
}

func (r *loggingRateLimiter[T]) When(item T) time.Duration {
	delay := r.TypedRateLimiter.When(item)
	r.t.Logf("RateLimiter assigned delay of %v for item: %v", delay, item)
	return delay
}

func newLoggingRateLimiter[T comparable](t *testing.T, inner workqueue.TypedRateLimiter[T]) workqueue.TypedRateLimiter[T] {
	return &loggingRateLimiter[T]{
		t:                t,
		TypedRateLimiter: inner,
	}
}

func TestEvictionWorker_Add_Enqueue_AddAfter(t *testing.T) {
	var added atomic.Int32
	w := &evictionWorker{
		name:             "test-queue",
		keyFunc:          util.KeyFunc(func(obj interface{}) (util.QueueKey, error) { return obj, nil }),
		reconcileFunc:    util.ReconcileFunc(func(_ util.QueueKey) error { added.Add(1); return nil }),
		resourceKindFunc: func(_ interface{}) (string, string) { return "cluster-a", "Pod" },
		queue: workqueue.NewTypedRateLimitingQueueWithConfig[any](
			ratelimiterflag.DefaultControllerRateLimiter[any](ratelimiterflag.Options{RateLimiterBaseDelay: time.Millisecond, RateLimiterMaxDelay: time.Millisecond, RateLimiterQPS: 1000, RateLimiterBucketSize: 1000}),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-queue"},
		),
	}

	w.Add(nil)
	w.Enqueue("k1")
	w.AddAfter("k2", 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	w.Run(ctx, 1)

	err := wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 150*time.Millisecond, true, func(context.Context) (bool, error) {
		return added.Load() >= 2, nil
	})
	if err != nil {
		t.Fatalf("expected at least 2 items processed, got %d", added.Load())
	}
}

func TestEvictionWorker_Reconcile_Scenarios(t *testing.T) {
	fastRateLimiter := ratelimiterflag.DefaultControllerRateLimiter[any](ratelimiterflag.Options{})

	tests := []struct {
		name                  string
		keyFunc               util.KeyFunc
		reconcileFuncFactory  func() (util.ReconcileFunc, *atomic.Int32)
		objectToEnqueue       any
		minExpectedReconciles int32
		expectedForgetCalls   int32
	}{
		{
			name:    "Success on first try",
			keyFunc: func(obj interface{}) (util.QueueKey, error) { return obj, nil },
			reconcileFuncFactory: func() (util.ReconcileFunc, *atomic.Int32) {
				var attempts atomic.Int32
				return func(_ util.QueueKey) error {
					attempts.Add(1)
					return nil
				}, &attempts
			},
			objectToEnqueue:       "item-succeeds",
			minExpectedReconciles: 1,
			expectedForgetCalls:   1,
		},
		{
			name:    "Fail once then succeed",
			keyFunc: func(obj interface{}) (util.QueueKey, error) { return obj, nil },
			reconcileFuncFactory: func() (util.ReconcileFunc, *atomic.Int32) {
				var attempts atomic.Int32
				return func(_ util.QueueKey) error {
					if attempts.Add(1) == 1 {
						return errors.New("boom")
					}
					return nil
				}, &attempts
			},
			objectToEnqueue:       "item-fails-once",
			minExpectedReconciles: 2,
			expectedForgetCalls:   1,
		},
		{
			name:    "Always fail",
			keyFunc: func(obj interface{}) (util.QueueKey, error) { return obj, nil },
			reconcileFuncFactory: func() (util.ReconcileFunc, *atomic.Int32) {
				var attempts atomic.Int32
				return func(_ util.QueueKey) error {
					attempts.Add(1)
					return errors.New("permanent failure")
				}, &attempts
			},
			objectToEnqueue:       "item-always-fails",
			minExpectedReconciles: 3,
			expectedForgetCalls:   0,
		},
		{
			name:    "Key function fails",
			keyFunc: func(_ interface{}) (util.QueueKey, error) { return nil, errors.New("key func error") },
			reconcileFuncFactory: func() (util.ReconcileFunc, *atomic.Int32) {
				var attempts atomic.Int32
				return func(_ util.QueueKey) error {
					attempts.Add(1)
					t.Error("Reconcile should not be called when keyFunc fails")
					return nil
				}, &attempts
			},
			objectToEnqueue:       "item-key-fails",
			minExpectedReconciles: 0,
			expectedForgetCalls:   0,
		},
		{
			name:    "Key function returns nil key",
			keyFunc: func(_ interface{}) (util.QueueKey, error) { return nil, nil },
			reconcileFuncFactory: func() (util.ReconcileFunc, *atomic.Int32) {
				var attempts atomic.Int32
				return func(_ util.QueueKey) error {
					attempts.Add(1)
					t.Error("Reconcile should not be called for nil key")
					return nil
				}, &attempts
			},
			objectToEnqueue:       "item-nil-key",
			minExpectedReconciles: 0,
			expectedForgetCalls:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconcileFunc, attempts := tt.reconcileFuncFactory()
			loggingLimiter := newLoggingRateLimiter(t, fastRateLimiter)
			mockQ := newMockQueue(loggingLimiter, "mock-queue")

			w := &evictionWorker{
				name:          "test-worker",
				keyFunc:       tt.keyFunc,
				reconcileFunc: reconcileFunc,
				queue:         mockQ,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			w.Run(ctx, 1)

			w.Enqueue(tt.objectToEnqueue)

			err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 450*time.Millisecond, true, func(context.Context) (bool, error) {
				return attempts.Load() >= tt.minExpectedReconciles, nil
			})
			if err != nil && tt.minExpectedReconciles > 0 {
				t.Fatalf("Timed out waiting for reconcile calls. Got %d, expected at least %d.", attempts.Load(), tt.minExpectedReconciles)
			}

			if tt.expectedForgetCalls > 0 {
				err = wait.PollUntilContextTimeout(ctx, 5*time.Millisecond, 100*time.Millisecond, true, func(context.Context) (bool, error) {
					return mockQ.forgetCalls.Load() >= tt.expectedForgetCalls, nil
				})
				if err != nil {
					t.Fatalf("Timed out waiting for Forget() to be called. Got %d calls, expected %d.", mockQ.forgetCalls.Load(), tt.expectedForgetCalls)
				}
			} else {
				time.Sleep(50 * time.Millisecond)
			}

			finalAttempts := attempts.Load()
			finalForgets := mockQ.forgetCalls.Load()

			if tt.minExpectedReconciles > 0 && finalAttempts < tt.minExpectedReconciles {
				t.Errorf("Expected at least %d reconcile attempts, got %d", tt.minExpectedReconciles, finalAttempts)
			}
			if tt.minExpectedReconciles == 0 && finalAttempts > 0 {
				t.Errorf("Expected 0 reconcile attempts, got %d", finalAttempts)
			}
			if finalForgets != tt.expectedForgetCalls {
				t.Errorf("Expected %d calls to Forget(), got %d", tt.expectedForgetCalls, finalForgets)
			}
			t.Logf("Test case finished. Final reconcile attempts: %d, Final forget calls: %d", finalAttempts, finalForgets)
		})
	}
}

func TestEvictionWorker_Run_Shutdown(_ *testing.T) {
	w := &evictionWorker{
		name:          "shutdown-queue",
		keyFunc:       util.KeyFunc(func(obj interface{}) (util.QueueKey, error) { return obj, nil }),
		reconcileFunc: util.ReconcileFunc(func(_ util.QueueKey) error { return nil }),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig[any](
			ratelimiterflag.DefaultControllerRateLimiter[any](ratelimiterflag.Options{RateLimiterBaseDelay: time.Millisecond, RateLimiterMaxDelay: time.Millisecond, RateLimiterQPS: 1000, RateLimiterBucketSize: 1000}),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "shutdown-queue"},
		),
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Run(ctx, 1)
	w.Add("x")
	cancel()

	<-time.After(200 * time.Millisecond)
}

// TestEvictionWorker_ThroughputWithDynamicRateLimiter is a table-driven integration test.
// It verifies that the evictionWorker's actual processing throughput correctly reflects
// the rate calculated by the DynamicRateLimiter under various cluster health scenarios.
func TestEvictionWorker_ThroughputWithDynamicRateLimiter(t *testing.T) {
	testDuration := 200 * time.Millisecond

	tests := []struct {
		name                 string
		evictionOpts         EvictionQueueOptions
		expectedMinProcessed int32
		expectedMaxProcessed int32
	}{
		{
			name: "rate=50 per second",
			evictionOpts: EvictionQueueOptions{
				ResourceEvictionRate: 50,
			},
			expectedMinProcessed: 6,
			expectedMaxProcessed: 12,
		},
		{
			name: "rate=0 should nearly halt",
			evictionOpts: EvictionQueueOptions{
				ResourceEvictionRate: 0,
			},
			expectedMinProcessed: 0,
			expectedMaxProcessed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var processedCount atomic.Int32
			opts := EvictionWorkerOptions{
				Name:                 fmt.Sprintf("throughput-test-%s", tt.name),
				KeyFunc:              func(obj interface{}) (util.QueueKey, error) { return obj, nil },
				ReconcileFunc:        func(_ util.QueueKey) error { processedCount.Add(1); return nil },
				EvictionQueueOptions: tt.evictionOpts,
				RateLimiterOptions:   ratelimiterflag.Options{},
			}
			worker := NewEvictionWorker(opts)

			ctx, cancel := context.WithTimeout(context.Background(), testDuration+50*time.Millisecond)
			defer cancel()
			worker.Run(ctx, 1)

			go func() {
				for i := 0; ; i++ {
					select {
					case <-ctx.Done():
						return
					default:
						worker.Enqueue(i)
						time.Sleep(1 * time.Millisecond)
					}
				}
			}()

			time.Sleep(testDuration)

			finalProcessedCount := processedCount.Load()
			t.Logf("In %v, worker processed %d items.", testDuration, finalProcessedCount)

			if finalProcessedCount < tt.expectedMinProcessed || finalProcessedCount > tt.expectedMaxProcessed {
				t.Errorf("Throughput out of range. Got %d, expected between %d and %d.",
					finalProcessedCount, tt.expectedMinProcessed, tt.expectedMaxProcessed)
			}
		})
	}
}
