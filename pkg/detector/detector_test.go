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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func BenchmarkEventFilterNoSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = nil
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "benchmark",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterNoWildcards(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterPrefixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^benchmark-.*$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}
func BenchmarkEventFilterSuffixMatchSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "example-benchmark",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterMultiSkipNameSpaces(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^.*-benchmark$"), regexp.MustCompile("^benchmark-.*$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "demo-deployment",
					"namespace": "benchmark-1",
				},
				"spec": map[string]any{
					"replicas": 2,
				},
			},
		})
	}
}

func BenchmarkEventFilterExtensionApiserverAuthentication(b *testing.B) {
	dt := &ResourceDetector{}
	dt.SkippedPropagatingNamespaces = append(dt.SkippedPropagatingNamespaces, regexp.MustCompile("^kube-.*$"))
	for b.Loop() {
		dt.EventFilter(&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Node",
					"metadata": map[string]any{
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
		obj             any
		expectedEnqueue bool
		isInInitialList bool
	}{
		{
			name: "valid unstructured object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectedEnqueue: true,
		},
		{
			name: "valid unstructured object, with low priority",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			expectedEnqueue: true,
			isInInitialList: true,
		},
		{
			name: "invalid unstructured object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{},
			},
			expectedEnqueue: false,
		},
		{
			name:            "non-runtime object",
			obj:             "not a runtime.Object",
			expectedEnqueue: false,
		},
		{
			name: "core v1 object",
			obj: &corev1.Pod{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
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
			d.OnAdd(tt.obj, tt.isInInitialList)
			if tt.expectedEnqueue {
				assert.Equal(t, 1, mockProcessor.enqueueCount, "Object should be enqueued")
				key, err := keys.ClusterWideKeyFunc(tt.obj)
				assert.NoError(t, err)
				assert.Equal(t, key, mockProcessor.lastEnqueued, "Object should be enqueued by its key")
				byKarmada, pending := d.takeResourceChange(key)
				assert.True(t, pending)
				assert.False(t, byKarmada, "Object additions require propagation")
			} else {
				assert.Equal(t, 0, mockProcessor.enqueueCount, "Object should not be enqueued")
			}
		})
	}
}

func TestOnUpdate(t *testing.T) {
	// newClaimedObject builds a resource template claimed by the given policy. "generation" is bumped by
	// Karmada, while "replicas" is the spec set by users.
	newClaimedObject := func(policyName string, generation, replicas int64) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"namespace":  "default",
					"name":       "test-deployment",
					"generation": generation,
					"annotations": map[string]any{
						policyv1alpha1.PropagationPolicyNamespaceAnnotation: "default",
						policyv1alpha1.PropagationPolicyNameAnnotation:      policyName,
					},
					"labels": map[string]any{
						policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy-id",
					},
				},
				"spec": map[string]any{
					"replicas": replicas,
				},
			},
		}
	}

	tests := []struct {
		name                      string
		oldObj                    any
		newObj                    any
		expectedEnqueue           bool
		expectChangeByKarmada     bool
		expectToUnstructuredError bool
	}{
		{
			name: "valid update with changes",
			oldObj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]any{
						"replicas": int64(1),
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]any{
						"replicas": int64(2),
					},
				},
			},
			expectedEnqueue: true,
		},
		{
			name: "update without changes",
			oldObj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]any{
						"replicas": int64(1),
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"spec": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			newObj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"annotations": map[string]any{
							util.PolicyPlacementAnnotation: "test",
						},
					},
				},
			},
			expectedEnqueue: true,
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
		{
			name:                  "lazy policy, only the fields changed by Karmada are changed",
			oldObj:                newClaimedObject("lazy-policy", 1, 2),
			newObj:                newClaimedObject("lazy-policy", 2, 2),
			expectedEnqueue:       true,
			expectChangeByKarmada: true,
		},
		{
			name:                  "lazy policy, specification is changed by users",
			oldObj:                newClaimedObject("lazy-policy", 1, 2),
			newObj:                newClaimedObject("lazy-policy", 2, 3),
			expectedEnqueue:       true,
			expectChangeByKarmada: false,
		},
	}

	lazyPolicy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "lazy-policy"},
		Spec:       policyv1alpha1.PropagationSpec{ActivationPreference: policyv1alpha1.LazyActivation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockAsyncWorker{}
			d := &ResourceDetector{
				Client:    fake.NewClientBuilder().WithScheme(setupTestScheme()).WithObjects(lazyPolicy).Build(),
				Processor: mockProcessor,
			}

			d.OnUpdate(tt.oldObj, tt.newObj)

			if tt.expectedEnqueue {
				assert.Equal(t, 1, mockProcessor.enqueueCount, "Object should be enqueued")
				key, err := keys.ClusterWideKeyFunc(tt.newObj)
				assert.NoError(t, err)
				assert.Equal(t, key, mockProcessor.lastEnqueued, "Object should be enqueued by its key")
				byKarmada, pending := d.takeResourceChange(key)
				assert.True(t, pending)
				assert.Equal(t, tt.expectChangeByKarmada, byKarmada, "Unexpected changed-by-Karmada mark")
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
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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
				Object: map[string]any{},
			},
			expectedEnqueue: false,
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
				key, err := keys.ClusterWideKeyFunc(tt.obj)
				assert.NoError(t, err)
				byKarmada, pending := d.takeResourceChange(key)
				assert.True(t, pending)
				assert.False(t, byKarmada, "Object deletions require reconciliation")
			} else {
				assert.Equal(t, 0, mockProcessor.enqueueCount, "Object should not be enqueued")
			}
		})
	}
}

