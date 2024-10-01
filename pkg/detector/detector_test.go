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
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
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

func TestClaimPolicyForObject(t *testing.T) {
	tests := []struct {
		name                string
		object              *unstructured.Unstructured
		policy              *policyv1alpha1.PropagationPolicy
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name: "claim policy for object without existing labels",
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
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					Labels: map[string]string{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-id",
					},
				},
			},
			expectedLabels: map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-id",
			},
			expectedAnnotations: map[string]string{
				policyv1alpha1.PropagationPolicyNamespaceAnnotation: "default",
				policyv1alpha1.PropagationPolicyNameAnnotation:      "test-policy",
			},
		},
		{
			name: "claim policy for object with existing labels",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"labels": map[string]interface{}{
							"existing-label": "existing-value",
						},
					},
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					Labels: map[string]string{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-id",
					},
				},
			},
			expectedLabels: map[string]string{
				"existing-label": "existing-value",
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-id",
			},
			expectedAnnotations: map[string]string{
				policyv1alpha1.PropagationPolicyNamespaceAnnotation: "default",
				policyv1alpha1.PropagationPolicyNameAnnotation:      "test-policy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			mockCl := &mockClient{
				Client: fakeClient,
				updateFunc: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
					// This simulates the actual update behavior
					unstructuredObj := obj.(*unstructured.Unstructured)
					tt.object.Object = unstructuredObj.Object
					return nil
				},
			}

			d := &ResourceDetector{
				Client: mockCl,
			}

			policyID, err := d.ClaimPolicyForObject(tt.object, tt.policy)
			assert.NoError(t, err)
			assert.Equal(t, tt.policy.Labels[policyv1alpha1.PropagationPolicyPermanentIDLabel], policyID)

			assert.Equal(t, tt.expectedLabels, tt.object.GetLabels())
			assert.Equal(t, tt.expectedAnnotations, tt.object.GetAnnotations())
		})
	}
}

func TestClaimClusterPolicyForObject(t *testing.T) {
	tests := []struct {
		name                string
		object              *unstructured.Unstructured
		policy              *policyv1alpha1.ClusterPropagationPolicy
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name: "claim cluster policy for object without existing labels",
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
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-policy",
					Labels: map[string]string{
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-id",
					},
				},
			},
			expectedLabels: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-id",
			},
			expectedAnnotations: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyAnnotation: "test-cluster-policy",
			},
		},
		{
			name: "claim cluster policy for object with existing labels",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"labels": map[string]interface{}{
							"existing-label": "existing-value",
						},
					},
				},
			},
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-policy",
					Labels: map[string]string{
						policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-id",
					},
				},
			},
			expectedLabels: map[string]string{
				"existing-label": "existing-value",
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-id",
			},
			expectedAnnotations: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyAnnotation: "test-cluster-policy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			mockCl := &mockClient{
				Client: fakeClient,
				updateFunc: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
					// This simulates the actual update behavior
					unstructuredObj := obj.(*unstructured.Unstructured)
					tt.object.Object = unstructuredObj.Object
					return nil
				},
			}

			d := &ResourceDetector{
				Client: mockCl,
			}

			policyID, err := d.ClaimClusterPolicyForObject(tt.object, tt.policy)
			assert.NoError(t, err)
			assert.Equal(t, tt.policy.Labels[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel], policyID)

			assert.Equal(t, tt.expectedLabels, tt.object.GetLabels())
			assert.Equal(t, tt.expectedAnnotations, tt.object.GetAnnotations())
		})
	}
}

