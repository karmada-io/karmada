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
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
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
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container1"}},
				},
			},
			newObj: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
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
		policies       []client.Object
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
			policies: []client.Object{
				&policyv1alpha1.PropagationPolicy{
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
				Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.policies...).Build(),
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
		policies       []client.Object
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
			policies: []client.Object{
				&policyv1alpha1.ClusterPropagationPolicy{
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
				Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.policies...).Build(),
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
				Spec: policyv1alpha1.PropagationSpec{
					Suspension: &policyv1alpha1.Suspension{Dispatching: ptr.To(true)},
				},
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
			err := fakeClient.Create(context.TODO(), tt.policy)
			assert.NoError(t, err)

			d := &ResourceDetector{
				Client:              fakeClient,
				DynamicClient:       fakeDynamicClient,
				EventRecorder:       fakeRecorder,
				ResourceInterpreter: &mockResourceInterpreter{},
				RESTMapper:          &mockRESTMapper{},
			}

			err = d.ApplyPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)

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
				Spec: policyv1alpha1.PropagationSpec{
					Suspension: &policyv1alpha1.Suspension{Dispatching: ptr.To(true)},
				},
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
				Spec: policyv1alpha1.PropagationSpec{
					Suspension: &policyv1alpha1.Suspension{Dispatching: ptr.To(true)},
				},
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

			d := &ResourceDetector{
				Client:              fakeClient,
				DynamicClient:       fakeDynamicClient,
				EventRecorder:       fakeRecorder,
				ResourceInterpreter: &mockResourceInterpreter{},
				RESTMapper:          &mockRESTMapper{},
			}
			err := fakeClient.Create(context.TODO(), tt.policy)
			assert.NoError(t, err)

			err = d.ApplyClusterPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

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
			}
		})
	}
}

// validateBasicBinding validates basic binding properties
func validateBasicBinding(t *testing.T, rb *workv1alpha2.ResourceBinding, expectedName, expectedNamespace string, expectedReplicas int32) {
	if rb.Name != names.GenerateBindingName("Deployment", expectedName) {
		t.Errorf("Expected binding name does not match")
	}
	if rb.Namespace != expectedNamespace {
		t.Errorf("Expected namespace does not match")
	}
	if rb.Spec.Replicas != expectedReplicas {
		t.Errorf("Expected replicas does not match")
	}
}

// validateSuspension validates suspension settings
func validateSuspension(t *testing.T, rb *workv1alpha2.ResourceBinding) {
	if rb.Spec.Suspension == nil {
		t.Errorf("Expected Suspension to be set")
	}
}

// validateReplicas validates replica settings
func validateReplicas(t *testing.T, rb *workv1alpha2.ResourceBinding, expectedReplicas int32) {
	if rb.Spec.Replicas != expectedReplicas {
		t.Errorf("Expected replicas to be %d", expectedReplicas)
	}
	if expectedReplicas == 0 && rb.Spec.ReplicaRequirements != nil {
		t.Errorf("Expected ReplicaRequirements to be nil")
	}
}

// validatePriority validates priority settings
func validatePriority(t *testing.T, rb *workv1alpha2.ResourceBinding) {
	if rb.Spec.SchedulePriority == nil {
		t.Errorf("Expected SchedulePriority to be set")
	}
	if rb.Spec.SchedulePriority.Priority != 1000 {
		t.Errorf("Expected priority to be 1000, got %d", rb.Spec.SchedulePriority.Priority)
	}
}

// validatePropagateDeps validates propagate dependencies settings
func validatePropagateDeps(t *testing.T, rb *workv1alpha2.ResourceBinding) {
	if !rb.Spec.PropagateDeps {
		t.Errorf("Expected PropagateDeps to be true")
	}
}