func TestReconcileConsumesChangedByKarmadaMark(t *testing.T) {
	key := keys.ClusterWideKey{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-deployment",
	}

	t.Run("the mark is cleared once consumed", func(t *testing.T) {
		detector := &ResourceDetector{}
		detector.markResourceChange(key, true)

		byKarmada, pending := detector.takeResourceChange(key)
		assert.True(t, pending)
		assert.True(t, byKarmada)
		_, pending = detector.takeResourceChange(key)
		assert.False(t, pending, "The mark should only be consumed once")
	})

	t.Run("the mark is restored for the retry when the reconciliation fails", func(t *testing.T) {
		detector := &ResourceDetector{RESTMapper: &failingRESTMapper{}}
		detector.markResourceChange(key, true)

		if err := detector.Reconcile(key); err == nil {
			t.Fatal("Reconcile() error = nil, want error as the object can't be retrieved")
		}

		// The flag used to be part of the queue key and was therefore kept across retries.
		if mark, loaded := detector.resourceChangeMarks.Load(key); !loaded || mark != true {
			t.Errorf("Reconcile() mark = %v, loaded = %v, want the changed-by-Karmada mark to be restored", mark, loaded)
		}
	})

	t.Run("the user change is restored for the retry", func(t *testing.T) {
		detector := &ResourceDetector{RESTMapper: &failingRESTMapper{}}
		detector.markResourceChange(key, false)

		if err := detector.Reconcile(key); err == nil {
			t.Fatal("Reconcile() error = nil, want error as the object can't be retrieved")
		}

		if mark, loaded := detector.resourceChangeMarks.Load(key); !loaded || mark != false {
			t.Errorf("Reconcile() mark = %v, loaded = %v, want the user change to be restored", mark, loaded)
		}
	})

	t.Run("no pending change", func(t *testing.T) {
		// No dependencies are needed when a stale retry has no changes left to process.
		detector := &ResourceDetector{}
		assert.NoError(t, detector.Reconcile(key))
	})

	t.Run("invalid key", func(t *testing.T) {
		detector := &ResourceDetector{RESTMapper: &failingRESTMapper{}}
		detector.markResourceChange(key, true)

		if err := detector.Reconcile("invalid key"); err == nil {
			t.Fatal("Reconcile() error = nil, want error for an invalid key")
		}

		// Only the reconciliation of the resource itself is allowed to consume its mark.
		if _, loaded := detector.resourceChangeMarks.Load(key); !loaded {
			t.Error("Reconcile() consumed the mark for an invalid key")
		}
	})
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
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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

func verifyWorkloadAffinity(t *testing.T, object *unstructured.Unstructured, policySpec *policyv1alpha1.PropagationSpec, bindingSpec *workv1alpha2.ResourceBindingSpec) {
	if policySpec.Placement.WorkloadAffinity == nil {
		assert.Nil(t, bindingSpec.WorkloadAffinityGroups)
		return
	}

	expectedAffinityGroup := ""
	if affinityTerm := policySpec.Placement.WorkloadAffinity.Affinity; affinityTerm != nil {
		if affinityValue, ok := object.GetLabels()[affinityTerm.GroupByLabelKey]; ok {
			expectedAffinityGroup = fmt.Sprintf("%s=%s", affinityTerm.GroupByLabelKey, affinityValue)
		}
	}

	expectedAntiAffinityGroup := ""
	if antiAffinityTerm := policySpec.Placement.WorkloadAffinity.AntiAffinity; antiAffinityTerm != nil {
		if antiAffinityValue, ok := object.GetLabels()[antiAffinityTerm.GroupByLabelKey]; ok {
			expectedAntiAffinityGroup = fmt.Sprintf("%s=%s", antiAffinityTerm.GroupByLabelKey, antiAffinityValue)
		}
	}

	if expectedAffinityGroup == "" && expectedAntiAffinityGroup == "" {
		assert.Nil(t, bindingSpec.WorkloadAffinityGroups)
		return
	}

	assert.NotNil(t, bindingSpec.WorkloadAffinityGroups)
	assert.Equal(t, expectedAffinityGroup, bindingSpec.WorkloadAffinityGroups.AffinityGroup)
	assert.Equal(t, expectedAntiAffinityGroup, bindingSpec.WorkloadAffinityGroups.AntiAffinityGroup)
}

func TestApplyPolicy(t *testing.T) {
	tests := []struct {
		name                    string
		object                  *unstructured.Unstructured
		policy                  *policyv1alpha1.PropagationPolicy
		resourceChangeByKarmada bool
		expectError             bool
		enableWorkloadAffinity  bool
	}{
		{
			name: "basic apply policy",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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
			enableWorkloadAffinity:  false,
		},
		{
			name: "both affinity and antiAffinity label exists",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
						"labels": map[string]any{
							"affinityKey":     "affinityValue",
							"antiAffinityKey": "antiAffinityValue",
						},
					},
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
							Affinity:     &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: "affinityKey"},
							AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: "antiAffinityKey"},
						},
					},
				},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
			enableWorkloadAffinity:  true,
		},
		{
			name: "Only Affinity rule and label exists",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
						"labels": map[string]any{
							"affinityKey": "affinityValue",
						},
					},
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
							Affinity: &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: "affinityKey"},
						},
					},
				},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
			enableWorkloadAffinity:  true,
		},
		{
			name: "Only Anti Affinity rule and label exists",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
						"labels": map[string]any{
							"antiAffinityKey": "antiAffinityValue",
						},
					},
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
							AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: "antiAffinityKey"},
						},
					},
				},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
			enableWorkloadAffinity:  true,
		},
		{
			name: "Affinity/AntiAffinity rule exists but label missing",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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
					Placement: policyv1alpha1.Placement{
						WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
							Affinity:     &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: "affinityKey"},
							AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: "antiAffinityKey"},
						},
					},
				},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
			enableWorkloadAffinity:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.object).Build()
			fakeRecorder := record.NewFakeRecorder(10)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

			d := &ResourceDetector{
				Client:              fakeClient,
				DynamicClient:       fakeDynamicClient,
				EventRecorder:       fakeRecorder,
				ResourceInterpreter: &mockResourceInterpreter{},
				RESTMapper:          &mockRESTMapper{},
			}

			if err := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.WorkloadAffinity, tt.enableWorkloadAffinity)); err != nil {
				t.Fatalf("Failed to set feature gate %s to %v: %v", features.WorkloadAffinity, tt.enableWorkloadAffinity, err)
			}

			err := d.ApplyPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)

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

				verifyWorkloadAffinity(t, tt.object, &tt.policy.Spec, &binding.Spec)
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
		enableWorkloadAffinity  bool
	}{
		{
			name: "apply cluster policy for namespaced resource",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
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
			enableWorkloadAffinity:  false,
		},
		{
			name: "apply cluster policy for cluster-scoped resource",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "rbac.authorization.k8s.io/v1",
					"kind":       "ClusterRole",
					"metadata": map[string]any{
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
			enableWorkloadAffinity:  false,
		},
		{
			name: "both affinity and antiAffinity label exists",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
						"labels": map[string]any{
							"affinityKey":     "affinityValue",
							"antiAffinityKey": "antiAffinityValue",
						},
					},
				},
			},
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						WorkloadAffinity: &policyv1alpha1.WorkloadAffinity{
							Affinity:     &policyv1alpha1.WorkloadAffinityTerm{GroupByLabelKey: "affinityKey"},
							AntiAffinity: &policyv1alpha1.WorkloadAntiAffinityTerm{GroupByLabelKey: "antiAffinityKey"},
						},
					},
				},
			},
			resourceChangeByKarmada: false,
			expectError:             false,
			enableWorkloadAffinity:  true,
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

			if err := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.WorkloadAffinity, tt.enableWorkloadAffinity)); err != nil {
				t.Fatalf("Failed to set feature gate %s to %v: %v", features.WorkloadAffinity, tt.enableWorkloadAffinity, err)
			}

			err := d.ApplyClusterPolicy(tt.object, keys.ClusterWideKey{}, tt.resourceChangeByKarmada, tt.policy)

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
					verifyWorkloadAffinity(t, tt.object, &tt.policy.Spec, &binding.Spec)
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