func TestBuildResourceBinding(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		labels      map[string]string
		annotations map[string]string
		policy      *policyv1alpha1.PropagationSpec
		expected    *workv1alpha2.ResourceBinding
	}{
		{
			name: "build resource binding",
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
			labels:      map[string]string{"test": "label"},
			annotations: map[string]string{"test": "annotation"},
			policy: &policyv1alpha1.PropagationSpec{
				PropagateDeps: true,
				SchedulerName: "test-scheduler",
				Placement:     policyv1alpha1.Placement{},
			},
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment-deployment",
					Namespace:   "default",
					Labels:      map[string]string{"test": "label"},
					Annotations: map[string]string{"test": "annotation"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "Deployment",
							Name:               "test-deployment",
							UID:                "test-uid",
							Controller:         func() *bool { b := true; return &b }(),
							BlockOwnerDeletion: func() *bool { b := true; return &b }(),
						},
					},
					Finalizers: []string{"karmada.io/binding-controller"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					PropagateDeps: true,
					SchedulerName: "test-scheduler",
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-deployment",
						Namespace:  "default",
						UID:        "test-uid",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				ResourceInterpreter: &mockResourceInterpreter{},
			}
			binding, err := d.BuildResourceBinding(tt.object, tt.labels, tt.annotations, tt.policy)

			if err != nil {
				t.Fatalf("BuildResourceBinding returned unexpected error: %v", err)
			}

			if binding == nil {
				t.Fatal("Expected non-nil binding, got nil")
			}

			assert.Equal(t, tt.expected.Name, binding.Name)
			assert.Equal(t, tt.expected.Namespace, binding.Namespace)
			assert.Equal(t, tt.expected.Labels, binding.Labels)
			assert.Equal(t, tt.expected.Annotations, binding.Annotations)
			assert.Equal(t, tt.expected.OwnerReferences, binding.OwnerReferences)
			assert.Equal(t, tt.expected.Finalizers, binding.Finalizers)
			assert.Equal(t, tt.expected.Spec.PropagateDeps, binding.Spec.PropagateDeps)
			assert.Equal(t, tt.expected.Spec.SchedulerName, binding.Spec.SchedulerName)
			assert.Equal(t, tt.expected.Spec.Resource, binding.Spec.Resource)

			// Check if Placement is set (it might be empty, but should be set)
			assert.NotNil(t, binding.Spec.Placement)
		})
	}
}

func TestBuildClusterResourceBinding(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		labels      map[string]string
		annotations map[string]string
		policy      *policyv1alpha1.PropagationSpec
		expected    *workv1alpha2.ClusterResourceBinding
	}{
		{
			name: "build cluster resource binding",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"kind":       "ClusterRole",
					"metadata": map[string]interface{}{
						"name": "test-clusterrole",
						"uid":  "test-uid",
					},
				},
			},
			labels:      map[string]string{"test": "label"},
			annotations: map[string]string{"test": "annotation"},
			policy: &policyv1alpha1.PropagationSpec{
				PropagateDeps: true,
				SchedulerName: "test-scheduler",
				Placement:     policyv1alpha1.Placement{},
			},
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-clusterrole-clusterrole",
					Labels:      map[string]string{"test": "label"},
					Annotations: map[string]string{"test": "annotation"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "rbac.authorization.k8s.io/v1",
							Kind:               "ClusterRole",
							Name:               "test-clusterrole",
							UID:                "test-uid",
							Controller:         func() *bool { b := true; return &b }(),
							BlockOwnerDeletion: func() *bool { b := true; return &b }(),
						},
					},
					Finalizers: []string{"karmada.io/cluster-resource-binding-controller"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					PropagateDeps: true,
					SchedulerName: "test-scheduler",
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "rbac.authorization.k8s.io/v1",
						Kind:       "ClusterRole",
						Name:       "test-clusterrole",
						UID:        "test-uid",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				ResourceInterpreter: &mockResourceInterpreter{},
			}
			binding, err := d.BuildClusterResourceBinding(tt.object, tt.labels, tt.annotations, tt.policy)

			if err != nil {
				t.Fatalf("BuildClusterResourceBinding returned unexpected error: %v", err)
			}

			if binding == nil {
				t.Fatal("Expected non-nil binding, got nil")
			}

			assert.Equal(t, tt.expected.Name, binding.Name)
			assert.Equal(t, tt.expected.Labels, binding.Labels)
			assert.Equal(t, tt.expected.Annotations, binding.Annotations)
			assert.Equal(t, tt.expected.OwnerReferences, binding.OwnerReferences)
			assert.Equal(t, tt.expected.Finalizers, binding.Finalizers)
			assert.Equal(t, tt.expected.Spec.PropagateDeps, binding.Spec.PropagateDeps)
			assert.Equal(t, tt.expected.Spec.SchedulerName, binding.Spec.SchedulerName)
			assert.Equal(t, tt.expected.Spec.Resource, binding.Spec.Resource)

			// Check if Placement is set (it might be empty, but should be set)
			assert.NotNil(t, binding.Spec.Placement)
		})
	}
}

