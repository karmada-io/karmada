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

package queue

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testingclock "k8s.io/utils/clock/testing"

	"github.com/karmada-io/karmada/pkg/scheduler/internal/heap"
	metrics "github.com/karmada-io/karmada/pkg/scheduler/metrics/queue"
)

func TestBackoffQueue_getBackoffTime(t *testing.T) {
	tests := []struct {
		name                   string
		initialBackoffDuration time.Duration
		maxBackoffDuration     time.Duration
		bindingInfo            *QueuedBindingInfo
		want                   time.Time
	}{
		{
			name:                   "no backoff",
			initialBackoffDuration: 1 * time.Second,
			maxBackoffDuration:     32 * time.Second,
			bindingInfo:            &QueuedBindingInfo{Timestamp: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), Attempts: 0},
			want:                   time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:                   "normal backoff within max backoff duration",
			initialBackoffDuration: 1 * time.Second,
			maxBackoffDuration:     32 * time.Second,
			bindingInfo:            &QueuedBindingInfo{Timestamp: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), Attempts: 4},
			want:                   time.Date(2025, 10, 1, 0, 0, 8, 0, time.UTC), // 1s * 2^(4-1) = 8s
		},
		{
			name:                   "zero maxBackoffDuration means no backoff",
			initialBackoffDuration: 0,
			maxBackoffDuration:     0,
			bindingInfo:            &QueuedBindingInfo{Timestamp: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), Attempts: 5},
			want:                   time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bq := &prioritySchedulingQueue{
				bindingInitialBackoffDuration: tt.initialBackoffDuration,
				bindingMaxBackoffDuration:     tt.maxBackoffDuration,
			}
			if got := bq.getBackoffTime(tt.bindingInfo); got != tt.want {
				t.Errorf("backoffQueue.getBackoffTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackoffQueue_calculateBackoffDuration(t *testing.T) {
	tests := []struct {
		name                   string
		initialBackoffDuration time.Duration
		maxBackoffDuration     time.Duration
		attempt                int
		want                   time.Duration
	}{
		{
			name:                   "no backoff",
			initialBackoffDuration: 1 * time.Nanosecond,
			maxBackoffDuration:     32 * time.Nanosecond,
			attempt:                0,
			want:                   0,
		},
		{
			name:                   "normal",
			initialBackoffDuration: 1 * time.Nanosecond,
			maxBackoffDuration:     32 * time.Nanosecond,
			attempt:                3,
			want:                   4 * time.Nanosecond,
		},
		{
			name:                   "hitting max backoff duration",
			initialBackoffDuration: 1 * time.Nanosecond,
			maxBackoffDuration:     32 * time.Nanosecond,
			attempt:                16,
			want:                   32 * time.Nanosecond,
		},
		{
			name:                   "overflow_32bit",
			initialBackoffDuration: 1 * time.Nanosecond,
			maxBackoffDuration:     math.MaxInt32 * time.Nanosecond,
			attempt:                32,
			want:                   math.MaxInt32 * time.Nanosecond,
		},
		{
			name:                   "overflow_64bit",
			initialBackoffDuration: 1 * time.Nanosecond,
			maxBackoffDuration:     math.MaxInt64 * time.Nanosecond,
			attempt:                64,
			want:                   math.MaxInt64 * time.Nanosecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bq := &prioritySchedulingQueue{
				bindingInitialBackoffDuration: tt.initialBackoffDuration,
				bindingMaxBackoffDuration:     tt.maxBackoffDuration,
			}
			bindingInfo := &QueuedBindingInfo{
				Attempts: tt.attempt,
			}
			if got := bq.calculateBackoffDuration(bindingInfo); got != tt.want {
				t.Errorf("backoffQueue.calculateBackoffDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackoffQueueOrdering(t *testing.T) {
	fakeClock := testingclock.NewFakeClock(time.Now())
	bindingInfos := map[string]*QueuedBindingInfo{
		"binding1": {
			NamespacedKey: "binding1",
			Priority:      10,
			Timestamp:     fakeClock.Now().Add(-6 * time.Second),
			Attempts:      4, // 1*2^3 = 8s, backoff not completed
		},
		"binding2": {
			NamespacedKey: "binding2",
			Priority:      1,
			Timestamp:     fakeClock.Now().Add(-2 * time.Second),
			Attempts:      1, // 1s, backoff completed
		},
		"binding3": {
			NamespacedKey: "binding3",
			Priority:      2,
			Timestamp:     fakeClock.Now().Add(-2 * time.Second),
			Attempts:      1, // 1s, backoff completed
		},
		"binding4": {
			NamespacedKey: "binding4",
			Priority:      2,
			Timestamp:     fakeClock.Now().Add(-3 * time.Second),
			Attempts:      2, // 2s, backoff completed
		},
	}

	// popAllBackoffQCompleted is a helper function to pop all bindings from backoffQ which have completed backoff
	popAllBackoffQCompleted := func(bq *prioritySchedulingQueue) []*QueuedBindingInfo {
		var poppedBindingInfos []*QueuedBindingInfo
		if bq == nil {
			return poppedBindingInfos
		}
		bq.lock.Lock()
		defer bq.lock.Unlock()
		for {
			bInfo, ok := bq.backoffQ.Peek()
			if !ok || bInfo == nil {
				break
			}
			if bq.isBindingBackingoff(bInfo) {
				break
			}
			_, err := bq.backoffQ.Pop()
			if err != nil {
				break
			}
			poppedBindingInfos = append(poppedBindingInfos, bInfo)
		}
		return poppedBindingInfos
	}

	tests := []struct {
		name              string
		bindingsInBackoff []string
		wantBindings      []string
	}{
		{
			name:              "backoffQueue is empty, and no binding moved to activeQ",
			bindingsInBackoff: []string{},
			wantBindings:      []string{},
		},
		{
			name:              "high priority binding with long backoff duration should not block lower priority bindings from being moved to activeQ",
			bindingsInBackoff: []string{"binding1", "binding2"},
			wantBindings:      []string{"binding2"},
		},
		{
			name:              "bindings with the same backoff time should be sorted by priority",
			bindingsInBackoff: []string{"binding2", "binding3"},
			wantBindings:      []string{"binding3", "binding2"},
		},
		{
			name:              "bindings with the same backoff time and priority should be sorted by timestamp",
			bindingsInBackoff: []string{"binding3", "binding4"},
			wantBindings:      []string{"binding4", "binding3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bq := &prioritySchedulingQueue{
				clock:                         fakeClock,
				bindingInitialBackoffDuration: DefaultBindingInitialBackoffDuration,
				bindingMaxBackoffDuration:     DefaultBindingMaxBackoffDuration,
			}
			bq.backoffQ = heap.NewWithRecorder(BindingKeyFunc, bq.lessBackoffCompletedWithPriority, metrics.NewBackoffBindingsRecorder())

			for _, binding := range tt.bindingsInBackoff {
				bq.backoffQ.AddOrUpdate(bindingInfos[binding])
			}
			bindingsCompletedBackoff := popAllBackoffQCompleted(bq)
			gotBindings := []string{}
			for _, bInfo := range bindingsCompletedBackoff {
				gotBindings = append(gotBindings, bInfo.NamespacedKey)
			}
			assert.Equalf(t, tt.wantBindings, gotBindings, "expected %v bindings to be completed backoff, got %v", tt.wantBindings, gotBindings)
			bindingsToStayInBackoff := len(tt.bindingsInBackoff) - len(tt.wantBindings)
			assert.Equalf(t, bindingsToStayInBackoff, bq.backoffQ.Len(), "expected %d bindings to stay in backoff queue, got %d", bindingsToStayInBackoff, bq.backoffQ.Len())
		})
	}
}
