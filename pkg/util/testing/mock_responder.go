/*
Copyright 2022 The Karmada Authors.

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

package testing

import (
	"encoding/json"
	"net/http/httptest"

	"k8s.io/apimachinery/pkg/runtime"
)

// MockResponder is a mock for `k8s.io/apiserver/pkg/registry/rest/rest.go:292 => Responder interface`
type MockResponder struct {
	resp *httptest.ResponseRecorder
}

// NewResponder creates an instance of MockResponder
func NewResponder(response *httptest.ResponseRecorder) *MockResponder {
	return &MockResponder{
		resp: response,
	}
}

// Object implements Responder interface
func (f *MockResponder) Object(statusCode int, obj runtime.Object) {
	f.resp.Code = statusCode

	if obj != nil {
		err := json.NewEncoder(f.resp).Encode(obj)
		if err != nil {
			f.Error(err)
		}
	}
}

// Error implements Responder interface
func (f *MockResponder) Error(err error) {
	_, _ = f.resp.WriteString(err.Error())
}
