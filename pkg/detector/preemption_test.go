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

package detector

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

type MockAsyncWorker struct {
	queue []interface{}
}

// Note: This is a dummy implementation of Add for testing purposes.
func (m *MockAsyncWorker) Add(item interface{}) {
	// No actual work is done in the mock; we just simulate running
	m.queue = append(m.queue, item)
}

// Note: This is a dummy implementation of AddAfter for testing purposes.
func (m *MockAsyncWorker) AddAfter(item interface{}, duration time.Duration) {
	// No actual work is done in the mock; we just simulate running
	fmt.Printf("%v", duration)
	m.queue = append(m.queue, item)
}

// Note: This is a dummy implementation of Enqueue for testing purposes.
func (m *MockAsyncWorker) Enqueue(obj interface{}) {
	// Assuming KeyFunc is used to generate a key; for simplicity, we use obj directly
	m.queue = append(m.queue, obj)
}

// Note: This is a dummy implementation of Run for testing purposes.
func (m *MockAsyncWorker) Run(ctx context.Context, workerNumber int) {
	// No actual work is done in the mock; we just simulate running
	fmt.Printf("%v", workerNumber)
	fmt.Printf("%v", <-ctx.Done())
}

// GetQueue returns the current state of the queue
func (m *MockAsyncWorker) GetQueue() []interface{} {
	return m.queue
}

func TestHandleDeprioritizedPropagationPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(v1alpha2.Install(scheme))
	utilruntime.Must(policyv1alpha1.Install(scheme))

	tests := []struct {
		name          string
		newPolicy     *policyv1alpha1.PropagationPolicy
		oldPolicy     *policyv1alpha1.PropagationPolicy
		objects       []runtime.Object
		setupClient   func() *fake.ClientBuilder
		wantQueueSize int
	}{
		{
			name: "preempt deprioritized propagation policy of len 1",
			newPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](4),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: []runtime.Object{
				&policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "test",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				},
			},
			setupClient: func() *fake.ClientBuilder {
				obj := &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "test",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj)
			},
			wantQueueSize: 1,
		},
		{
			name: "preempt deprioritized propagation policy of len 2",
			newPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](5),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: []runtime.Object{
				&policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "test",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				},
				&policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-2",
						Namespace: "test",
						Labels: map[string]string{
							policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-2",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](4),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test-2",
								Name:       "default-2",
							},
						},
					},
				},
			},
			setupClient: func() *fake.ClientBuilder {
				obj := []client.Object{
					&policyv1alpha1.PropagationPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: "test",
							Labels: map[string]string{
								policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-1",
							},
						},
						Spec: policyv1alpha1.PropagationSpec{
							Priority:   ptr.To[int32](3),
							Preemption: policyv1alpha1.PreemptAlways,
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "default",
								},
							},
						},
					},
					&policyv1alpha1.PropagationPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo-2",
							Namespace: "test",
							Labels: map[string]string{
								policyv1alpha1.PropagationPolicyPermanentIDLabel: "policy-2",
							},
						},
						Spec: policyv1alpha1.PropagationSpec{
							Priority:   ptr.To[int32](4),
							Preemption: policyv1alpha1.PreemptAlways,
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test-2",
									Name:       "default-2",
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj...)
			},
			wantQueueSize: 2,
		},
		{
			name: "no policy to preempt",
			newPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.PropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](4),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: nil,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			wantQueueSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, tt.objects...)
			genMgr := genericmanager.NewSingleClusterInformerManager(ctx, fakeDynamicClient, 0)
			resourceDetector := &ResourceDetector{
				Client:          fakeClient,
				DynamicClient:   fakeDynamicClient,
				InformerManager: genMgr,
			}
			mockWorker := &MockAsyncWorker{}
			resourceDetector.policyReconcileWorker = mockWorker
			resourceDetector.InformerManager.Start()
			resourceDetector.InformerManager.WaitForCacheSync()

			resourceDetector.HandleDeprioritizedPropagationPolicy(*tt.oldPolicy, *tt.newPolicy)

			gotQueueSize := len(mockWorker.GetQueue())
			if gotQueueSize != tt.wantQueueSize {
				t.Errorf("HandleDeprioritizedPropagationPolicy() want queue size %v, got %v", tt.wantQueueSize, gotQueueSize)
			}
		})
	}
}

func TestHandleDeprioritizedClusterPropagationPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(v1alpha2.Install(scheme))
	utilruntime.Must(policyv1alpha1.Install(scheme))

	tests := []struct {
		name          string
		newPolicy     *policyv1alpha1.ClusterPropagationPolicy
		oldPolicy     *policyv1alpha1.ClusterPropagationPolicy
		objects       []runtime.Object
		setupClient   func() *fake.ClientBuilder
		wantQueueSize int
	}{
		{
			name: "preempt deprioritized cluster propagation policy of len 1",
			newPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](4),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: []runtime.Object{
				&policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				},
			},
			setupClient: func() *fake.ClientBuilder {
				obj := &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj)
			},
			wantQueueSize: 1,
		},
		{
			name: "preempt deprioritized cluster propagation policy of len 2",
			newPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](5),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: []runtime.Object{
				&policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-1",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](3),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test",
								Name:       "default",
							},
						},
					},
				},
				&policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-2",
						Namespace: "bar-2",
						Labels: map[string]string{
							policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-2",
						},
					},
					Spec: policyv1alpha1.PropagationSpec{
						Priority:   ptr.To[int32](4),
						Preemption: policyv1alpha1.PreemptAlways,
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Namespace:  "test-2",
								Name:       "default-2",
							},
						},
					},
				},
			},
			setupClient: func() *fake.ClientBuilder {
				obj := []client.Object{
					&policyv1alpha1.ClusterPropagationPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: "bar",
							Labels: map[string]string{
								policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-1",
							},
						},
						Spec: policyv1alpha1.PropagationSpec{
							Priority:   ptr.To[int32](3),
							Preemption: policyv1alpha1.PreemptAlways,
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test",
									Name:       "default",
								},
							},
						},
					},
					&policyv1alpha1.ClusterPropagationPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo-2",
							Namespace: "bar-2",
							Labels: map[string]string{
								policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel: "policy-2",
							},
						},
						Spec: policyv1alpha1.PropagationSpec{
							Priority:   ptr.To[int32](4),
							Preemption: policyv1alpha1.PreemptAlways,
							ResourceSelectors: []policyv1alpha1.ResourceSelector{
								{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Namespace:  "test-2",
									Name:       "default-2",
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj...)
			},
			wantQueueSize: 2,
		},
		{
			name: "no cluster policy to preempt",
			newPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](2),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			oldPolicy: &policyv1alpha1.ClusterPropagationPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "test"},
				Spec: policyv1alpha1.PropagationSpec{
					Priority: ptr.To[int32](4),
					ResourceSelectors: []policyv1alpha1.ResourceSelector{
						{
							APIVersion:    "apps/v1",
							Kind:          "Deployment",
							Namespace:     "test",
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
						},
					},
				},
			},
			objects: nil,
			setupClient: func() *fake.ClientBuilder {
				return fake.NewClientBuilder().WithScheme(scheme)
			},
			wantQueueSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient().Build()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, tt.objects...)
			genMgr := genericmanager.NewSingleClusterInformerManager(ctx, fakeDynamicClient, 0)
			resourceDetector := &ResourceDetector{
				Client:          fakeClient,
				DynamicClient:   fakeDynamicClient,
				InformerManager: genMgr,
			}
			mockWorker := &MockAsyncWorker{}
			resourceDetector.clusterPolicyReconcileWorker = mockWorker
			resourceDetector.InformerManager.Start()
			resourceDetector.InformerManager.WaitForCacheSync()

			resourceDetector.HandleDeprioritizedClusterPropagationPolicy(*tt.oldPolicy, *tt.newPolicy)

			gotQueueSize := len(mockWorker.GetQueue())
			if gotQueueSize != tt.wantQueueSize {
				t.Errorf("HandleDeprioritizedClusterPropagationPolicy() want queue size %v, got %v", tt.wantQueueSize, gotQueueSize)
			}
		})
	}
}