func TestBuildResourceBinding(t *testing.T) {
	tests := []struct {
		name                string
		object              *unstructured.Unstructured
		policyNamespace     string
		policyName          string
		policyID            string
		enabled             bool
		replicas            int32
		replicaRequirements *workv1alpha2.ReplicaRequirements
		claimFunc           func(object metav1.Object, policyId string, objectMeta metav1.ObjectMeta)
		wantErr             bool
		validate            func(t *testing.T, rb *workv1alpha2.ResourceBinding)
		setup               func(t *testing.T) error
		teardown            func(t *testing.T)
	}{
		{
			name: "Basic test case - PropagationPolicy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "test-ns",
			policyName:          "test-policy",
			policyID:            "test-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			claimFunc:           AddPPClaimMetadata,
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validateBasicBinding(t, rb, "test-deployment", "test-ns", 3)
			},
		},
		{
			name: "Basic test case - ClusterPropagationPolicy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "",
			policyName:          "test-cluster-policy",
			policyID:            "test-cluster-policy-id",
			enabled:             true,
			replicas:            5,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			claimFunc:           AddCPPClaimMetadata,
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validateBasicBinding(t, rb, "test-deployment", "test-ns", 5)
			},
		},
		{
			name: "Test with suspension policy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "test-ns",
			policyName:          "test-policy",
			policyID:            "test-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			claimFunc:           AddPPClaimMetadata,
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validateSuspension(t, rb)
			},
		},
		{
			name: "Test with disabled replicas",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "test-ns",
			policyName:          "test-policy",
			policyID:            "test-policy-id",
			enabled:             false,
			replicas:            0,
			replicaRequirements: nil,
			claimFunc:           AddPPClaimMetadata,
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validateReplicas(t, rb, 0)
			},
		},
		{
			name: "Test with priority scheduling",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "test-ns",
			policyName:          "test-policy",
			policyID:            "test-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			claimFunc:           AddPPClaimMetadata,
			setup: func(t *testing.T) error {
				originalValue := features.FeatureGate.Enabled(features.PriorityBasedScheduling)
				if err := features.FeatureGate.SetFromMap(map[string]bool{
					string(features.PriorityBasedScheduling): true,
				}); err != nil {
					return err
				}
				t.Cleanup(func() {
					if err := features.FeatureGate.SetFromMap(map[string]bool{
						string(features.PriorityBasedScheduling): originalValue,
					}); err != nil {
						t.Errorf("Failed to restore feature gate: %v", err)
					}
				})
				return nil
			},
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validatePriority(t, rb)
			},
		},
		{
			name: "Test with propagate dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			policyNamespace:     "test-ns",
			policyName:          "test-policy",
			policyID:            "test-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			claimFunc:           AddPPClaimMetadata,
			validate: func(t *testing.T, rb *workv1alpha2.ResourceBinding) {
				validatePropagateDeps(t, rb)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(t); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			detector := setupTestDetector()

			if tt.name == "Test with priority scheduling" {
				createTestPriorityClass(t, detector)
			}

			if tt.policyNamespace != "" {
				createTestPolicy(t, detector, tt.policyNamespace, tt.policyName)
			} else {
				createTestClusterPolicy(t, detector, tt.policyName)
			}

			rb, err := detector.BuildResourceBinding(
				tt.object,
				tt.policyNamespace,
				tt.policyName,
				tt.policyID,
				tt.enabled,
				tt.replicas,
				tt.replicaRequirements,
				tt.claimFunc,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("BuildResourceBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validate != nil {
				tt.validate(t, rb)
			}

			if tt.teardown != nil {
				tt.teardown(t)
			}
		})
	}
}

