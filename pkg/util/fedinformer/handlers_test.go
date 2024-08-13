/*
Copyright 2024 The Karmada Authors.

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

package fedinformer

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type TestObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec string
}

func (in *TestObject) DeepCopyObject() runtime.Object {
	return &TestObject{
		TypeMeta:   in.TypeMeta,
		ObjectMeta: in.ObjectMeta,
		Spec:       in.Spec,
	}
}

type CustomResourceEventHandler struct {
	handler cache.ResourceEventHandler
}

func (c *CustomResourceEventHandler) OnAdd(obj interface{}, isInInitialList bool) {
	if h, ok := c.handler.(interface{ OnAdd(interface{}, bool) }); ok {
		h.OnAdd(obj, isInInitialList)
	} else {
		c.handler.OnAdd(obj, false)
	}
}

func (c *CustomResourceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	c.handler.OnUpdate(oldObj, newObj)
}

func (c *CustomResourceEventHandler) OnDelete(obj interface{}) {
	c.handler.OnDelete(obj)
}

func TestNewHandlerOnAllEvents(t *testing.T) {
	testCases := []struct {
		name     string
		event    string
		input    interface{}
		expected runtime.Object
	}{
		{
			name:     "Add event",
			event:    "add",
			input:    &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-add"}, Spec: "add"},
			expected: &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-add"}, Spec: "add"},
		},
		{
			name:     "Update event",
			event:    "update",
			input:    &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-update"}, Spec: "update"},
			expected: &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-update"}, Spec: "update"},
		},
		{
			name:     "Delete event",
			event:    "delete",
			input:    &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-delete"}, Spec: "delete"},
			expected: &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-delete"}, Spec: "delete"},
		},
		{
			name:     "Delete event with DeletedFinalStateUnknown",
			event:    "delete",
			input:    cache.DeletedFinalStateUnknown{Obj: &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-delete-unknown"}, Spec: "delete-unknown"}},
			expected: &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj-delete-unknown"}, Spec: "delete-unknown"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var calledWith runtime.Object
			fn := func(obj interface{}) {
				calledWith = obj.(runtime.Object)
			}

			handler := &CustomResourceEventHandler{NewHandlerOnAllEvents(fn)}

			switch tc.event {
			case "add":
				handler.OnAdd(tc.input, false)
			case "update":
				oldObj := &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "old-obj"}, Spec: "old"}
				handler.OnUpdate(oldObj, tc.input)
			case "delete":
				handler.OnDelete(tc.input)
			}

			if !reflect.DeepEqual(calledWith, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, calledWith)
			}
		})
	}
}

func TestNewHandlerOnEvents(t *testing.T) {
	testCases := []struct {
		name  string
		event string
	}{
		{"Add event", "add"},
		{"Update event", "update"},
		{"Delete event", "delete"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var addCalled, updateCalled, deleteCalled bool
			addFunc := func(_ interface{}) { addCalled = true }
			updateFunc := func(_, _ interface{}) { updateCalled = true }
			deleteFunc := func(_ interface{}) { deleteCalled = true }

			handler := &CustomResourceEventHandler{NewHandlerOnEvents(addFunc, updateFunc, deleteFunc)}

			testObj := &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "test-obj"}}

			switch tc.event {
			case "add":
				handler.OnAdd(testObj, false)
				if !addCalled {
					t.Error("AddFunc was not called")
				}
			case "update":
				handler.OnUpdate(testObj, testObj)
				if !updateCalled {
					t.Error("UpdateFunc was not called")
				}
			case "delete":
				handler.OnDelete(testObj)
				if !deleteCalled {
					t.Error("DeleteFunc was not called")
				}
			}
		})
	}
}

func TestNewFilteringHandlerOnAllEvents(t *testing.T) {
	testCases := []struct {
		name           string
		event          string
		input          interface{}
		expectedAdd    bool
		expectedUpdate bool
		expectedDelete bool
	}{
		{
			name:           "Add passing object",
			event:          "add",
			input:          &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "passing-obj"}, Spec: "pass"},
			expectedAdd:    true,
			expectedUpdate: false,
			expectedDelete: false,
		},
		{
			name:           "Add failing object",
			event:          "add",
			input:          &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "failing-obj"}, Spec: "fail"},
			expectedAdd:    false,
			expectedUpdate: false,
			expectedDelete: false,
		},
		{
			name:           "Update to passing object",
			event:          "update",
			input:          &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "passing-obj"}, Spec: "pass"},
			expectedAdd:    false,
			expectedUpdate: true,
			expectedDelete: false,
		},
		{
			name:           "Update to failing object",
			event:          "update",
			input:          &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "failing-obj"}, Spec: "fail"},
			expectedAdd:    false,
			expectedUpdate: false,
			expectedDelete: true,
		},
		{
			name:           "Delete passing object",
			event:          "delete",
			input:          &TestObject{ObjectMeta: metav1.ObjectMeta{Name: "passing-obj"}, Spec: "pass"},
			expectedAdd:    false,
			expectedUpdate: false,
			expectedDelete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var addCalled, updateCalled, deleteCalled bool
			addFunc := func(_ interface{}) { addCalled = true }
			updateFunc := func(_, _ interface{}) { updateCalled = true }
			deleteFunc := func(_ interface{}) { deleteCalled = true }

			filterFunc := func(obj interface{}) bool {
				testObj := obj.(*TestObject)
				return testObj.Spec == "pass"
			}

			handler := NewFilteringHandlerOnAllEvents(filterFunc, addFunc, updateFunc, deleteFunc)

			switch tc.event {
			case "add":
				handler.OnAdd(tc.input, false)
			case "update":
				handler.OnUpdate(&TestObject{Spec: "pass"}, tc.input)
			case "delete":
				handler.OnDelete(tc.input)
			}

			if addCalled != tc.expectedAdd {
				t.Errorf("AddFunc called: %v, expected: %v", addCalled, tc.expectedAdd)
			}
			if updateCalled != tc.expectedUpdate {
				t.Errorf("UpdateFunc called: %v, expected: %v", updateCalled, tc.expectedUpdate)
			}
			if deleteCalled != tc.expectedDelete {
				t.Errorf("DeleteFunc called: %v, expected: %v", deleteCalled, tc.expectedDelete)
			}
		})
	}
}