func TestBuildResourceBindingWithSchedulePriorityPreemptionPolicy(t *testing.T) {
	originalFeatureGate := features.FeatureGate.DeepCopy()
	t.Cleanup(func() {
		features.FeatureGate = originalFeatureGate
	})

	preemptLowerPriority := corev1.PreemptLowerPriority
	preemptNever := corev1.PreemptNever
	unknownPreemptionPolicy := corev1.PreemptionPolicy("Unknown")
	wantPreemptLowerPriority := workv1alpha2.PreemptLowerPriority

	tests := []struct {
		name                           string
		priorityBasedSchedulingEnabled bool
		preemptiveSchedulingEnabled    bool
		priorityClassPreemptionPolicy  *corev1.PreemptionPolicy
		priorityClassExists            bool
		wantSchedulePriority           bool
		wantBindingPreemptionPolicy    *workv1alpha2.PreemptionPolicy
		wantErr                        bool
	}{
		{
			name:                           "API-defaulted or explicit preempt lower priority maps when both feature gates are enabled",
			priorityBasedSchedulingEnabled: true,
			preemptiveSchedulingEnabled:    true,
			priorityClassPreemptionPolicy:  &preemptLowerPriority,
			priorityClassExists:            true,
			wantSchedulePriority:           true,
			wantBindingPreemptionPolicy:    &wantPreemptLowerPriority,
		},
		{
			name:                           "never leaves binding preemption policy unset",
			priorityBasedSchedulingEnabled: true,
			preemptiveSchedulingEnabled:    true,
			priorityClassPreemptionPolicy:  &preemptNever,
			priorityClassExists:            true,
			wantSchedulePriority:           true,
		},
		{
			name:                           "nil priority class preemption policy leaves binding policy unset",
			priorityBasedSchedulingEnabled: true,
			preemptiveSchedulingEnabled:    true,
			priorityClassExists:            true,
			wantSchedulePriority:           true,
		},
		{
			name:                           "unknown priority class preemption policy leaves binding policy unset",
			priorityBasedSchedulingEnabled: true,
			preemptiveSchedulingEnabled:    true,
			priorityClassPreemptionPolicy:  &unknownPreemptionPolicy,
			priorityClassExists:            true,
			wantSchedulePriority:           true,
		},
		{
			name:                           "disabled preemptive scheduling leaves binding policy unset",
			priorityBasedSchedulingEnabled: true,
			priorityClassPreemptionPolicy:  &preemptLowerPriority,
			priorityClassExists:            true,
			wantSchedulePriority:           true,
		},
		{
			name:                          "disabled priority based scheduling leaves schedule priority unset",
			preemptiveSchedulingEnabled:   true,
			priorityClassPreemptionPolicy: &preemptLowerPriority,
			priorityClassExists:           true,
		},
		{
			name:                           "missing priority class returns an error",
			priorityBasedSchedulingEnabled: true,
			preemptiveSchedulingEnabled:    true,
			wantErr:                        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := features.FeatureGate.Set(fmt.Sprintf("%s=%t,%s=%t",
				features.PriorityBasedScheduling,
				test.priorityBasedSchedulingEnabled,
				features.PriorityBasedPreemptiveScheduling,
				test.preemptiveSchedulingEnabled,
			)); err != nil {
				t.Fatalf("failed to set feature gates: %v", err)
			}

			const priorityClassName = "high-priority"
			scheme := setupTestScheme()
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			if test.priorityClassExists {
				clientBuilder = clientBuilder.WithObjects(&schedulingv1.PriorityClass{
					ObjectMeta:       metav1.ObjectMeta{Name: priorityClassName},
					Value:            1000,
					PreemptionPolicy: test.priorityClassPreemptionPolicy,
				})
			}

			detector := &ResourceDetector{
				Client:              clientBuilder.Build(),
				EventRecorder:       record.NewFakeRecorder(10),
				ResourceInterpreter: &mockResourceInterpreter{},
			}
			object := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "test-deployment",
						"namespace": "default",
						"uid":       "test-uid",
					},
				},
			}
			policySpec := &policyv1alpha1.PropagationSpec{
				SchedulePriority: &policyv1alpha1.SchedulePriority{
					PriorityClassSource: policyv1alpha1.KubePriorityClass,
					PriorityClassName:   priorityClassName,
				},
			}

			binding, err := detector.BuildResourceBinding(object, policySpec, "policy-id", metav1.ObjectMeta{}, func(_ metav1.Object, _ string, _ metav1.ObjectMeta) {})
			if test.wantErr {
				assert.Error(t, err)
				assert.Nil(t, binding)
				return
			}

			if !assert.NoError(t, err) || !assert.NotNil(t, binding) {
				return
			}
			if !test.wantSchedulePriority {
				assert.Nil(t, binding.Spec.SchedulePriority)
				return
			}

			if assert.NotNil(t, binding.Spec.SchedulePriority) {
				assert.Equal(t, int32(1000), binding.Spec.SchedulePriority.Priority)
				assert.Equal(t, test.wantBindingPreemptionPolicy, binding.Spec.SchedulePriority.PreemptionPolicy)
			}
		})
	}
}

