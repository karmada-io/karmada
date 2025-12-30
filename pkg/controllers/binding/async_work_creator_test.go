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

package binding

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewWorkAssumeCache(t *testing.T) {
	cache := NewWorkAssumeCache()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.assumed)
	assert.Equal(t, 0, cache.Size())
}

func TestWorkAssumeCache_Assume(t *testing.T) {
	cache := NewWorkAssumeCache()
	bindingKey := "default/test-binding"

	tasks := []*WorkTask{
		{
			WorkMeta: metav1.ObjectMeta{
				Namespace: "karmada-es-member1",
				Name:      "work-1",
			},
		},
		{
			WorkMeta: metav1.ObjectMeta{
				Namespace: "karmada-es-member2",
				Name:      "work-2",
			},
		},
	}

	cache.Assume(bindingKey, tasks)

	assert.True(t, cache.IsAssumed(bindingKey))
	assert.Equal(t, 2, cache.GetPendingCount(bindingKey))
	assert.Equal(t, 1, cache.Size())
}

func TestWorkAssumeCache_IsAssumed(t *testing.T) {
	cache := NewWorkAssumeCache()

	// Not assumed initially
	assert.False(t, cache.IsAssumed("non-existent"))

	// Assume and check
	cache.Assume("test-key", []*WorkTask{{
		WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work"},
	}})
	assert.True(t, cache.IsAssumed("test-key"))
}

func TestWorkAssumeCache_ConfirmWork(t *testing.T) {
	cache := NewWorkAssumeCache()
	bindingKey := "default/test-binding"

	tasks := []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
	}

	cache.Assume(bindingKey, tasks)
	assert.Equal(t, 2, cache.GetPendingCount(bindingKey))

	// Confirm first work
	cache.ConfirmWork(bindingKey, "ns/work-1")
	assert.Equal(t, 1, cache.GetPendingCount(bindingKey))
	assert.True(t, cache.IsAssumed(bindingKey))

	// Confirm second work - should remove binding entry
	cache.ConfirmWork(bindingKey, "ns/work-2")
	assert.Equal(t, 0, cache.GetPendingCount(bindingKey))
	assert.False(t, cache.IsAssumed(bindingKey))
}

func TestWorkAssumeCache_ForgetWork(t *testing.T) {
	cache := NewWorkAssumeCache()
	bindingKey := "default/test-binding"

	tasks := []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
	}

	cache.Assume(bindingKey, tasks)
	assert.True(t, cache.IsAssumed(bindingKey))

	cache.ForgetWork(bindingKey, "ns/work-1")
	assert.False(t, cache.IsAssumed(bindingKey))
}

func TestWorkAssumeCache_ForgetBinding(t *testing.T) {
	cache := NewWorkAssumeCache()
	bindingKey := "default/test-binding"

	tasks := []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
	}

	cache.Assume(bindingKey, tasks)
	assert.True(t, cache.IsAssumed(bindingKey))

	cache.ForgetBinding(bindingKey)
	assert.False(t, cache.IsAssumed(bindingKey))
	assert.Equal(t, 0, cache.Size())
}

func TestWorkAssumeCache_GetPendingCount(t *testing.T) {
	cache := NewWorkAssumeCache()

	// Non-existent binding
	assert.Equal(t, 0, cache.GetPendingCount("non-existent"))

	// Add tasks
	bindingKey := "test-binding"
	tasks := []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-3"}},
	}
	cache.Assume(bindingKey, tasks)
	assert.Equal(t, 3, cache.GetPendingCount(bindingKey))
}

func TestWorkAssumeCache_CleanupStale(t *testing.T) {
	cache := NewWorkAssumeCache()

	// Add entries with different timestamps
	cache.Assume("binding-1", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
	})

	// Manually set old timestamp
	cache.mu.Lock()
	cache.assumed["binding-1"].timestamp = time.Now().Add(-20 * time.Minute)
	cache.mu.Unlock()

	// Add fresh entry
	cache.Assume("binding-2", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
	})

	assert.Equal(t, 2, cache.Size())

	// Cleanup with 10 minute max age
	cleaned := cache.CleanupStale(10 * time.Minute)
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, 1, cache.Size())
	assert.False(t, cache.IsAssumed("binding-1"))
	assert.True(t, cache.IsAssumed("binding-2"))
}

func TestWorkAssumeCache_Size(t *testing.T) {
	cache := NewWorkAssumeCache()
	assert.Equal(t, 0, cache.Size())

	cache.Assume("binding-1", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
	})
	assert.Equal(t, 1, cache.Size())

	cache.Assume("binding-2", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
	})
	assert.Equal(t, 2, cache.Size())

	cache.ForgetBinding("binding-1")
	assert.Equal(t, 1, cache.Size())
}

