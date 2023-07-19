package restmapper

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	coretesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var fakeResources = []*metav1.APIResourceList{
	{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{{Name: "deployments", Namespaced: true, Kind: "Deployment"}},
	},
	{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}},
	},
	{
		GroupVersion: "v2",
		APIResources: []metav1.APIResource{{Name: "pods", Namespaced: true, Kind: "Pod"}},
	},
	{
		GroupVersion: "extensions/v1beta",
		APIResources: []metav1.APIResource{{Name: "jobs", Namespaced: true, Kind: "Job"}},
	},
}

// getGVRTestCases organizes the test cases for GetGroupVersionResource.
// It can be shared by both benchmark and unit test.
var getGVRTestCases = []struct {
	name        string
	inputGVK    schema.GroupVersionKind
	expectedGVR schema.GroupVersionResource
	expectErr   bool
}{
	{
		name:        "v1,Pod cache miss",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "v2,Pod cache miss",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v2", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v2", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "extensions/v1beta,Job cache miss",
		inputGVK:    schema.GroupVersionKind{Group: "extensions", Version: "v1beta", Kind: "Job"},
		expectedGVR: schema.GroupVersionResource{Group: "extensions", Version: "v1beta", Resource: "jobs"},
		expectErr:   false,
	},
	{
		name:        "v1,Pod cache hit once",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "v1,Pod cache hit twice",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "v2,Pod cache hit once",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v2", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v2", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "v2,Pod cache hit twice",
		inputGVK:    schema.GroupVersionKind{Group: "", Version: "v2", Kind: "Pod"},
		expectedGVR: schema.GroupVersionResource{Group: "", Version: "v2", Resource: "pods"},
		expectErr:   false,
	},
	{
		name:        "extensions/v1beta,Job cache hit once",
		inputGVK:    schema.GroupVersionKind{Group: "extensions", Version: "v1beta", Kind: "Job"},
		expectedGVR: schema.GroupVersionResource{Group: "extensions", Version: "v1beta", Resource: "jobs"},
		expectErr:   false,
	},
	{
		name:        "extensions/v1beta,Job cache hit twice",
		inputGVK:    schema.GroupVersionKind{Group: "extensions", Version: "v1beta", Kind: "Job"},
		expectedGVR: schema.GroupVersionResource{Group: "extensions", Version: "v1beta", Resource: "jobs"},
		expectErr:   false,
	},
	{
		name:      "cache miss and invalidate the cache",
		inputGVK:  schema.GroupVersionKind{Group: "non-existence", Version: "non-existence", Kind: "non-existence"},
		expectErr: true,
	},
}

var discoveryClient = &discoveryfake.FakeDiscovery{Fake: &coretesting.Fake{Resources: fakeResources}}

func BenchmarkGetGroupVersionResource(b *testing.B) {
	var option = apiutil.WithCustomMapper(func() (meta.RESTMapper, error) {
		groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
		if err != nil {
			return nil, err
		}
		return restmapper.NewDiscoveryRESTMapper(groupResources), nil
	})

	mapper, err := apiutil.NewDynamicRESTMapper(&rest.Config{}, option)
	if err != nil {
		b.Error(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range getGVRTestCases {
			_, err := GetGroupVersionResource(mapper, tc.inputGVK)
			if (err != nil && !tc.expectErr) || (err == nil && tc.expectErr) {
				b.Errorf("GetGroupVersionResource For %#v Error: %v, wantErr: %v", tc.inputGVK, err, tc.expectErr)
			}
		}
	}
}
func BenchmarkGetGroupVersionResourceWithoutCache(b *testing.B) {
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		b.Fatalf("Failed to load resources: %v", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range getGVRTestCases {
			_, err := GetGroupVersionResource(mapper, tc.inputGVK)
			if (err != nil && !tc.expectErr) || (err == nil && tc.expectErr) {
				b.Errorf("GetGroupVersionResource For %#v Error: %v, wantErr: %v", tc.inputGVK, err, tc.expectErr)
			}
		}
	}
}

func BenchmarkGetGroupVersionResourceWithCache(b *testing.B) {
	cachedMapper := &cachedRESTMapper{}

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		b.Fatalf("Failed to load resources: %v", err)
	}

	newMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	cachedMapper.restMapper = newMapper
	cachedMapper.discoveryClient = discoveryClient

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range getGVRTestCases {
			_, err := GetGroupVersionResource(cachedMapper, tc.inputGVK)
			if (err != nil && !tc.expectErr) || (err == nil && tc.expectErr) {
				b.Errorf("GetGroupVersionResource For %#v Error: %v, wantErr: %v", tc.inputGVK, err, tc.expectErr)
			}
		}
	}
}

func TestGetGroupVersionResourceWithCache(t *testing.T) {
	cachedMapper := &cachedRESTMapper{}

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("Failed to load resources: %v", err)
	}

	newMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	cachedMapper.restMapper = newMapper
	cachedMapper.discoveryClient = discoveryClient

	for _, tc := range getGVRTestCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetGroupVersionResource(cachedMapper, tc.inputGVK)
			if (err != nil && !tc.expectErr) || (err == nil && tc.expectErr) {
				t.Fatalf("GetGroupVersionResource (%#v) error: %v, wantErr: %v", tc.inputGVK, err, tc.expectErr)
			}

			if got != tc.expectedGVR {
				t.Fatalf("GetGroupVersionResource(%#v) = %#v, want: %#v", tc.inputGVK, got, tc.expectedGVR)
			}
		})
	}
}
