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

package binder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestNewAssumeCache(t *testing.T) {
	cache := NewAssumeCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.assumed)
}

func TestAssumeCache_Assume(t *testing.T) {
	cache := NewAssumeCache()
	key := "default/test-binding"
	result := &BindResult{
		Binding: &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-binding",
			},
		},
	}

	cache.Assume(key, result)
	assert.True(t, cache.IsAssumed(key))
}

func TestAssumeCache_Confirm(t *testing.T) {
	cache := NewAssumeCache()
	key := "default/test-binding"
	result := &BindResult{}

	cache.Assume(key, result)
	assert.True(t, cache.IsAssumed(key))

	cache.Confirm(key)
	assert.False(t, cache.IsAssumed(key))
}

func TestAssumeCache_Forget(t *testing.T) {
	cache := NewAssumeCache()
	key := "default/test-binding"
	result := &BindResult{}

	cache.Assume(key, result)
	assert.True(t, cache.IsAssumed(key))

	cache.Forget(key)
	assert.False(t, cache.IsAssumed(key))
}

func TestAssumeCache_IsAssumed(t *testing.T) {
	cache := NewAssumeCache()

	// Not assumed initially
	assert.False(t, cache.IsAssumed("non-existent"))

	// After assuming
	cache.Assume("test-key", &BindResult{})
	assert.True(t, cache.IsAssumed("test-key"))
}

func TestAssumeCache_Get(t *testing.T) {
	cache := NewAssumeCache()
	key := "default/test-binding"
	result := &BindResult{
		Placement: "test-placement",
	}

	// Not found initially
	got, ok := cache.Get(key)
	assert.False(t, ok)
	assert.Nil(t, got)

	// After assuming
	cache.Assume(key, result)
	got, ok = cache.Get(key)
	assert.True(t, ok)
	assert.Equal(t, "test-placement", got.Placement)
}

func TestNewAsyncBinder(t *testing.T) {
	tests := []struct {
		name            string
		workers         int
		expectedWorkers int
	}{
		{
			name:            "default workers when zero",
			workers:         0,
			expectedWorkers: DefaultBindWorkers,
		},
		{
			name:            "default workers when negative",
			workers:         -1,
			expectedWorkers: DefaultBindWorkers,
		},
		{
			name:            "custom workers",
			workers:         16,
			expectedWorkers: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binder := NewAsyncBinder(nil, nil, tt.workers)
			assert.NotNil(t, binder)
			assert.Equal(t, tt.expectedWorkers, binder.workers)
			assert.NotNil(t, binder.bindQueue)
			assert.NotNil(t, binder.assumeCache)
		})
	}
}

func TestAsyncBinder_getBindingKey(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	tests := []struct {
		name     string
		result   *BindResult
		expected string
	}{
		{
			name: "ResourceBinding",
			result: &BindResult{
				Binding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test-rb",
					},
				},
			},
			expected: "default/test-rb",
		},
		{
			name: "ClusterResourceBinding",
			result: &BindResult{
				ClusterBinding: &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-crb",
					},
				},
			},
			expected: "test-crb",
		},
		{
			name:     "empty result",
			result:   &BindResult{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := binder.getBindingKey(tt.result)
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestAsyncBinder_AsyncBind(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	result := &BindResult{
		Binding: &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-binding",
			},
		},
		ScheduleResult: []workv1alpha2.TargetCluster{
			{Name: "member1", Replicas: 1},
		},
	}

	err := binder.AsyncBind(result)
	assert.NoError(t, err)
	assert.True(t, binder.GetAssumeCache().IsAssumed("default/test-binding"))
	assert.Equal(t, 1, binder.QueueLength())
}

func TestAsyncBinder_GetAssumeCache(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)
	cache := binder.GetAssumeCache()
	assert.NotNil(t, cache)
	assert.Equal(t, binder.assumeCache, cache)
}