func TestIsWaiting(t *testing.T) {
	tests := []struct {
		name     string
		key      keys.ClusterWideKey
		waiting  map[keys.ClusterWideKey]struct{}
		expected bool
	}{
		{
			name: "object is waiting",
			key:  keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			waiting: map[keys.ClusterWideKey]struct{}{
				{Namespace: "default", Name: "test-obj"}: {},
			},
			expected: true,
		},
		{
			name:     "object is not waiting",
			key:      keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			waiting:  map[keys.ClusterWideKey]struct{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				waitingObjects: tt.waiting,
			}
			result := d.isWaiting(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddWaiting(t *testing.T) {
	tests := []struct {
		name           string
		key            keys.ClusterWideKey
		initialWaiting map[keys.ClusterWideKey]struct{}
		expectedLength int
	}{
		{
			name:           "add new object to empty waiting list",
			key:            keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{},
			expectedLength: 1,
		},
		{
			name: "add new object to non-empty waiting list",
			key:  keys.ClusterWideKey{Namespace: "default", Name: "test-obj-2"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{
				{Namespace: "default", Name: "test-obj-1"}: {},
			},
			expectedLength: 2,
		},
		{
			name: "add existing object to waiting list",
			key:  keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{
				{Namespace: "default", Name: "test-obj"}: {},
			},
			expectedLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				waitingObjects: tt.initialWaiting,
			}
			d.AddWaiting(tt.key)
			assert.Equal(t, tt.expectedLength, len(d.waitingObjects))
			assert.Contains(t, d.waitingObjects, tt.key)
		})
	}
}

func TestRemoveWaiting(t *testing.T) {
	tests := []struct {
		name           string
		key            keys.ClusterWideKey
		initialWaiting map[keys.ClusterWideKey]struct{}
		expectedLength int
	}{
		{
			name: "remove existing object from waiting list",
			key:  keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{
				{Namespace: "default", Name: "test-obj"}: {},
			},
			expectedLength: 0,
		},
		{
			name: "remove non-existing object from waiting list",
			key:  keys.ClusterWideKey{Namespace: "default", Name: "test-obj-2"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{
				{Namespace: "default", Name: "test-obj-1"}: {},
			},
			expectedLength: 1,
		},
		{
			name:           "remove from empty waiting list",
			key:            keys.ClusterWideKey{Namespace: "default", Name: "test-obj"},
			initialWaiting: map[keys.ClusterWideKey]struct{}{},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResourceDetector{
				waitingObjects: tt.initialWaiting,
			}
			d.RemoveWaiting(tt.key)
			assert.Equal(t, tt.expectedLength, len(d.waitingObjects))
			assert.NotContains(t, d.waitingObjects, tt.key)
		})
	}
}

func TestOnPropagationPolicyAdd(t *testing.T) {
	mockWorker := &mockAsyncWorker{}
	d := &ResourceDetector{
		policyReconcileWorker: mockWorker,
	}

	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	d.OnPropagationPolicyAdd(policy)
	assert.Equal(t, 1, mockWorker.enqueueCount, "PropagationPolicy should be enqueued")
}

func TestOnPropagationPolicyUpdate(t *testing.T) {
	mockWorker := &mockAsyncWorker{}
	d := &ResourceDetector{
		policyReconcileWorker: mockWorker,
	}

	oldPolicy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	priority := int32(1)
	newPolicy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: policyv1alpha1.PropagationSpec{
			Priority: &priority,
		},
	}

	d.OnPropagationPolicyUpdate(oldPolicy, newPolicy)
	assert.Equal(t, 1, mockWorker.enqueueCount, "Updated PropagationPolicy should be enqueued")
}

func TestOnClusterPropagationPolicyAdd(t *testing.T) {
	mockWorker := &mockAsyncWorker{}
	d := &ResourceDetector{
		clusterPolicyReconcileWorker: mockWorker,
	}

	policy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-policy",
		},
	}

	d.OnClusterPropagationPolicyAdd(policy)
	assert.Equal(t, 1, mockWorker.enqueueCount, "ClusterPropagationPolicy should be enqueued")
}

func TestOnClusterPropagationPolicyUpdate(t *testing.T) {
	mockWorker := &mockAsyncWorker{}
	d := &ResourceDetector{
		clusterPolicyReconcileWorker: mockWorker,
	}

	oldPolicy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-policy",
		},
	}

	priority := int32(1)
	newPolicy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-policy",
		},
		Spec: policyv1alpha1.PropagationSpec{
			Priority: &priority,
		},
	}

	d.OnClusterPropagationPolicyUpdate(oldPolicy, newPolicy)
	assert.Equal(t, 1, mockWorker.enqueueCount, "Updated ClusterPropagationPolicy should be enqueued")
}

