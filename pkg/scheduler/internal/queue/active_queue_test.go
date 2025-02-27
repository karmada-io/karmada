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
	"testing"
)

func TestPriorityQueue(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func() bool
	}{
		{
			name: "Add b1, b2 and Get the binding with highest priority",
			testFunc: func() bool {
				b1 := &QueuedBindingInfo{NamespacedKey: "foo1", Priority: 2}
				b2 := &QueuedBindingInfo{NamespacedKey: "foo2", Priority: 3}
				queue := NewActiveQueue(nil)
				queue.Push(b1)
				queue.Push(b2)
				item, shutdown := queue.Pop()
				if shutdown {
					return false
				}
				return item.NamespacedKey == b2.NamespacedKey
			},
		},
		{
			name: "Add b1, b2 and Get the binding which was enqueued first",
			testFunc: func() bool {
				b1 := &QueuedBindingInfo{NamespacedKey: "foo1"}
				b2 := &QueuedBindingInfo{NamespacedKey: "foo2"}
				queue := NewActiveQueue(nil)
				queue.Push(b1)
				queue.Push(b2)
				item, shutdown := queue.Pop()
				if shutdown {
					return false
				}
				return item.NamespacedKey == b1.NamespacedKey
			},
		},
	}
	for _, tt := range tests {
		success := tt.testFunc()
		if !success {
			t.Fatalf(`%q should be success, but get failed`, tt.name)
		}
	}
}