func TestNewAsyncWorkCreator(t *testing.T) {
	tests := []struct {
		name            string
		workers         int
		expectedWorkers int
	}{
		{
			name:            "default workers when zero",
			workers:         0,
			expectedWorkers: DefaultAsyncWorkWorkers,
		},
		{
			name:            "default workers when negative",
			workers:         -1,
			expectedWorkers: DefaultAsyncWorkWorkers,
		},
		{
			name:            "custom workers",
			workers:         10,
			expectedWorkers: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := NewAsyncWorkCreator(nil, nil, tt.workers, nil)
			assert.NotNil(t, creator)
			assert.Equal(t, tt.expectedWorkers, creator.workers)
			assert.NotNil(t, creator.workQueue)
			assert.NotNil(t, creator.assumeCache)
		})
	}
}

func TestAsyncWorkCreator_Submit(t *testing.T) {
	requeued := false
	requeueFunc := func(_ string) {
		requeued = true
	}

	creator := NewAsyncWorkCreator(nil, nil, 1, requeueFunc)

	tasks := []*WorkTask{
		{
			WorkMeta: metav1.ObjectMeta{
				Namespace: "karmada-es-member1",
				Name:      "work-1",
			},
			Workload: &unstructured.Unstructured{},
		},
	}

	err := creator.Submit("default/test-binding", tasks)
	assert.NoError(t, err)
	assert.True(t, creator.IsAssumed("default/test-binding"))
	assert.Equal(t, 1, creator.QueueLength())
	assert.False(t, requeued)
}

func TestAsyncWorkCreator_Submit_EmptyTasks(t *testing.T) {
	creator := NewAsyncWorkCreator(nil, nil, 1, nil)

	err := creator.Submit("default/test-binding", []*WorkTask{})
	assert.NoError(t, err)
	assert.False(t, creator.IsAssumed("default/test-binding"))
	assert.Equal(t, 0, creator.QueueLength())
}

func TestAsyncWorkCreator_IsAssumed(t *testing.T) {
	creator := NewAsyncWorkCreator(nil, nil, 1, nil)

	assert.False(t, creator.IsAssumed("non-existent"))

	_ = creator.Submit("test-binding", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work"}},
	})
	assert.True(t, creator.IsAssumed("test-binding"))
}

func TestAsyncWorkCreator_ForgetBinding(t *testing.T) {
	creator := NewAsyncWorkCreator(nil, nil, 1, nil)

	_ = creator.Submit("test-binding", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work"}},
	})
	assert.True(t, creator.IsAssumed("test-binding"))

	creator.ForgetBinding("test-binding")
	assert.False(t, creator.IsAssumed("test-binding"))
}

func TestAsyncWorkCreator_GetStats(t *testing.T) {
	creator := NewAsyncWorkCreator(nil, nil, 1, nil)

	_ = creator.Submit("binding-1", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
	})
	_ = creator.Submit("binding-2", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-3"}},
	})

	queued, created, failed := creator.GetStats()
	assert.Equal(t, int64(3), queued)
	assert.Equal(t, int64(0), created)
	assert.Equal(t, int64(0), failed)
}

func TestAsyncWorkCreator_QueueLength(t *testing.T) {
	creator := NewAsyncWorkCreator(nil, nil, 1, nil)

	assert.Equal(t, 0, creator.QueueLength())

	_ = creator.Submit("binding-1", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-1"}},
	})
	assert.Equal(t, 1, creator.QueueLength())

	_ = creator.Submit("binding-2", []*WorkTask{
		{WorkMeta: metav1.ObjectMeta{Namespace: "ns", Name: "work-2"}},
	})
	assert.Equal(t, 2, creator.QueueLength())
}

func TestWorkTask_Fields(t *testing.T) {
	suspendDispatching := true
	preserveResources := false

	task := &WorkTask{
		BindingKey: "default/my-binding",
		WorkMeta: metav1.ObjectMeta{
			Namespace: "karmada-es-member1",
			Name:      "my-work",
			Labels:    map[string]string{"app": "test"},
		},
		Workload: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
			},
		},
		SuspendDispatching:          &suspendDispatching,
		PreserveResourcesOnDeletion: &preserveResources,
		Timestamp:                   time.Now(),
	}

	assert.Equal(t, "default/my-binding", task.BindingKey)
	assert.Equal(t, "karmada-es-member1", task.WorkMeta.Namespace)
	assert.Equal(t, "my-work", task.WorkMeta.Name)
	assert.NotNil(t, task.Workload)
	assert.True(t, *task.SuspendDispatching)
	assert.False(t, *task.PreserveResourcesOnDeletion)
}