func TestHandlePropagationPolicyDeletion(t *testing.T) {
	tests := []struct {
		name      string
		policyID  string
		objects   []runtime.Object
		expectErr bool
	}{
		{
			name:     "delete propagation policy with existing bindings",
			policyID: "test-policy-id",
			objects: []runtime.Object{
				&workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-binding-1",
						Namespace: "default",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy-id",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test-deployment",
							Namespace:  "default",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name:      "delete propagation policy without bindings",
			policyID:  "non-existent-policy-id",
			objects:   []runtime.Object{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()

			dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			d := &ResourceDetector{
				Client:        fakeClient,
				DynamicClient: dynamicClient,
				RESTMapper:    &mockRESTMapper{},
			}

			err := d.HandlePropagationPolicyDeletion(tt.policyID)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify that the bindings have been cleaned up
			rbList := &workv1alpha2.ResourceBindingList{}
			err = fakeClient.List(context.TODO(), rbList, &client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{
					policyv1alpha1.PropagationPolicyPermanentIDLabel: tt.policyID,
				}),
			})
			assert.NoError(t, err)
			assert.Empty(t, rbList.Items, "All ResourceBindings should be cleaned up")
		})
	}
}

func TestHandleClusterPropagationPolicyDeletion(t *testing.T) {
	scheme := setupTestScheme()

	policyID := "test-policy-id"
	crb := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crb",
			Labels: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: policyID,
			},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-deployment",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(crb).Build()

	d := &ResourceDetector{
		Client:        fakeClient,
		RESTMapper:    &mockRESTMapper{},
		DynamicClient: &mockDynamicClient{},
	}

	err := d.HandleClusterPropagationPolicyDeletion(policyID)
	assert.NoError(t, err)

	// Check if the ClusterResourceBinding's labels were cleaned up
	updatedCRB := &workv1alpha2.ClusterResourceBinding{}
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: "test-crb"}, updatedCRB)
	assert.NoError(t, err)
	assert.NotContains(t, updatedCRB.Labels, policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel)
}

func TestHandlePropagationPolicyCreationOrUpdate(t *testing.T) {
	scheme := setupTestScheme()

	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			Labels: map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy-id",
			},
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeRecorder := record.NewFakeRecorder(10)
	mockProcessor := &mockAsyncWorker{}

	d := &ResourceDetector{
		Client:        fakeClient,
		EventRecorder: fakeRecorder,
		Processor:     mockProcessor,
	}

	err := d.HandlePropagationPolicyCreationOrUpdate(policy)
	assert.NoError(t, err)
}

func TestHandleClusterPropagationPolicyCreationOrUpdate(t *testing.T) {
	scheme := setupTestScheme()

	policy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-policy",
			Labels: map[string]string{
				policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "test-cluster-policy-id",
			},
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeRecorder := record.NewFakeRecorder(10)
	mockProcessor := &mockAsyncWorker{}

	d := &ResourceDetector{
		Client:        fakeClient,
		EventRecorder: fakeRecorder,
		Processor:     mockProcessor,
	}

	err := d.HandleClusterPropagationPolicyCreationOrUpdate(policy)
	assert.NoError(t, err)
}

func TestCleanupResourceBindingMarks(t *testing.T) {
	tests := []struct {
		name                string
		rb                  *workv1alpha2.ResourceBinding
		cleanupFunc         func(obj metav1.Object)
		updateError         error
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectError         bool
	}{
		{
			name: "cleanup resource binding marks",
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
					Labels: map[string]string{
						"test-label": "value",
						"keep-label": "keep-value",
					},
					Annotations: map[string]string{
						"test-annotation": "value",
						"keep-annotation": "keep-value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(map[string]string{"keep-label": "keep-value"})
				obj.SetAnnotations(map[string]string{"keep-annotation": "keep-value"})
			},
			expectedLabels: map[string]string{
				"keep-label": "keep-value",
			},
			expectedAnnotations: map[string]string{
				"keep-annotation": "keep-value",
			},
			expectError: false,
		},
		{
			name: "cleanup all marks",
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-2",
					Namespace: "default",
					Labels: map[string]string{
						"test-label": "value",
					},
					Annotations: map[string]string{
						"test-annotation": "value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(nil)
				obj.SetAnnotations(nil)
			},
			expectedLabels:      nil,
			expectedAnnotations: nil,
			expectError:         false,
		},
		{
			name: "handle update conflict",
			rb: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-3",
					Namespace: "default",
					Labels: map[string]string{
						"test-label": "value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(nil)
			},
			updateError: apierrors.NewConflict(schema.GroupResource{Group: "work.karmada.io", Resource: "resourcebindings"}, "test-binding-3", errors.New("conflict")),
			expectedLabels: map[string]string{
				"test-label": "value",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.rb).Build()

			d := &ResourceDetector{
				Client: &errorInjectingClient{
					Client:      fakeClient,
					injectError: tt.updateError,
				},
			}

			err := d.CleanupResourceBindingMarks(tt.rb, tt.cleanupFunc)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedRB := &workv1alpha2.ResourceBinding{}
				err = fakeClient.Get(context.TODO(), client.ObjectKey{Namespace: tt.rb.Namespace, Name: tt.rb.Name}, updatedRB)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLabels, updatedRB.Labels)
				assert.Equal(t, tt.expectedAnnotations, updatedRB.Annotations)
			}
		})
	}
}

