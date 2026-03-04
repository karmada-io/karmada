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

package testing

import "net/http"

// MockOpenSearchTransport is a mock implementation of opensearchtransport.Interface for testing purposes.
type MockOpenSearchTransport struct {
	PerformFunc func(req *http.Request) (*http.Response, error)
}

// Perform executes the mock PerformFunc if it has been defined (is not nil).
// If PerformFunc is undefined (nil), this method will panic, signaling that
// a mock implementation for Perform has not been provided for this test.
func (m *MockOpenSearchTransport) Perform(req *http.Request) (*http.Response, error) {
	if m.PerformFunc != nil {
		return m.PerformFunc(req)
	}
	panic("perform is not implemented")
}
