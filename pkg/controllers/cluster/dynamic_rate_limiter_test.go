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
	"math"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/util/workqueue"

	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
)

// No health-based inputs are needed; health degradation is disabled.

func TestDynamicRateLimiter_When_Scenarios(t *testing.T) {
	tests := []struct {
		name     string
		opts     EvictionQueueOptions
		expected time.Duration
	}{
		{name: "rate 50 => 20ms", opts: EvictionQueueOptions{ResourceEvictionRate: 50}, expected: 20 * time.Millisecond},
		{name: "rate 20 => 50ms", opts: EvictionQueueOptions{ResourceEvictionRate: 20}, expected: 50 * time.Millisecond},
		{name: "rate 30 => 33ms", opts: EvictionQueueOptions{ResourceEvictionRate: 30}, expected: 33 * time.Millisecond},
		{name: "rate 10 => 100ms", opts: EvictionQueueOptions{ResourceEvictionRate: 10}, expected: 100 * time.Millisecond},
		{name: "rate 0 => halt", opts: EvictionQueueOptions{ResourceEvictionRate: 0}, expected: maxEvictionDelay},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewDynamicRateLimiter[any](tt.opts)
			d := limiter.When(struct{}{})
			const epsilon = time.Millisecond
			if diff := d - tt.expected; diff < -epsilon || diff > epsilon {
				t.Fatalf("unexpected duration: got %v, want %v", d, tt.expected)
			}
		})
	}
}

func TestDynamicRateLimiter_getCurrentRate(t *testing.T) {
	tests := []struct {
		name     string
		opts     EvictionQueueOptions
		expected float32
	}{
		{name: "configured 0", opts: EvictionQueueOptions{ResourceEvictionRate: 0}, expected: 0},
		{name: "configured 13.37", opts: EvictionQueueOptions{ResourceEvictionRate: 13.37}, expected: 13.37},
		{name: "configured 50", opts: EvictionQueueOptions{ResourceEvictionRate: 50}, expected: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewDynamicRateLimiter[any](tt.opts)
			rate := limiter.(*DynamicRateLimiter[any]).getCurrentRate()
			if rate != tt.expected {
				t.Errorf("unexpected rate: got %v, want %v", rate, tt.expected)
			}
		})
	}
}

// TestGracefulEvictionRateLimiter_ExponentialBackoff Validation When a task continues to fail,
// The combined rate limiter correctly exhibits exponential avoidance behavior.
func TestGracefulEvictionRateLimiter_ExponentialBackoff(t *testing.T) {
	rateLimiterOpts := ratelimiterflag.Options{}
	const defaultBaseDelay = 5 * time.Millisecond
	const defaultMaxDelay = 1000 * time.Second

	evictionOpts := EvictionQueueOptions{
		ResourceEvictionRate: 50, // 20ms dynamic delay
	}

	t.Logf("Testing with default exponential backoff options: BaseDelay=%v, MaxDelay=%v", defaultBaseDelay, defaultMaxDelay)
	expectedDynamicDelay := time.Second / time.Duration(evictionOpts.ResourceEvictionRate)
	t.Logf("Dynamic limiter is in healthy mode, providing a base delay of: %v (overridden for test)", expectedDynamicDelay)

	limiter := NewGracefulEvictionRateLimiter[any](evictionOpts, rateLimiterOpts)
	queue := workqueue.NewTypedRateLimitingQueueWithConfig[any](limiter, workqueue.TypedRateLimitingQueueConfig[any]{
		Name: "backoff-test-final",
	})

	var (
		mu           sync.Mutex
		attemptTimes []time.Time
	)

	reconcileFunc := util.ReconcileFunc(func(_ util.QueueKey) error {
		mu.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		mu.Unlock()
		return errors.New("always fail to trigger backoff")
	})

	worker := &evictionWorker{
		name:          "backoff-worker",
		keyFunc:       func(obj any) (util.QueueKey, error) { return obj, nil },
		reconcileFunc: reconcileFunc,
		queue:         queue,
	}

	testDuration := 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()
	worker.Run(ctx, 1)
	worker.Add("test-item")
	<-ctx.Done()

	mu.Lock()
	defer mu.Unlock()

	t.Logf("Total attempts in %v: %d", testDuration, len(attemptTimes))
	if len(attemptTimes) < 5 {
		t.Fatalf("Expected at least 5 attempts to observe backoff, but got %d", len(attemptTimes))
	}

	t.Log("--- Analyzing delays between attempts ---")

	for i := 1; i < len(attemptTimes); i++ {
		observedDelay := attemptTimes[i].Sub(attemptTimes[i-1])

		numRequeues := i - 1
		expectedBackoffNs := float64(defaultBaseDelay.Nanoseconds()) * math.Pow(2, float64(numRequeues))
		if expectedBackoffNs > float64(defaultMaxDelay.Nanoseconds()) {
			expectedBackoffNs = float64(defaultMaxDelay.Nanoseconds())
		}
		expectedBackoffDelay := time.Duration(expectedBackoffNs)

		var expectedFinalDelay = max(expectedBackoffDelay, expectedDynamicDelay)

		t.Logf("Attempt %2d: Observed Delay=%-18v | Expected Backoff Delay=%-18v | Effective Expected Delay >= %-18v",
			i+1, observedDelay, expectedBackoffDelay, expectedFinalDelay)

		if observedDelay < expectedFinalDelay*9/10 {
			t.Errorf("Attempt %d: Observed delay %v is significantly less than the effective expected delay %v", i+1, observedDelay, expectedFinalDelay)
		}
	}
}
