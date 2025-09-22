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
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/client-go/util/workqueue"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
)

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

	// Add should ignore nil
	w.Add(nil)
	// Enqueue with valid object
	w.Enqueue("k1")
	// AddAfter should schedule item
	w.AddAfter("k2", 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	w.Run(ctx, 1)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if added.Load() >= 2 { // k1 and k2
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected at least 2 items processed, got %d", added.Load())
}

func TestEvictionWorker_Reconcile_Error_Requeues(t *testing.T) {
	var attempts atomic.Int32
	w := &evictionWorker{
		name:    "err-queue",
		keyFunc: util.KeyFunc(func(obj interface{}) (util.QueueKey, error) { return obj, nil }),
		reconcileFunc: util.ReconcileFunc(func(_ util.QueueKey) error {
			// Fail first time, succeed second time
			if attempts.Add(1) == 1 {
				return errors.New("boom")
			}
			return nil
		}),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig[any](
			ratelimiterflag.DefaultControllerRateLimiter[any](ratelimiterflag.Options{RateLimiterBaseDelay: time.Millisecond, RateLimiterMaxDelay: time.Millisecond, RateLimiterQPS: 1000, RateLimiterBucketSize: 1000}),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "err-queue"},
		),
	}

	w.Add("item")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	w.Run(ctx, 1)

	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if attempts.Load() >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected item to be requeued and processed twice, got %d", attempts.Load())
}

func TestEvictionWorker_Run_Shutdown(t *testing.T) {
	t.Helper()
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

	// Give some time for shutdown without deadlock
	timer := time.NewTimer(200 * time.Millisecond)
	<-timer.C
}