func TestCleanupClusterResourceBindingMarks(t *testing.T) {
	tests := []struct {
		name                string
		crb                 *workv1alpha2.ClusterResourceBinding
		cleanupFunc         func(obj metav1.Object)
		updateError         error
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectError         bool
	}{
		{
			name: "cleanup cluster resource binding marks",
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding",
					Labels: map[string]string{
						"test-label": "value",
						"keep-label": "keep-value",
					},
					Annotations: map[string]string{
						"test-annotation": "value",
						"keep-annotation": "keep-value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(map[string]string{"keep-label": "keep-value"})
				obj.SetAnnotations(map[string]string{"keep-annotation": "keep-value"})
			},
			expectedLabels: map[string]string{
				"keep-label": "keep-value",
			},
			expectedAnnotations: map[string]string{
				"keep-annotation": "keep-value",
			},
			expectError: false,
		},
		{
			name: "cleanup all marks",
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-2",
					Labels: map[string]string{
						"test-label": "value",
					},
					Annotations: map[string]string{
						"test-annotation": "value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(nil)
				obj.SetAnnotations(nil)
			},
			expectedLabels:      nil,
			expectedAnnotations: nil,
			expectError:         false,
		},
		{
			name: "handle update conflict",
			crb: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-3",
					Labels: map[string]string{
						"test-label": "value",
					},
				},
			},
			cleanupFunc: func(obj metav1.Object) {
				obj.SetLabels(nil)
			},
			updateError: apierrors.NewConflict(schema.GroupResource{Group: "work.karmada.io", Resource: "clusterresourcebindings"}, "test-cluster-binding-3", errors.New("conflict")),
			expectedLabels: map[string]string{
				"test-label": "value",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.crb).Build()

			d := &ResourceDetector{
				Client: &errorInjectingClient{
					Client:      fakeClient,
					injectError: tt.updateError,
				},
			}

			err := d.CleanupClusterResourceBindingMarks(tt.crb, tt.cleanupFunc)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedCRB := &workv1alpha2.ClusterResourceBinding{}
				err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: tt.crb.Name}, updatedCRB)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLabels, updatedCRB.Labels)
				assert.Equal(t, tt.expectedAnnotations, updatedCRB.Annotations)
			}
		})
	}
}

//Helper Functions

// shallowCopyUnstructured creates a shallow copy of an Unstructured object
func shallowCopyUnstructured(obj *unstructured.Unstructured) *unstructured.Unstructured {
	newObj := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}
	for k, v := range obj.Object {
		if k == "metadata" {
			newObj.Object[k] = copyMetadata(v.(map[string]interface{}))
		} else {
			newObj.Object[k] = v
		}
	}
	return newObj
}

// copyMetadata creates a deep copy of the metadata map
func copyMetadata(metadata map[string]interface{}) map[string]interface{} {
	newMetadata := make(map[string]interface{})
	for k, v := range metadata {
		if k == "labels" || k == "annotations" {
			switch typedV := v.(type) {
			case map[string]string:
				newMap := make(map[string]string)
				for subK, subV := range typedV {
					newMap[subK] = subV
				}
				newMetadata[k] = newMap
			case map[string]interface{}:
				newMap := make(map[string]interface{})
				for subK, subV := range typedV {
					newMap[subK] = subV
				}
				newMetadata[k] = newMap
			default:
				// If it's neither map[string]string nor map[string]interface{},
				// just assign it as is
				newMetadata[k] = v
			}
		} else {
			newMetadata[k] = v
		}
	}
	return newMetadata
}

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

