/*
Copyright 2016 The Kubernetes Authors.
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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/controller/garbagecollector/garbagecollector_test.go#L943-L990
// https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/controller/garbagecollector/garbagecollector_test.go#L707-L797

package lifted

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/controller/garbagecollector/garbagecollector_test.go#L712-L802

// TestGetDeletableResources ensures GetDeletableResources always returns
// something usable regardless of discovery output.
func TestGetDeletableResources(t *testing.T) {
	tests := map[string]struct {
		serverResources    []*metav1.APIResourceList
		err                error
		deletableResources map[schema.GroupVersionResource]struct{}
	}{
		"no error": {
			serverResources: []*metav1.APIResourceList{
				{
					// Valid GroupVersion
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"delete", "list", "watch"}},
						{Name: "services", Namespaced: true, Kind: "Service"},
					},
				},
				{
					// Invalid GroupVersion, should be ignored
					GroupVersion: "foo//whatever",
					APIResources: []metav1.APIResource{
						{Name: "bars", Namespaced: true, Kind: "Bar", Verbs: metav1.Verbs{"delete", "list", "watch"}},
					},
				},
				{
					// Valid GroupVersion, missing required verbs, should be ignored
					GroupVersion: "acme/v1",
					APIResources: []metav1.APIResource{
						{Name: "widgets", Namespaced: true, Kind: "Widget", Verbs: metav1.Verbs{"delete"}},
					},
				},
			},
			err: nil,
			deletableResources: map[schema.GroupVersionResource]struct{}{
				{Group: "apps", Version: "v1", Resource: "pods"}: {},
			},
		},
		"nonspecific failure, includes usable results": {
			serverResources: []*metav1.APIResourceList{
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"delete", "list", "watch"}},
						{Name: "services", Namespaced: true, Kind: "Service"},
					},
				},
			},
			err: fmt.Errorf("internal error"),
			deletableResources: map[schema.GroupVersionResource]struct{}{
				{Group: "apps", Version: "v1", Resource: "pods"}: {},
			},
		},
		"partial discovery failure, includes usable results": {
			serverResources: []*metav1.APIResourceList{
				{
					GroupVersion: "apps/v1",
					APIResources: []metav1.APIResource{
						{Name: "pods", Namespaced: true, Kind: "Pod", Verbs: metav1.Verbs{"delete", "list", "watch"}},
						{Name: "services", Namespaced: true, Kind: "Service"},
					},
				},
			},
			err: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "foo", Version: "v1"}: fmt.Errorf("discovery failure"),
				},
			},
			deletableResources: map[schema.GroupVersionResource]struct{}{
				{Group: "apps", Version: "v1", Resource: "pods"}: {},
			},
		},
		"discovery failure, no results": {
			serverResources:    nil,
			err:                fmt.Errorf("internal error"),
			deletableResources: map[schema.GroupVersionResource]struct{}{},
		},
	}

	for name, test := range tests {
		t.Logf("testing %q", name)
		client := &fakeServerResources{
			PreferredResources: test.serverResources,
			Error:              test.err,
		}
		actual := GetDeletableResources(client)
		if !reflect.DeepEqual(test.deletableResources, actual) {
			t.Errorf("expected resources:\n%v\ngot:\n%v", test.deletableResources, actual)
		}
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.26/pkg/controller/garbagecollector/garbagecollector_test.go#L948-L990

type fakeServerResources struct {
	PreferredResources []*metav1.APIResourceList
	Error              error
	Lock               sync.Mutex
	InterfaceUsedCount int
}

func (*fakeServerResources) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	return nil, nil
}

// Deprecated: use ServerGroupsAndResources instead.
func (*fakeServerResources) ServerResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (*fakeServerResources) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (f *fakeServerResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	f.Lock.Lock()
	defer f.Lock.Unlock()
	f.InterfaceUsedCount++
	return f.PreferredResources, f.Error
}

func (*fakeServerResources) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