func TestEnqueueResourceTemplateForPolicyChange(t *testing.T) {
	testClusterWideKey := keys.ClusterWideKey{
		Group:     "foo",
		Version:   "foo",
		Kind:      "foo",
		Namespace: "foo",
		Name:      "foo",
	}
	tests := []struct {
		name       string
		pref       policyv1alpha1.ActivationPreference
		expectMark bool
	}{
		{
			name:       "lazy policy marks the change as made by Karmada",
			pref:       policyv1alpha1.LazyActivation,
			expectMark: true,
		},
		{
			name:       "non-lazy policy doesn't mark the change as made by Karmada",
			pref:       "",
			expectMark: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProcessor := &mockAsyncWorker{}
			detector := &ResourceDetector{Processor: mockProcessor}

			detector.enqueueResourceTemplateForPolicyChange(testClusterWideKey, tt.pref)

			assert.Equal(t, 1, mockProcessor.enqueueCount, "Resource template should be enqueued")
			assert.Equal(t, testClusterWideKey, mockProcessor.lastEnqueued, "Resource template should be enqueued by its own key")
			byKarmada, pending := detector.takeResourceChange(testClusterWideKey)
			assert.True(t, pending)
			assert.Equal(t, tt.expectMark, byKarmada, "Unexpected changed-by-Karmada mark")
		})
	}
}