// mockDynamicClient is a custom implementation of dynamic.Interface that avoids deep copying
type mockDynamicClient struct {
	objects map[string]*unstructured.Unstructured
}

func (c *mockDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &mockNamespaceableResourceClient{client: c, resource: resource}
}

// mockNamespaceableResourceClient implements dynamic.NamespaceableResourceInterface
type mockNamespaceableResourceClient struct {
	client   *mockDynamicClient
	resource schema.GroupVersionResource
}

func (c *mockNamespaceableResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	return &mockResourceClient{client: c.client, namespace: ns, resource: c.resource}
}

func (c *mockNamespaceableResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.Namespace("").Create(ctx, obj, options, subresources...)
}

func (c *mockNamespaceableResourceClient) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.Namespace("").Update(ctx, obj, options, subresources...)
}

func (c *mockNamespaceableResourceClient) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return c.Namespace("").UpdateStatus(ctx, obj, options)
}

func (c *mockNamespaceableResourceClient) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return c.Namespace("").Delete(ctx, name, options, subresources...)
}

func (c *mockNamespaceableResourceClient) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.Namespace("").DeleteCollection(ctx, options, listOptions)
}

func (c *mockNamespaceableResourceClient) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.Namespace("").Get(ctx, name, options, subresources...)
}

func (c *mockNamespaceableResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return c.Namespace("").List(ctx, opts)
}

func (c *mockNamespaceableResourceClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Namespace("").Watch(ctx, opts)
}

func (c *mockNamespaceableResourceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.Namespace("").Patch(ctx, name, pt, data, options, subresources...)
}

func (c *mockNamespaceableResourceClient) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return c.Namespace("").Apply(ctx, name, obj, options, subresources...)
}

func (c *mockNamespaceableResourceClient) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return c.Namespace("").ApplyStatus(ctx, name, obj, options)
}

// mockResourceClient implements dynamic.ResourceInterface
type mockResourceClient struct {
	client    *mockDynamicClient
	namespace string
	resource  schema.GroupVersionResource
}

func (c *mockResourceClient) Create(_ context.Context, obj *unstructured.Unstructured, _ metav1.CreateOptions, _ ...string) (*unstructured.Unstructured, error) {
	key := fmt.Sprintf("%s/%s/%s", c.resource.Resource, c.namespace, obj.GetName())
	c.client.objects[key] = shallowCopyUnstructured(obj)
	return obj, nil
}

func (c *mockResourceClient) Update(_ context.Context, obj *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
	key := fmt.Sprintf("%s/%s/%s", c.resource.Resource, c.namespace, obj.GetName())
	c.client.objects[key] = shallowCopyUnstructured(obj)
	return obj, nil
}

func (c *mockResourceClient) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return c.Update(ctx, obj, options)
}

func (c *mockResourceClient) Delete(_ context.Context, name string, _ metav1.DeleteOptions, _ ...string) error {
	key := fmt.Sprintf("%s/%s/%s", c.resource.Resource, c.namespace, name)
	delete(c.client.objects, key)
	return nil
}

func (c *mockResourceClient) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	return nil
}

func (c *mockResourceClient) Get(_ context.Context, name string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	key := fmt.Sprintf("%s/%s/%s", c.resource.Resource, c.namespace, name)
	obj, ok := c.client.objects[key]
	if !ok {
		return nil, apierrors.NewNotFound(c.resource.GroupResource(), name)
	}
	return shallowCopyUnstructured(obj), nil
}

func (c *mockResourceClient) List(_ context.Context, _ metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{}, nil
}

func (c *mockResourceClient) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

func (c *mockResourceClient) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (c *mockResourceClient) Apply(ctx context.Context, _ string, obj *unstructured.Unstructured, _ metav1.ApplyOptions, _ ...string) (*unstructured.Unstructured, error) {
	return c.Update(ctx, obj, metav1.UpdateOptions{})
}

func (c *mockResourceClient) ApplyStatus(ctx context.Context, _ string, obj *unstructured.Unstructured, _ metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return c.UpdateStatus(ctx, obj, metav1.UpdateOptions{})
}

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

// mockClient is a mock implementation of client.Client
type mockClient struct {
	client.Client
	updateFunc func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

func (m *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return m.updateFunc(ctx, obj, opts...)
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

// errorInjectingClient is a wrapper around client.Client that allows injecting errors
type errorInjectingClient struct {
	client.Client
	injectError error
}

func (c *errorInjectingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.injectError != nil {
		return c.injectError
	}
	return c.Client.Update(ctx, obj, opts...)
}
