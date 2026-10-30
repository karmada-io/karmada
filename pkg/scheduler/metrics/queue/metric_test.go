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

package metrics

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var _ MetricRecorder = &fakeBindingsRecorder{}

type fakeBindingsRecorder struct {
	counter int64
}

func (r *fakeBindingsRecorder) Inc() {
	atomic.AddInt64(&r.counter, 1)
}

func (r *fakeBindingsRecorder) Dec() {
	atomic.AddInt64(&r.counter, -1)
}

func (r *fakeBindingsRecorder) Clear() {
	atomic.StoreInt64(&r.counter, 0)
}

func TestInc(t *testing.T) {
	fakeRecorder := fakeBindingsRecorder{}
	var wg sync.WaitGroup
	loops := 100
	wg.Add(loops)
	for range loops {
		go func() {
			fakeRecorder.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	if fakeRecorder.counter != int64(loops) {
		t.Errorf("Expected %v, got %v", loops, fakeRecorder.counter)
	}
}

func TestDec(t *testing.T) {
	fakeRecorder := fakeBindingsRecorder{counter: 100}
	var wg sync.WaitGroup
	loops := 100
	wg.Add(loops)
	for range loops {
		go func() {
			fakeRecorder.Dec()
			wg.Done()
		}()
	}
	wg.Wait()
	if fakeRecorder.counter != int64(0) {
		t.Errorf("Expected %v, got %v", loops, fakeRecorder.counter)
	}
}

func TestClear(t *testing.T) {
	fakeRecorder := fakeBindingsRecorder{}
	var wg sync.WaitGroup
	incLoops, decLoops := 100, 80
	wg.Add(incLoops + decLoops)
	for range incLoops {
		go func() {
			fakeRecorder.Inc()
			wg.Done()
		}()
	}
	for range decLoops {
		go func() {
			fakeRecorder.Dec()
			wg.Done()
		}()
	}
	wg.Wait()
	if fakeRecorder.counter != int64(incLoops-decLoops) {
		t.Errorf("Expected %v, got %v", incLoops-decLoops, fakeRecorder.counter)
	}
	// verify Clear() works
	fakeRecorder.Clear()
	if fakeRecorder.counter != int64(0) {
		t.Errorf("Expected %v, got %v", 0, fakeRecorder.counter)
	}
}

func TestPendingBindingsRecorder(t *testing.T) {
	tests := []struct {
		name     string
		recorder *PendingBindingsRecorder
		gauge    prometheus.Gauge
	}{
		{name: "active", recorder: NewActiveBindingsRecorder(), gauge: ActiveBindings()},
		{name: "backoff", recorder: NewBackoffBindingsRecorder(), gauge: BackoffBindings()},
		{name: "unschedulable", recorder: NewUnschedulableBindingsRecorder(), gauge: UnschedulableBindings()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.recorder.Clear()
			tt.recorder.Inc()
			tt.recorder.Inc()
			tt.recorder.Dec()
			if got := testutil.ToFloat64(tt.gauge); got != 1 {
				t.Errorf("after Inc, Inc, Dec: expected 1, got %v", got)
			}
			tt.recorder.Clear()
			if got := testutil.ToFloat64(tt.gauge); got != 0 {
				t.Errorf("after Clear: expected 0, got %v", got)
			}
		})
	}
}

// TestPendingBindingsServedByControllerRuntimeRegistry guards against registering the
// queue metrics in a registry that the scheduler's /metrics endpoint does not serve.
func TestPendingBindingsServedByControllerRuntimeRegistry(t *testing.T) {
	recorder := NewActiveBindingsRecorder()
	recorder.Clear()
	recorder.Inc()
	defer recorder.Clear()

	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics from controller-runtime registry: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "karmada_scheduler_pending_bindings" {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "queue" && label.GetValue() == "active" {
					if got := metric.GetGauge().GetValue(); got != 1 {
						t.Errorf("expected karmada_scheduler_pending_bindings{queue=\"active\"} to be 1, got %v", got)
					}
					return
				}
			}
		}
	}
	t.Fatalf("karmada_scheduler_pending_bindings{queue=\"active\"} not found in controller-runtime registry")
}