// Helper Functions

// setupTestScheme creates a runtime scheme with necessary types for testing
func setupTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha2.Install(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = schedulingv1.AddToScheme(scheme)
	_ = policyv1alpha1.Install(scheme)
	return scheme
}

// Mock implementations

// mockAsyncWorker is a mock implementation of util.AsyncPriorityWorker
type mockAsyncWorker struct {
	enqueueCount int
	lastEnqueued any
}

func (m *mockAsyncWorker) Enqueue(item any) {
	m.enqueueCount++
	m.lastEnqueued = item
}

func (m *mockAsyncWorker) Add(item any) {
	m.enqueueCount++
	m.lastEnqueued = item
}
func (m *mockAsyncWorker) AddAfter(_ any, _ time.Duration) {}
func (m *mockAsyncWorker) AddWithOpts(_ util.AddOpts, items ...any) {
	for _, item := range items {
		m.Add(item)
	}
}
func (m *mockAsyncWorker) EnqueueWithOpts(_ util.AddOpts, item any) {
	m.Enqueue(item)
}

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

func (m *mockResourceInterpreter) GetComponents(_ *unstructured.Unstructured) ([]workv1alpha2.Component, error) {
	return nil, nil
}

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

// failingRESTMapper is a mock RESTMapper whose RESTMapping always fails.
type failingRESTMapper struct {
	mockRESTMapper
}

func (f *failingRESTMapper) RESTMapping(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	return nil, fmt.Errorf("fake error")
}

