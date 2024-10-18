/*
Copyright 2023 The Karmada Authors.

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

package detector

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func BenchmarkEventFilterNoSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = nil
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoWildcards(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterPrefixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}
func BenchmarkEventFilterSuffixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "example-benchmark",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterMultiSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"), regexp.MustCompile("^benchmark-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]interface{}{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterExtensionApiserverAuthentication(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^kube-.*$"))
	for i := 0; i < b.N; i++ {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "extension-apiserver-authentication",
					"namespace": "kube-system",
				},
			},
		})
	}
}

func TestGVRDisabled(t *testing.T) {
	tests := []struct {
		name     string
		gvr      schema.GroupVersionResource
		config   *util.SkippedResourceConfig
		expected bool
	}{
		{
			name:     "GVR not disabled",
			gvr:      schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			config:   &util.SkippedResourceConfig{},
			expected: false,
		},
		{
			name:     "GVR disabled by group",
			gvr:      schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			config:   &util.SkippedResourceConfig{Groups: map[string]struct{}{"apps": {}}},
			expected: true,
		},
		{
			name:     "GVR disabled by group version",
			gvr:      schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			config:   &util.SkippedResourceConfig{GroupVersions: map[schema.GroupVersion]struct{}{{Group: "apps", Version: "v1"}: {}}},
			expected: true,
		},
		{
			name:     "SkippedResourceConfig is nil",
			gvr:      schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			config:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				SkippedResourceConfig: tt.config,
				RESTMapper:            &mockRESTMapper{},
			}
			result := d.gvrDisabled(tt.gvr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNeedLeaderElection(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{
			name: "NeedLeaderElection always returns true",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{}
			got := d.NeedLeaderElection()
			assert.Equal(t, tt.want, got, "NeedLeaderElection() = %v, want %v", got, tt.want)
		})
	}
}

func TestEventFilter(t *testing.T) {
	tests := []struct {
		name                         string
		obj                          *unstructured.Unstructured
		skippedPropagatingNamespaces []*regexp.Regexp
		expected                     bool
	}{
		{
			name: "object in karmada-system namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": "karmada-system",
						"name":      "test-obj",
					},
				},
			},
			expected: false,
		},
		{
			name: "object in karmada-cluster namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": "karmada-cluster",
						"name":      "test-obj",
					},
				},
			},
			expected: false,
		},
		{
			name: "object in karmada-es-* namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": "karmada-es-test",
						"name":      "test-obj",
					},
				},
			},
			expected: false,
		},
		{
			name: "object in skipped namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": "kube-system",
						"name":      "test-obj",
					},
				},
			},
			skippedPropagatingNamespaces: []*regexp.Regexp{regexp.MustCompile("kube-.*")},
			expected:                     false,
		},
		{
			name: "object in non-skipped namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "test-obj",
					},
				},
			},
			expected: true,
		},
		{
			name: "extension-apiserver-authentication configmap in kube-system",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"namespace": "kube-system",
						"name":      "extension-apiserver-authentication",
					},
				},
			},
			expected: false,
		},
		{
			name: "cluster-scoped resource",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Node",
					"metadata": map[string]interface{}{
						"name": "test-node",
					},
				},
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				SkippedPropagatingNamespaces: tt.skippedPropagatingNamespaces,
			}
			result := d.EventFilter(tt.obj)
			assert.Equal(t, tt.expected, result, "For test case: %s", tt.name)
		})
	}
}

func TestOnAdd(t *testing.T) {
	tests := []struct {
		name            string
		obj             interface{}
		expectedEnqueue bool
	}{
		{
			name: "valid unstructured object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectedEnqueue: true,
		},
		{
			name: "invalid unstructured object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expectedEnqueue: true, // The function doesn't check for validity, so it will still enqueue
		},
		{
			name:            "non-runtime object",
			obj:             "not a runtime.Object",
			expectedEnqueue: false,
		},
		{
			name: "core v1 object",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectedEnqueue: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockAsyncWorker{}
			d := &ResourceDetector{
				Processor: mockProcessor,
			}
			d.OnAdd(tt.obj)
			if tt.expectedEnqueue {
				assert.Equal(t, 1, mockProcessor.enqueueCount, "Object should be enqueued")
				assert.IsType(t, ResourceItem{}, mockProcessor.lastEnqueued, "Enqueued item should be of type ResourceItem")
				enqueued := mockProcessor.lastEnqueued.(ResourceItem)
				assert.Equal(t, tt.obj, enqueued.Obj, "Enqueued object should match the input object")
			} else {
				assert.Equal(t, 0, mockProcessor.enqueueCount, "Object should not be enqueued")
			}
		})
	}
}

func TestOnUpdate(t *testing.T) {
	tests := []struct {
		name                      string
		oldObj                    interface{}
		newObj                    interface{}
		expectedEnqueue           bool
		expectedChangeByKarmada   bool
		expectToUnstructuredError bool
	}{
		{
			name: "valid update with changes",
			oldObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectedEnqueue:         true,
			expectedChangeByKarmada: false,
		},
		{
			name: "update without changes",
			oldObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			expectedEnqueue: false,
		},
		{
			name:            "invalid object",
			oldObj:          "not a runtime.Object",
			newObj:          "not a runtime.Object",
			expectedEnqueue: false,
		},
		{
			name: "change by Karmada",
			oldObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"annotations": map[string]interface{}{
							util.PolicyPlacementAnnotation: "test",
						},
					},
				},
			},
			expectedEnqueue:         true,
			expectedChangeByKarmada: true,
		},
		{
			name: "core v1 object",
			oldObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			newObj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}, {Name: "container2"}},
				},
			},
			expectedEnqueue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockAsyncWorker{}
			d := &ResourceDetector{
				Processor: mockProcessor,
			}

			d.OnUpdate(tt.oldObj, tt.newObj)

			if tt.expectedEnqueue {
				assert.Equal(t, 1, mockProcessor.enqueueCount, "Object should be enqueued")
				assert.IsType(t, ResourceItem{}, mockProcessor.lastEnqueued, "Enqueued item should be of type ResourceItem")
				enqueued := mockProcessor.lastEnqueued.(ResourceItem)
				assert.Equal(t, tt.newObj, enqueued.Obj, "Enqueued object should match the new object")
				assert.Equal(t, tt.expectedChangeByKarmada, enqueued.ResourceChangeByKarmada, "ResourceChangeByKarmada flag should match expected value")
			} else {
				assert.Equal(t, 0, mockProcessor.enqueueCount, "Object should not be enqueued")
			}
		})
	}
}

func TestOnDelete(t *testing.T) {
	tests := []struct {
		name            string
		obj             runtime.Object
		expectedEnqueue bool
	}{
		{
			name: "valid object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectedEnqueue: true,
		},
		{
			name: "invalid object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expectedEnqueue: true, // The function doesn't check for validity, so it will still enqueue
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockAsyncWorker{}
			d := &ResourceDetector{
				Processor: mockProcessor,
			}

			d.OnDelete(tt.obj)

			if tt.expectedEnqueue {
				assert.Equal(t, 1, mockProcessor.enqueueCount, "Object should be enqueued")
			} else {
				assert.Equal(t, 0, mockProcessor.enqueueCount, "Object should not be enqueued")
			}
		})
	}
}

func TestLookForMatchedPolicy(t *testing.T) {
	tests := []struct {
		name           string
		object         *unstructured.Unstructured
		policies       []*policyv1alpha1.PropagationPolicy
		expectedPolicy *policyv1alpha1.PropagationPolicy
	}{
		{
			name: "matching policy found",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			policies: []*policyv1alpha1.PropagationPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "policy-1",
						Namespace: "default",
					},
					Spec: policyv1alpha1.PropagationSpec{
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
							},
						},
					},
				},
			},
			expectedPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "policy-1",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme)

			d := &ResourceDetector{
				DynamicClient: fakeClient,
				propagationPolicyLister: &mockPropagationPolicyLister{
					policies: tt.policies,
				},
			}

			objectKey := keys.ClusterWideKey{
				Name:      tt.object.GetName(),
				Namespace: tt.object.GetNamespace(),
				Kind:      tt.object.GetKind(),
			}

			policy, err := d.LookForMatchedPolicy(tt.object, objectKey)

			if err != nil {
				t.Errorf("LookForMatchedPolicy returned an error: %v", err)
			}

			fmt.Printf("Returned policy: %+v\n", policy)

			if tt.expectedPolicy == nil {
				assert.Nil(t, policy)
			} else {
				assert.NotNil(t, policy)
				if policy != nil {
					assert.Equal(t, tt.expectedPolicy.Name, policy.Name)
					assert.Equal(t, tt.expectedPolicy.Namespace, policy.Namespace)
				}
			}
		})
	}
}

func TestLookForMatchedClusterPolicy(t *testing.T) {
	tests := []struct {
		name           string
		object         *unstructured.Unstructured
		policies       []*policyv1alpha1.ClusterPropagationPolicy
		expectedPolicy *policyv1alpha1.ClusterPropagationPolicy
	}{
		{
			name: "matching cluster policy found",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			policies: []*policyv1alpha1.ClusterPropagationPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-policy-1",
					},
					Spec: policyv1alpha1.PropagationSpec{
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
							},
						},
					},
				},
			},
			expectedPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-policy-1",
				},
				Spec: policyv1alpha1.PropagationSpec{
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme)

			d := &ResourceDetector{
				DynamicClient: fakeClient,
				clusterPropagationPolicyLister: &mockClusterPropagationPolicyLister{
					policies: tt.policies,
				},
			}

			objectKey := keys.ClusterWideKey{
				Name:      tt.object.GetName(),
				Namespace: tt.object.GetNamespace(),
				Kind:      tt.object.GetKind(),
			}

			policy, err := d.LookForMatchedClusterPolicy(tt.object, objectKey)

			if err != nil {
				t.Errorf("LookForMatchedClusterPolicy returned an error: %v", err)
			}

			fmt.Printf("Returned cluster policy: %+v\n", policy)

			if tt.expectedPolicy == nil {
				assert.Nil(t, policy)
			} else {
				assert.NotNil(t, policy)
				if policy != nil {
					assert.Equal(t, tt.expectedPolicy.Name, policy.Name)
				}
			}
		})
	}
}

func TestApplyPolicy(t *testing.T) {
	tests := []struct {
		name                    string
		object                  *unstructured.Unstructured
		policy                  *policyv1alpha1.PropagationPolicy
		resourceChangeByKarmada bool
		expectError             bool
	}{
		{
			name: "basic apply policy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
					},
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.object).Build()
			fakeRecorder := record.NewFakeRecorder(10)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			mockDetector := &mockResourceDetector{
				ResourceDetector: ResourceDetector{
					Client:              fakeClient,
					DynamicClient:       fakeDynamicClient,
					EventRecorder:       fakeRecorder,
					ResourceInterpreter: &mockResourceInterpreter{},
					RESTMapper:          &mockRESTMapper{},
				},
				mockClaimPolicyForObject: func(_ *unstructured.Unstructured, _ *policyv1alpha1.PropagationPolicy) (string, error) {
					return "mocked-policy-id", nil
				},
				mockBuildResourceBinding: func(object *unstructured.Unstructured, _, _ map[string]string, _ *policyv1alpha1.PropagationSpec) (*workv1alpha2.ResourceBinding, error) {
					return &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      object.GetName() + "-" + strings.ToLower(object.GetKind()),
							Namespace: object.GetNamespace(),
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion: object.GetAPIVersion(),
								Kind:       object.GetKind(),
								Name:       object.GetName(),
								Namespace:  object.GetNamespace(),
							},
						},
					}, nil
				},
			}

			err := mockDetector.ApplyPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify that the ResourceBinding was created
				binding := &workv1alpha2.ResourceBinding{}
				err = fakeClient.Get(context.TODO(), client.ObjectKey{
					Namespace: tt.object.GetNamespace(),
					Name:      tt.object.GetName() + "-" + strings.ToLower(tt.object.GetKind()),
				}, binding)
				assert.NoError(t, err)
				assert.Equal(t, tt.object.GetName(), binding.Spec.Resource.Name)
			}
		})
	}
}
func TestApplyClusterPolicy(t *testing.T) {
	tests := []struct {
		name                    string
		object                  *unstructured.Unstructured
		policy                  *policyv1alpha1.ClusterPropagationPolicy
		resourceChangeByKarmada bool
		expectError             bool
	}{
		{
			name: "apply cluster policy for namespaced resource",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
					},
				},
			},
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-policy",
				},
				Spec: policyv1alpha1.PropagationSpec{},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
		},
		{
			name: "apply cluster policy for cluster-scoped resource",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"kind":       "ClusterRole",
					"metadata": map[string]interface{}{
						"name": "test-cluster-role",
						"uid":  "test-uid",
					},
				},
			},
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-policy",
				},
				Spec: policyv1alpha1.PropagationSpec{},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			fakeRecorder := record.NewFakeRecorder(10)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			d := &mockResourceDetector{
				ResourceDetector: ResourceDetector{
					Client:              fakeClient,
					DynamicClient:       fakeDynamicClient,
					EventRecorder:       fakeRecorder,
					ResourceInterpreter: &mockResourceInterpreter{},
					RESTMapper:          &mockRESTMapper{},
				},
				mockClaimClusterPolicyForObject: func(_ *unstructured.Unstructured, _ *policyv1alpha1.ClusterPropagationPolicy) (string, error) {
					return "mocked-cluster-policy-id", nil
				},
				mockBuildResourceBinding: func(object *unstructured.Unstructured, _, _ map[string]string, _ *policyv1alpha1.PropagationSpec) (*workv1alpha2.ResourceBinding, error) {
					binding := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      object.GetName() + "-" + strings.ToLower(object.GetKind()),
							Namespace: object.GetNamespace(),
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion: object.GetAPIVersion(),
								Kind:       object.GetKind(),
								Name:       object.GetName(),
								Namespace:  object.GetNamespace(),
							},
						},
					}
					return binding, nil
				},
				mockBuildClusterResourceBinding: func(object *unstructured.Unstructured, _, _ map[string]string, _ *policyv1alpha1.PropagationSpec) (*workv1alpha2.ClusterResourceBinding, error) {
					binding := &workv1alpha2.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: object.GetName() + "-" + strings.ToLower(object.GetKind()),
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion: object.GetAPIVersion(),
								Kind:       object.GetKind(),
								Name:       object.GetName(),
							},
						},
					}
					return binding, nil
				},
			}

			err := d.ApplyClusterPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check if ResourceBinding or ClusterResourceBinding was created
			if tt.object.GetNamespace() != "" {
				binding := &workv1alpha2.ResourceBinding{}
				err = fakeClient.Get(context.TODO(), client.ObjectKey{
					Namespace: tt.object.GetNamespace(),
					Name:      tt.object.GetName() + "-" + strings.ToLower(tt.object.GetKind()),
				}, binding)
				assert.NoError(t, err)
				assert.Equal(t, tt.object.GetName(), binding.Spec.Resource.Name)
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{}
				err = fakeClient.Get(context.TODO(), client.ObjectKey{
					Name: tt.object.GetName() + "-" + strings.ToLower(tt.object.GetKind()),
				}, binding)
				assert.NoError(t, err)
				assert.Equal(t, tt.object.GetName(), binding.Spec.Resource.Name)
			}
		})
	}
}

//Helper Functions

// setupTestScheme creates a runtime scheme with necessary types for testing
func setupTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha2.Install(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}

// Mock implementations

// mockAsyncWorker is a mock implementation of util.AsyncWorker
type mockAsyncWorker struct {
	enqueueCount int
	lastEnqueued interface{}
}

func (m *mockAsyncWorker) Enqueue(item interface{}) {
	m.enqueueCount++
	m.lastEnqueued = item
}

func (m *mockAsyncWorker) Add(_ interface{}) {
	m.enqueueCount++
}
func (m *mockAsyncWorker) AddAfter(_ interface{}, _ time.Duration) {}

func (m *mockAsyncWorker) Run(_ int, _ <-chan struct{}) {}

// mockRESTMapper is a simple mock that satisfies the meta.RESTMapper interface
type mockRESTMapper struct{}

func (m *mockRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: resource.Resource}, nil
}

func (m *mockRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	gvk, err := m.KindFor(resource)
	if err != nil {
		return nil, err
	}
	return []schema.GroupVersionKind{gvk}, nil
}

func (m *mockRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return input, nil
}

func (m *mockRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return []schema.GroupVersionResource{input}, nil
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return &meta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: gk.Group, Version: versions[0], Resource: gk.Kind},
		GroupVersionKind: schema.GroupVersionKind{Group: gk.Group, Version: versions[0], Kind: gk.Kind},
		Scope:            meta.RESTScopeNamespace,
	}, nil
}

func (m *mockRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	mapping, err := m.RESTMapping(gk, versions...)
	if err != nil {
		return nil, err
	}
	return []*meta.RESTMapping{mapping}, nil
}

func (m *mockRESTMapper) ResourceSingularizer(resource string) (string, error) {
	return resource, nil
}

// mockResourceDetector is a mock implementation of ResourceDetector
type mockResourceDetector struct {
	ResourceDetector
	mockClaimPolicyForObject        func(object *unstructured.Unstructured, policy *policyv1alpha1.PropagationPolicy) (string, error)
	mockClaimClusterPolicyForObject func(object *unstructured.Unstructured, policy *policyv1alpha1.ClusterPropagationPolicy) (string, error)
	mockBuildResourceBinding        func(object *unstructured.Unstructured, labels, annotations map[string]string, spec *policyv1alpha1.PropagationSpec) (*workv1alpha2.ResourceBinding, error)
	mockBuildClusterResourceBinding func(object *unstructured.Unstructured, labels, annotations map[string]string, spec *policyv1alpha1.PropagationSpec) (*workv1alpha2.ClusterResourceBinding, error)
}

func (m *mockResourceDetector) ClaimPolicyForObject(object *unstructured.Unstructured, policy *policyv1alpha1.PropagationPolicy) (string, error) {
	if m.mockClaimPolicyForObject != nil {
		return m.mockClaimPolicyForObject(object, policy)
	}
	return "", nil
}

func (m *mockResourceDetector) ClaimClusterPolicyForObject(object *unstructured.Unstructured, policy *policyv1alpha1.ClusterPropagationPolicy) (string, error) {
	if m.mockClaimClusterPolicyForObject != nil {
		return m.mockClaimClusterPolicyForObject(object, policy)
	}
	return "", nil
}

func (m *mockResourceDetector) BuildResourceBinding(object *unstructured.Unstructured, labels, annotations map[string]string, spec *policyv1alpha1.PropagationSpec) (*workv1alpha2.ResourceBinding, error) {
	if m.mockBuildResourceBinding != nil {
		return m.mockBuildResourceBinding(object, labels, annotations, spec)
	}
	return &workv1alpha2.ResourceBinding{}, nil
}

func (m *mockResourceDetector) BuildClusterResourceBinding(object *unstructured.Unstructured, labels, annotations map[string]string, spec *policyv1alpha1.PropagationSpec) (*workv1alpha2.ClusterResourceBinding, error) {
	if m.mockBuildClusterResourceBinding != nil {
		return m.mockBuildClusterResourceBinding(object, labels, annotations, spec)
	}
	return &workv1alpha2.ClusterResourceBinding{}, nil
}

// mockPropagationPolicyLister is a mock implementation of the PropagationPolicyLister
type mockPropagationPolicyLister struct {
	policies []*policyv1alpha1.PropagationPolicy
}

func (m *mockPropagationPolicyLister) List(_ labels.Selector) (ret []runtime.Object, err error) {
	var result []runtime.Object
	for _, p := range m.policies {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
		if err != nil {
			return nil, err
		}
		result = append(result, &unstructured.Unstructured{Object: u})
	}
	return result, nil
}

func (m *mockPropagationPolicyLister) Get(name string) (runtime.Object, error) {
	for _, p := range m.policies {
		if p.Name == name {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
			if err != nil {
				return nil, err
			}
			return &unstructured.Unstructured{Object: u}, nil
		}
	}
	return nil, nil
}

func (m *mockPropagationPolicyLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &mockGenericNamespaceLister{
		policies:  m.policies,
		namespace: namespace,
	}
}

// mockGenericNamespaceLister is a mock implementation of cache.GenericNamespaceLister
type mockGenericNamespaceLister struct {
	policies  []*policyv1alpha1.PropagationPolicy
	namespace string
}

func (m *mockGenericNamespaceLister) List(_ labels.Selector) (ret []runtime.Object, err error) {
	var result []runtime.Object
	for _, p := range m.policies {
		if p.Namespace == m.namespace {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
			if err != nil {
				return nil, err
			}
			result = append(result, &unstructured.Unstructured{Object: u})
		}
	}
	return result, nil
}

func (m *mockGenericNamespaceLister) Get(name string) (runtime.Object, error) {
	for _, p := range m.policies {
		if p.Name == name && p.Namespace == m.namespace {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
			if err != nil {
				return nil, err
			}
			return &unstructured.Unstructured{Object: u}, nil
		}
	}
	return nil, nil
}

// mockClusterPropagationPolicyLister is a mock implementation of the ClusterPropagationPolicyLister
type mockClusterPropagationPolicyLister struct {
	policies []*policyv1alpha1.ClusterPropagationPolicy
}

func (m *mockClusterPropagationPolicyLister) List(_ labels.Selector) (ret []runtime.Object, err error) {
	var result []runtime.Object
	for _, p := range m.policies {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
		if err != nil {
			return nil, err
		}
		result = append(result, &unstructured.Unstructured{Object: u})
	}
	return result, nil
}

func (m *mockClusterPropagationPolicyLister) Get(name string) (runtime.Object, error) {
	for _, p := range m.policies {
		if p.Name == name {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
			if err != nil {
				return nil, err
			}
			return &unstructured.Unstructured{Object: u}, nil
		}
	}
	return nil, nil
}

func (m *mockClusterPropagationPolicyLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return nil // ClusterPropagationPolicies are not namespaced
}

// mockResourceInterpreter is a mock implementation of the ResourceInterpreter interface
type mockResourceInterpreter struct{}

func (m *mockResourceInterpreter) Start(_ context.Context) error {
	return nil
}

func (m *mockResourceInterpreter) HookEnabled(_ schema.GroupVersionKind, _ configv1alpha1.InterpreterOperation) bool {
	return false
}

func (m *mockResourceInterpreter) GetReplicas(_ *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	return 0, nil, nil
}

func (m *mockResourceInterpreter) ReviseReplica(object *unstructured.Unstructured, _ int64) (*unstructured.Unstructured, error) {
	return object, nil
}

func (m *mockResourceInterpreter) Retain(desired *unstructured.Unstructured, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return desired, nil
}

func (m *mockResourceInterpreter) AggregateStatus(object *unstructured.Unstructured, _ []workv1alpha2.AggregatedStatusItem) (*unstructured.Unstructured, error) {
	return object, nil
}

func (m *mockResourceInterpreter) GetDependencies(_ *unstructured.Unstructured) ([]configv1alpha1.DependentObjectReference, error) {
	return nil, nil
}

func (m *mockResourceInterpreter) ReflectStatus(_ *unstructured.Unstructured) (*runtime.RawExtension, error) {
	return nil, nil
}

func (m *mockResourceInterpreter) InterpretHealth(_ *unstructured.Unstructured) (bool, error) {
	return true, nil
}