func TestBuildClusterResourceBinding(t *testing.T) {
	tests := []struct {
		name                string
		object              *unstructured.Unstructured
		policyName          string
		policyID            string
		enabled             bool
		replicas            int32
		replicaRequirements *workv1alpha2.ReplicaRequirements
		wantErr             bool
		validate            func(t *testing.T, crb *workv1alpha2.ClusterResourceBinding)
	}{
		{
			name: "Basic test case - enabled replicas",
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
			policyName:          "test-cluster-policy",
			policyID:            "test-cluster-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			validate: func(t *testing.T, crb *workv1alpha2.ClusterResourceBinding) {
				if crb.Name != names.GenerateBindingName("ClusterRole", "test-cluster-role") {
					t.Errorf("Expected binding name does not match")
				}
				if crb.Spec.Replicas != 3 {
					t.Errorf("Expected replicas does not match")
				}
				if crb.Spec.ReplicaRequirements == nil {
					t.Errorf("Expected ReplicaRequirements to be set")
				}
			},
		},
		{
			name: "Basic test case - disabled replicas",
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
			policyName:          "test-cluster-policy",
			policyID:            "test-cluster-policy-id",
			enabled:             false,
			replicas:            0,
			replicaRequirements: nil,
			validate: func(t *testing.T, crb *workv1alpha2.ClusterResourceBinding) {
				if crb.Name != names.GenerateBindingName("ClusterRole", "test-cluster-role") {
					t.Errorf("Expected binding name does not match")
				}
				if crb.Spec.Replicas != 0 {
					t.Errorf("Expected replicas to be 0")
				}
				if crb.Spec.ReplicaRequirements != nil {
					t.Errorf("Expected ReplicaRequirements to be nil")
				}
			},
		},
		{
			name: "Test with suspension policy",
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
			policyName:          "test-cluster-policy",
			policyID:            "test-cluster-policy-id",
			enabled:             true,
			replicas:            3,
			replicaRequirements: &workv1alpha2.ReplicaRequirements{},
			validate: func(t *testing.T, crb *workv1alpha2.ClusterResourceBinding) {
				if crb.Spec.Suspension == nil {
					t.Errorf("Expected Suspension to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			s.AddKnownTypes(policyv1alpha1.SchemeGroupVersion, &policyv1alpha1.ClusterPropagationPolicy{})

			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
			dynamicClient := dynamicfake.NewSimpleDynamicClient(s)

			detector := &ResourceDetector{
				Client:          fakeClient,
				DynamicClient:   dynamicClient,
				EventRecorder:   &record.FakeRecorder{},
				InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), dynamicClient, 0*time.Second),
			}

			// Create ClusterPropagationPolicy with suspension
			policy := &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.policyName,
				},
				Spec: policyv1alpha1.PropagationSpec{
					Suspension: &policyv1alpha1.Suspension{
						Dispatching: ptr.To(true),
					},
				},
			}
			if err := fakeClient.Create(context.TODO(), policy); err != nil {
				t.Fatalf("Failed to create ClusterPropagationPolicy: %v", err)
			}

			crb, err := detector.BuildClusterResourceBinding(
				tt.object,
				tt.policyName,
				tt.policyID,
				tt.enabled,
				tt.replicas,
				tt.replicaRequirements,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("BuildClusterResourceBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validate != nil {
				tt.validate(t, crb)
			}
		})
	}
}

// Helper Functions

// setupTestScheme creates a runtime scheme with necessary types for testing
func setupTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha2.Install(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = policyv1alpha1.Install(scheme)
	return scheme
}

// setupTestDetector creates a ResourceDetector instance for testing
func setupTestDetector() *ResourceDetector {
	s := scheme.Scheme
	s.AddKnownTypes(policyv1alpha1.SchemeGroupVersion, &policyv1alpha1.PropagationPolicy{})
	s.AddKnownTypes(policyv1alpha1.SchemeGroupVersion, &policyv1alpha1.ClusterPropagationPolicy{})
	s.AddKnownTypes(schedulingv1.SchemeGroupVersion, &schedulingv1.PriorityClass{})

	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(s)

	return &ResourceDetector{
		Client:          fakeClient,
		DynamicClient:   dynamicClient,
		EventRecorder:   &record.FakeRecorder{},
		InformerManager: genericmanager.NewSingleClusterInformerManager(context.TODO(), dynamicClient, 0*time.Second),
	}
}

// createTestPolicy creates a policy for testing
func createTestPolicy(t *testing.T, detector *ResourceDetector, namespace, name string) {
	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			Suspension: &policyv1alpha1.Suspension{
				Dispatching: ptr.To(true),
			},
			PropagateDeps: true,
			SchedulePriority: &policyv1alpha1.SchedulePriority{
				PriorityClassSource: policyv1alpha1.KubePriorityClass,
				PriorityClassName:   "high-priority",
			},
		},
	}
	if err := detector.Client.Create(context.TODO(), policy); err != nil {
		t.Fatalf("Failed to create PropagationPolicy: %v", err)
	}
}

// createTestClusterPolicy creates a cluster policy for testing
func createTestClusterPolicy(t *testing.T, detector *ResourceDetector, name string) {
	policy := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			Suspension: &policyv1alpha1.Suspension{
				Dispatching: ptr.To(true),
			},
			PropagateDeps: true,
			SchedulePriority: &policyv1alpha1.SchedulePriority{
				PriorityClassSource: policyv1alpha1.KubePriorityClass,
				PriorityClassName:   "high-priority",
			},
		},
	}
	if err := detector.Client.Create(context.TODO(), policy); err != nil {
		t.Fatalf("Failed to create ClusterPropagationPolicy: %v", err)
	}
}

// createTestPriorityClass creates a priority class for testing
func createTestPriorityClass(t *testing.T, detector *ResourceDetector) {
	priorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "high-priority",
		},
		Value: 1000,
	}
	if err := detector.Client.Create(context.TODO(), priorityClass); err != nil {
		t.Fatalf("Failed to create PriorityClass: %v", err)
	}
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

func (m *mockAsyncWorker) Run(_ context.Context, _ int) {}

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