func TestResourceChangeMerging(t *testing.T) {
	for _, marks := range [][]bool{nil, {true}, {false}, {true, true}, {false, true}, {true, false}} {
		t.Run(fmt.Sprint(marks), func(t *testing.T) {
			d := &ResourceDetector{}
			key := keys.ClusterWideKey{Name: "test"}
			wantByKarmada := len(marks) > 0
			for _, mark := range marks {
				d.markResourceChange(key, mark)
				wantByKarmada = wantByKarmada && mark
			}
			byKarmada, pending := d.takeResourceChange(key)
			assert.Equal(t, len(marks) > 0, pending)
			assert.Equal(t, wantByKarmada, byKarmada)
			_, pending = d.takeResourceChange(key)
			assert.False(t, pending)
		})
	}
}

func TestReconcileSkipsLazyBindingForKarmadaChange(t *testing.T) {
	d, _, key, bindingKey := newResourceChangeTestDetector(t)
	d.addResourceTemplate(key, true, util.AddOpts{})

	require.NoError(t, d.Reconcile(key))
	assertResourceChangeBinding(t, d, bindingKey, false)
	_, pending := d.takeResourceChange(key)
	assert.False(t, pending, "The pending mark should be consumed by the reconciliation")
}

func TestResourceChangeConcurrentMarks(t *testing.T) {
	const writers = 64
	key := keys.ClusterWideKey{Name: "test"}

	t.Run("a user change wins", func(t *testing.T) {
		d := &ResourceDetector{}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func(byKarmada bool) {
				defer wg.Done()
				<-start
				d.markResourceChange(key, byKarmada)
			}(i > 0) // The first writer marks a user change, the others mark Karmada changes.
		}
		close(start)
		wg.Wait()

		mark, pending := d.takeResourceChange(key)
		assert.True(t, pending)
		assert.False(t, mark, "A user change must not be hidden by concurrent changes made by Karmada")
	})

	t.Run("all changes made by Karmada", func(t *testing.T) {
		d := &ResourceDetector{}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range writers {
			wg.Go(func() {
				<-start
				d.markResourceChange(key, true)
			})
		}
		close(start)
		wg.Wait()

		mark, pending := d.takeResourceChange(key)
		assert.True(t, pending)
		assert.True(t, mark)
	})
}

func TestResourceChangeDuringFailedReconcile(t *testing.T) {
	for _, initialByKarmada := range []bool{false, true} {
		for _, newByKarmada := range []bool{false, true} {
			t.Run(fmt.Sprintf("initial=%t,new=%t", initialByKarmada, newByKarmada), func(t *testing.T) {
				d, _, key, bindingKey := newResourceChangeTestDetector(t)
				d.addResourceTemplate(key, initialByKarmada, util.AddOpts{})
				d.RESTMapper = &resourceChangeRESTMapper{
					RESTMapper: &mockRESTMapper{},
					beforeMapping: func() error {
						d.addResourceTemplate(key, newByKarmada, util.AddOpts{})
						return fmt.Errorf("temporary mapping failure")
					},
				}
				require.Error(t, d.Reconcile(key))
				mark, pending := d.resourceChangeMarks.Load(key)
				require.True(t, pending, "Failed changes must remain pending")
				assert.Equal(t, initialByKarmada && newByKarmada, mark)

				d.RESTMapper = &mockRESTMapper{}
				require.NoError(t, d.Reconcile(key))
				assertResourceChangeBinding(t, d, bindingKey, !initialByKarmada || !newByKarmada)
				_, pending = d.takeResourceChange(key)
				assert.False(t, pending)
			})
		}
	}
}