func TestAsyncBinder_QueueLength(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	assert.Equal(t, 0, binder.QueueLength())

	_ = binder.AsyncBind(&BindResult{
		Binding: &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "rb1"},
		},
	})
	assert.Equal(t, 1, binder.QueueLength())

	_ = binder.AsyncBind(&BindResult{
		Binding: &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "rb2"},
		},
	})
	assert.Equal(t, 2, binder.QueueLength())
}

func TestBindResult_Fields(t *testing.T) {
	condition := metav1.Condition{
		Type:    "Scheduled",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: "Scheduled successfully",
	}

	result := &BindResult{
		Binding: &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-binding",
			},
		},
		ScheduleResult: []workv1alpha2.TargetCluster{
			{Name: "member1", Replicas: 2},
			{Name: "member2", Replicas: 3},
		},
		Placement:    `{"clusterAffinity":{"clusterNames":["member1","member2"]}}`,
		Condition:    condition,
		AffinityName: "test-affinity",
		Timestamp:    time.Now(),
	}

	assert.NotNil(t, result.Binding)
	assert.Nil(t, result.ClusterBinding)
	assert.Len(t, result.ScheduleResult, 2)
	assert.Equal(t, "member1", result.ScheduleResult[0].Name)
	assert.Equal(t, int32(2), result.ScheduleResult[0].Replicas)
	assert.Equal(t, "Scheduled", result.Condition.Type)
	assert.Equal(t, "test-affinity", result.AffinityName)
}

func TestBindResult_ClusterBinding(t *testing.T) {
	result := &BindResult{
		ClusterBinding: &workv1alpha2.ClusterResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-binding",
			},
		},
		ScheduleResult: []workv1alpha2.TargetCluster{
			{Name: "member1"},
		},
	}

	assert.Nil(t, result.Binding)
	assert.NotNil(t, result.ClusterBinding)
	assert.Equal(t, "test-cluster-binding", result.ClusterBinding.Name)
}

func TestMarshalPlacement(t *testing.T) {
	tests := []struct {
		name      string
		placement interface{}
		wantErr   bool
	}{
		{
			name: "simple map",
			placement: map[string]interface{}{
				"clusterAffinity": map[string]interface{}{
					"clusterNames": []string{"member1", "member2"},
				},
			},
			wantErr: false,
		},
		{
			name:      "empty placement",
			placement: map[string]interface{}{},
			wantErr:   false,
		},
		{
			name:      "nil placement",
			placement: nil,
			wantErr:   false,
		},
		{
			name: "complex placement",
			placement: map[string]interface{}{
				"clusterAffinity": map[string]interface{}{
					"clusterNames": []string{"cluster1"},
				},
				"spreadConstraints": []map[string]interface{}{
					{
						"spreadByField": "region",
						"maxGroups":     2,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalPlacement(tt.placement)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestAsyncBinder_AsyncBind_WithClusterBinding(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	result := &BindResult{
		ClusterBinding: &workv1alpha2.ClusterResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-crb",
			},
		},
		ScheduleResult: []workv1alpha2.TargetCluster{
			{Name: "member1"},
		},
	}

	err := binder.AsyncBind(result)
	assert.NoError(t, err)
	assert.True(t, binder.GetAssumeCache().IsAssumed("test-crb"))
}

func TestAsyncBinder_recordScheduleResultEvent_NilEventRecorder(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	// Should not panic with nil event recorder
	rb := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-rb",
		},
	}

	// This should not panic
	assert.NotPanics(t, func() {
		binder.recordScheduleResultEvent(rb, nil, nil, nil)
	})
}

func TestAsyncBinder_recordScheduleResultEvent_SkipOnSuccess(t *testing.T) {
	binder := NewAsyncBinder(nil, nil, 1)

	rb := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-rb",
		},
	}

	// Should not record event on success (err == nil) and should not panic
	assert.NotPanics(t, func() {
		binder.recordScheduleResultEvent(rb, nil, nil, nil)
	})
}
