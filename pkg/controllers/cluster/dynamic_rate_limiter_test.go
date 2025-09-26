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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	gmtesting "github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager/testing"
)

// fakeGenericLister implements cache.GenericLister for tests.
// It returns pre-configured objects or error in List.
type fakeGenericLister struct {
	objects []runtime.Object
	err     error
}

func (f *fakeGenericLister) List(_ labels.Selector) ([]runtime.Object, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.objects, nil
}

func (f *fakeGenericLister) Get(_ string) (runtime.Object, error)              { return nil, nil }
func (f *fakeGenericLister) ByNamespace(_ string) cache.GenericNamespaceLister { return nil }

func makeCluster(name string, ready bool) *clusterv1alpha1.Cluster {
	condStatus := metav1.ConditionFalse
	if ready {
		condStatus = metav1.ConditionTrue
	}
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{{
				Type:   clusterv1alpha1.ClusterConditionReady,
				Status: condStatus,
			}},
		},
	}
}

func TestDynamicRateLimiter_When_Scenarios(t *testing.T) {
	clusterGVR := clusterv1alpha1.SchemeGroupVersion.WithResource("clusters")

	// helper to compute expected duration from rate
	expectedDur := func(rate float32) time.Duration {
		if rate == 0 {
			return maxEvictionDelay
		}
		return time.Duration(1 / rate * float32(time.Second))
	}

	tests := []struct {
		name     string
		objs     []runtime.Object
		listErr  error
		opts     EvictionQueueOptions
		expected time.Duration
	}{
		{
			name:     "no clusters => use default rate",
			objs:     nil,
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 50, SecondaryResourceEvictionRate: 5, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(50),
		},
		{
			name:     "healthy => default rate",
			objs:     []runtime.Object{makeCluster("c1", true), makeCluster("c2", true)},
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 20, SecondaryResourceEvictionRate: 2, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(20),
		},
		{
			name: "unhealthy large-scale => secondary rate",
			objs: func() []runtime.Object {
				var out []runtime.Object
				for i := 0; i < 6; i++ {
					out = append(out, makeCluster("h"+string(rune('a'+i)), true))
				}
				for i := 0; i < 5; i++ {
					out = append(out, makeCluster("u"+string(rune('a'+i)), false))
				}
				return out
			}(),
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 30, SecondaryResourceEvictionRate: 3, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: expectedDur(3),
		},
		{
			name:     "unhealthy small-scale => halt (max delay)",
			objs:     []runtime.Object{makeCluster("c1", false), makeCluster("c2", true)},
			listErr:  nil,
			opts:     EvictionQueueOptions{ResourceEvictionRate: 10, SecondaryResourceEvictionRate: 1, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: maxEvictionDelay,
		},
		{
			name:     "lister error => halt",
			objs:     nil,
			listErr:  errors.New("boom"),
			opts:     EvictionQueueOptions{ResourceEvictionRate: 10, SecondaryResourceEvictionRate: 1, UnhealthyClusterThreshold: 0.3, LargeClusterNumThreshold: 10},
			expected: maxEvictionDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing scenario: %s", tt.name)
			mgr := gmtesting.NewFakeSingleClusterManager(true, true, func(gvr schema.GroupVersionResource) cache.GenericLister {
				if gvr != clusterGVR {
					return nil
				}
				return &fakeGenericLister{objects: tt.objs, err: tt.listErr}
			})
			limiter := NewDynamicRateLimiter[any](mgr, tt.opts)
			d := limiter.When(struct{}{})
			t.Logf("Got delay: %v, Expected delay: %v", d, tt.expected)
			if d != tt.expected {
				t.Fatalf("unexpected duration: got %v, want %v", d, tt.expected)
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
		ResourceEvictionRate:          50, // 20ms dynamic delay
		SecondaryResourceEvictionRate: 0.1,
		UnhealthyClusterThreshold:     0.55,
		LargeClusterNumThreshold:      10,
	}

	t.Logf("Testing with default exponential backoff options: BaseDelay=%v, MaxDelay=%v", defaultBaseDelay, defaultMaxDelay)
	expectedDynamicDelay := time.Second / time.Duration(evictionOpts.ResourceEvictionRate)
	t.Logf("Dynamic limiter is in healthy mode, providing a base delay of: %v (overridden for test)", expectedDynamicDelay)

	healthyClusters := []runtime.Object{makeCluster("c1", true), makeCluster("c2", true)}
	clusterGVR := clusterv1alpha1.SchemeGroupVersion.WithResource("clusters")
	mgr := gmtesting.NewFakeSingleClusterManager(true, true, func(gvr schema.GroupVersionResource) cache.GenericLister {
		if gvr != clusterGVR {
			return nil
		}
		return &fakeGenericLister{objects: healthyClusters, err: nil}
	})

	limiter := NewGracefulEvictionRateLimiter[any](mgr, evictionOpts, rateLimiterOpts)
	queue := workqueue.NewTypedRateLimitingQueueWithConfig[any](limiter, workqueue.TypedRateLimitingQueueConfig[any]{
		Name: "backoff-test-final",
	})

	var (
		mu           sync.Mutex
		attemptTimes []time.Time
	)

	reconcileFunc := util.ReconcileFunc(func(key util.QueueKey) error {
		mu.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		mu.Unlock()
		return errors.New("always fail to trigger backoff")
	})

	worker := &evictionWorker{
		name:          "backoff-worker",
		keyFunc:       func(obj interface{}) (util.QueueKey, error) { return obj, nil },
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
	var lastObservedDelay time.Duration
	for i := 1; i < len(attemptTimes); i++ {
		observedDelay := attemptTimes[i].Sub(attemptTimes[i-1])

		numRequeues := i - 1
		expectedBackoffNs := float64(defaultBaseDelay.Nanoseconds()) * math.Pow(2, float64(numRequeues))
		if expectedBackoffNs > float64(defaultMaxDelay.Nanoseconds()) {
			expectedBackoffNs = float64(defaultMaxDelay.Nanoseconds())
		}
		expectedBackoffDelay := time.Duration(expectedBackoffNs)

		var expectedFinalDelay time.Duration
		if expectedBackoffDelay > expectedDynamicDelay {
			expectedFinalDelay = expectedBackoffDelay
		} else {
			expectedFinalDelay = expectedDynamicDelay
		}

		t.Logf("Attempt %2d: Observed Delay=%-18v | Expected Backoff Delay=%-18v | Effective Expected Delay >= %-18v",
			i+1, observedDelay, expectedBackoffDelay, expectedFinalDelay)

		if i > 1 {
			// Only check for a strict increase if the backoff delay is larger than the dynamic delay.
			if expectedBackoffDelay > expectedDynamicDelay && observedDelay < lastObservedDelay {
				t.Errorf("Attempt %d: Delay did not increase as expected after backoff became dominant. Previous: %v, Current: %v", i+1, lastObservedDelay, observedDelay)
			}
		}

		if observedDelay < expectedFinalDelay*9/10 {
			t.Errorf("Attempt %d: Observed delay %v is significantly less than the effective expected delay %v", i+1, observedDelay, expectedFinalDelay)
		}
		lastObservedDelay = observedDelay
	}
}