func TestResourceChangeArrivesDuringReconcile(t *testing.T) {
	d, obj, key, bindingKey := newResourceChangeTestDetector(t)
	indexer := d.InformerManager.(*resourceChangeInformerManager).indexer

	// The pending change is made by Karmada, so the lazy binding is expected to be left untouched.
	d.addResourceTemplate(key, true, util.AddOpts{})

	var injected bool
	d.RESTMapper = &resourceChangeRESTMapper{
		RESTMapper: &mockRESTMapper{},
		beforeMapping: func() error {
			if injected {
				return nil
			}
			injected = true
			// A user change arrives after the pending mark was consumed but before this reconciliation finishes.
			updated := obj.DeepCopy()
			updated.SetResourceVersion("3")
			if err := unstructured.SetNestedField(updated.Object, int64(3), "spec", "replicas"); err != nil {
				return err
			}
			if err := indexer.Update(updated); err != nil {
				return err
			}
			d.OnUpdate(obj, updated)
			return nil
		},
	}

	require.NoError(t, d.Reconcile(key))
	binding := &workv1alpha2.ResourceBinding{}
	require.NoError(t, d.Client.Get(t.Context(), bindingKey, binding))
	assert.Equal(t, "1", binding.Spec.Resource.ResourceVersion, "The change made by Karmada must not refresh the lazy binding")
	mark, pending := d.resourceChangeMarks.Load(key)
	require.True(t, pending, "The change arriving during the reconciliation must stay pending")
	assert.Equal(t, false, mark)
	assert.Equal(t, 2, d.Processor.(*mockAsyncWorker).enqueueCount, "The change arriving during the reconciliation must be enqueued")

	require.NoError(t, d.Reconcile(key))
	require.NoError(t, d.Client.Get(t.Context(), bindingKey, binding))
	assert.Equal(t, "3", binding.Spec.Resource.ResourceVersion)
	assert.Equal(t, []string{"new-cluster"}, binding.Spec.Placement.ClusterAffinity.ClusterNames)
	_, pending = d.takeResourceChange(key)
	assert.False(t, pending)
}

func TestResourceChangeOnAddAndDelete(t *testing.T) {
	for _, event := range []string{"add", "delete"} {
		for _, policyFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s,policyFirst=%t", event, policyFirst), func(t *testing.T) {
				d, obj, key, bindingKey := newResourceChangeTestDetector(t)
				if policyFirst {
					d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
				}
				if event == "add" {
					d.OnAdd(obj, true)
				} else {
					d.OnDelete(obj)
				}
				if !policyFirst {
					d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
				}
				require.NoError(t, d.Reconcile(key))
				assertResourceChangeBinding(t, d, bindingKey, true)
			})
		}
	}
}

func TestResourceChangeDelayedRetry(t *testing.T) {
	d, _, key, bindingKey := newResourceChangeTestDetector(t)
	results := make(chan error, 3)
	first := true
	d.Processor = util.NewAsyncWorker(util.Options{
		Name:               "resource-change-retry-test",
		KeyFunc:            ClusterWideKeyFunc,
		RateLimiterOptions: ratelimiterflag.Options{RateLimiterBaseDelay: 500 * time.Millisecond},
		ReconcileFunc: func(k util.QueueKey) error {
			if first {
				first = false
				d.RESTMapper = &failingRESTMapper{}
			} else {
				d.RESTMapper = &mockRESTMapper{}
			}
			err := d.Reconcile(k)
			results <- err
			return err
		},
	})
	d.Processor.Run(t.Context(), 1)
	d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
	require.Error(t, waitResourceChangeResult(t, results))

	// A new event consumes the restored mark before the ordinary queue's delayed retry arrives.
	d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
	require.NoError(t, waitResourceChangeResult(t, results))
	require.NoError(t, waitResourceChangeResult(t, results))
	assertResourceChangeBinding(t, d, bindingKey, false)
}

func TestResourceChangeSerialReconcile(t *testing.T) {
	for _, usePriorityQueue := range []bool{false, true} {
		t.Run(fmt.Sprintf("priorityQueue=%t", usePriorityQueue), func(t *testing.T) {
			d, obj, key, bindingKey := newResourceChangeTestDetector(t)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			started, release := make(chan struct{}), make(chan struct{})
			var first atomic.Bool
			d.RESTMapper = &resourceChangeRESTMapper{
				RESTMapper: &mockRESTMapper{},
				beforeMapping: func() error {
					if first.CompareAndSwap(false, true) {
						close(started)
						select {
						case <-release:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					return nil
				},
			}
			other := obj.DeepCopy()
			other.SetName("other")
			other.SetLabels(map[string]string{util.ResourceTemplateClaimedByLabel: "other-controller"})
			require.NoError(t, d.InformerManager.(*resourceChangeInformerManager).indexer.Add(other))
			otherKey, err := keys.ClusterWideKeyFunc(other)
			require.NoError(t, err)
			results, otherResults := make(chan error, 3), make(chan error, 1)
			var active atomic.Int32
			d.Processor = util.NewAsyncWorker(util.Options{
				Name:             "resource-change-serial-test",
				KeyFunc:          ClusterWideKeyFunc,
				UsePriorityQueue: usePriorityQueue,
				ReconcileFunc: func(k util.QueueKey) error {
					if k == otherKey {
						err := d.Reconcile(k)
						otherResults <- err
						return err
					}
					assert.Equal(t, int32(1), active.Add(1), "The same resource must not reconcile concurrently")
					err := d.Reconcile(k)
					active.Add(-1)
					results <- err
					return err
				},
			})
			d.Processor.Run(ctx, 2)
			d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("Timed out waiting for the first reconciliation")
			}

			// Both origins arrive while this resource is processing. Another resource can still proceed.
			old := obj.DeepCopy()
			require.NoError(t, unstructured.SetNestedField(old.Object, int64(1), "spec", "replicas"))
			d.OnUpdate(old, obj)
			d.enqueueResourceTemplateForPolicyChange(key, policyv1alpha1.LazyActivation)
			d.OnAdd(other, false)
			require.NoError(t, waitResourceChangeResult(t, otherResults))
			close(release)
			require.NoError(t, waitResourceChangeResult(t, results))
			require.NoError(t, waitResourceChangeResult(t, results))
			assertResourceChangeBinding(t, d, bindingKey, true)
		})
	}
}

func newResourceChangeTestDetector(t *testing.T) (*ResourceDetector, *unstructured.Unstructured, keys.ClusterWideKey, client.ObjectKey) {
	t.Helper()
	policy := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default", Name: "lazy-policy",
			Labels: map[string]string{policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-id"},
		},
		Spec: policyv1alpha1.PropagationSpec{
			ActivationPreference: policyv1alpha1.LazyActivation,
			ResourceSelectors:    []policyv1alpha1.ResourceSelector{{APIVersion: "apps/v1", Kind: "Deployment", Name: "demo"}},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{ClusterNames: []string{"new-cluster"}},
			},
		},
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{"namespace": "default", "name": "demo", "uid": "uid", "resourceVersion": "2"},
		"spec":     map[string]any{"replicas": int64(2)},
	}}
	AddPPClaimMetadata(obj, "policy-id", policy.ObjectMeta)
	key, err := keys.ClusterWideKeyFunc(obj)
	require.NoError(t, err)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(obj))
	d := &ResourceDetector{
		RESTMapper:          &mockRESTMapper{},
		InformerManager:     &resourceChangeInformerManager{indexer: indexer},
		Processor:           &mockAsyncWorker{},
		ResourceInterpreter: &mockResourceInterpreter{},
		EventRecorder:       record.NewFakeRecorder(20),
	}
	old := obj.DeepCopy()
	old.SetResourceVersion("1")
	oldPolicy := policy.DeepCopy()
	oldPolicy.Spec.Placement.ClusterAffinity.ClusterNames = []string{"old-cluster"}
	binding, err := d.BuildResourceBinding(old, &oldPolicy.Spec, "policy-id", policy.ObjectMeta, AddPPClaimMetadata)
	require.NoError(t, err)
	d.Client = fake.NewClientBuilder().WithScheme(setupTestScheme()).WithObjects(policy, binding).Build()
	return d, obj, key, client.ObjectKeyFromObject(binding)
}

func assertResourceChangeBinding(t *testing.T, d *ResourceDetector, key client.ObjectKey, updated bool) {
	t.Helper()
	binding := &workv1alpha2.ResourceBinding{}
	require.NoError(t, d.Client.Get(t.Context(), key, binding))
	version, cluster := "1", "old-cluster"
	if updated {
		version, cluster = "2", "new-cluster"
	}
	assert.Equal(t, version, binding.Spec.Resource.ResourceVersion)
	assert.Equal(t, []string{cluster}, binding.Spec.Placement.ClusterAffinity.ClusterNames)
}

func waitResourceChangeResult(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for reconciliation")
		return nil
	}
}

type resourceChangeInformerManager struct {
	genericmanager.SingleClusterInformerManager
	indexer cache.Indexer
}

func (m *resourceChangeInformerManager) Lister(gvr schema.GroupVersionResource) cache.GenericLister {
	return cache.NewGenericLister(m.indexer, gvr.GroupResource())
}

type resourceChangeRESTMapper struct {
	meta.RESTMapper
	beforeMapping func() error
}

func (m *resourceChangeRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if err := m.beforeMapping(); err != nil {
		return nil, err
	}
	return m.RESTMapper.RESTMapping(gk, versions...)
}
