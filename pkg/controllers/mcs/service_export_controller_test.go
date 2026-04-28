/*
Copyright 2025 The Karmada Authors.

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

package mcs

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	testinggenericmanager "github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager/testing"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha1.Install(scheme)
	_ = clusterv1alpha1.Install(scheme)
	return scheme
}

// mockErrorClient wraps a client and allows configuring errors for specific operations
type mockErrorClient struct {
	client.Client
	getError    error
	listError   error
	deleteError error
	updateError error
}

func (e *mockErrorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if e.getError != nil {
		return e.getError
	}
	return e.Client.Get(ctx, key, obj, opts...)
}

func (e *mockErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if e.listError != nil {
		return e.listError
	}
	return e.Client.List(ctx, list, opts...)
}

func (e *mockErrorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if e.deleteError != nil {
		return e.deleteError
	}
	return e.Client.Delete(ctx, obj, opts...)
}

func (e *mockErrorClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if e.updateError != nil {
		return e.updateError
	}
	return e.Client.Update(ctx, obj, opts...)
}

// mockLister simulates cache.GenericLister
type mockLister struct {
	getFunc  func(name string) (runtime.Object, error)
	listFunc func(selector labels.Selector) ([]runtime.Object, error)
}

func (m *mockLister) List(selector labels.Selector) ([]runtime.Object, error) {
	if m.listFunc != nil {
		return m.listFunc(selector)
	}
	return nil, nil
}

func (m *mockLister) Get(name string) (runtime.Object, error) {
	if m.getFunc != nil {
		return m.getFunc(name)
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
}

func (m *mockLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return &mockNamespaceLister{lister: m}
}

// mockNamespaceLister simulates cache.GenericNamespaceLister
type mockNamespaceLister struct {
	lister *mockLister
}

func (m *mockNamespaceLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	return m.lister.List(selector)
}

func (m *mockNamespaceLister) Get(name string) (runtime.Object, error) {
	return m.lister.Get(name)
}

func TestCleanEndpointSliceWork(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name    string
		prepare func() (client.Client, *workv1alpha1.Work)
		verify  func(t *testing.T, c client.Client, originalWork *workv1alpha1.Work, err error)
	}{
		{
			name: "should delete work when only managed by ServiceExport",
			prepare: func() (client.Client, *workv1alpha1.Work) {
				work := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work",
						Namespace: "test-ns",
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(), work
			},
			verify: func(t *testing.T, c client.Client, originalWork *workv1alpha1.Work, err error) {
				assert.NoError(t, err)
				w := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: originalWork.Name, Namespace: originalWork.Namespace}, w)
				assert.True(t, apierrors.IsNotFound(getErr), "should not find work after deletion")
			},
		},
		{
			name: "should only clean ServiceExport label when managed by multiple controllers",
			prepare: func() (client.Client, *workv1alpha1.Work) {
				work := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work",
						Namespace: "test-ns",
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: "OtherController.ServiceExport",
							util.ServiceNameLabel:                "svc",
							util.ServiceNamespaceLabel:           "ns",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(), work
			},
			verify: func(t *testing.T, c client.Client, originalWork *workv1alpha1.Work, err error) {
				assert.NoError(t, err)
				w := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: originalWork.Name, Namespace: originalWork.Namespace}, w)
				assert.NoError(t, getErr, "work should still exist")
				assert.NotContains(t, w.Labels[util.EndpointSliceWorkManagedByLabel], util.ServiceExportKind)
				assert.NotContains(t, w.Labels, util.ServiceNameLabel)
				assert.NotContains(t, w.Labels, util.ServiceNamespaceLabel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, work := tt.prepare()
			err := cleanEndpointSliceWork(ctx, c, work)
			tt.verify(t, c, work, err)
		})
	}
}

func TestCleanupWorkWithEndpointSliceDelete(t *testing.T) {
	ctx := context.TODO()

	cluster := "member1"
	namespace := "ns1"
	name := "eps1"

	key := keys.FederatedKey{
		Cluster: cluster,
		ClusterWideKey: keys.ClusterWideKey{
			Group:     "discovery.k8s.io",
			Version:   "v1",
			Kind:      util.EndpointSliceKind,
			Namespace: namespace,
			Name:      name,
		},
	}

	execNS := names.GenerateExecutionSpaceName(cluster)
	workName := names.GenerateWorkName(key.Kind, key.Name, key.Namespace)

	tests := []struct {
		name    string
		prepare func() client.Client
		verify  func(t *testing.T, c client.Client, err error)
	}{
		{
			name: "work not found should return nil",
			prepare: func() client.Client {
				return fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "get returns other error should return error",
			prepare: func() client.Client {
				return &mockErrorClient{
					Client:   fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					getError: fmt.Errorf("client error"),
				}
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "work exists and only managed by ServiceExport -> deleted",
			prepare: func() client.Client {
				w := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workName,
						Namespace: execNS,
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(w).Build()
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				w := &workv1alpha1.Work{}
				e := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, w)
				assert.True(t, apierrors.IsNotFound(e), "work should be deleted")
			},
		},
		{
			name: "work exists and managed by multiple controllers -> updated",
			prepare: func() client.Client {
				w := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workName,
						Namespace: execNS,
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: "OtherController.ServiceExport",
							util.ServiceNameLabel:                "svc",
							util.ServiceNamespaceLabel:           "ns",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(w).Build()
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				w := &workv1alpha1.Work{}
				e := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, w)
				assert.NoError(t, e)
				assert.NotContains(t, w.Labels[util.EndpointSliceWorkManagedByLabel], util.ServiceExportKind)
				assert.NotContains(t, w.Labels, util.ServiceNameLabel)
				assert.NotContains(t, w.Labels, util.ServiceNamespaceLabel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()
			err := cleanupWorkWithEndpointSliceDelete(ctx, c, key)
			tt.verify(t, c, err)
		})
	}
}

func TestCleanupWorkWithServiceExportDelete(t *testing.T) {
	ctx := context.TODO()

	cluster := "member1"
	namespace := "ns1"
	name := "svc1"

	key := keys.FederatedKey{
		Cluster: cluster,
		ClusterWideKey: keys.ClusterWideKey{
			Group:     "multicluster.x-k8s.io",
			Version:   "v1alpha1",
			Kind:      util.ServiceExportKind,
			Namespace: namespace,
			Name:      name,
		},
	}

	execNS := names.GenerateExecutionSpaceName(cluster)

	tests := []struct {
		name    string
		prepare func() client.Client
		verify  func(t *testing.T, c client.Client, err error)
	}{
		{
			name: "list returns error should return error",
			prepare: func() client.Client {
				return &mockErrorClient{
					Client:    fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					listError: fmt.Errorf("list failed"),
				}
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "list failed")
			},
		},
		{
			name: "no works found should return nil",
			prepare: func() client.Client {
				return fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "single work managed by ServiceExport should be deleted",
			prepare: func() client.Client {
				work := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-1",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                name,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build()
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				work := &workv1alpha1.Work{}
				e := c.Get(ctx, types.NamespacedName{Name: "work-1", Namespace: execNS}, work)
				assert.True(t, apierrors.IsNotFound(e), "work should be deleted")
			},
		},
		{
			name: "multiple works with mixed management should be handled correctly",
			prepare: func() client.Client {
				work1 := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-1",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                name,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				work2 := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-2",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                name,
							util.EndpointSliceWorkManagedByLabel: "OtherController.ServiceExport",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work1, work2).Build()
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// work1 should be deleted
				work1 := &workv1alpha1.Work{}
				e1 := c.Get(ctx, types.NamespacedName{Name: "work-1", Namespace: execNS}, work1)
				assert.True(t, apierrors.IsNotFound(e1), "work-1 should be deleted")

				// work2 should be updated (labels removed)
				work2 := &workv1alpha1.Work{}
				e2 := c.Get(ctx, types.NamespacedName{Name: "work-2", Namespace: execNS}, work2)
				assert.NoError(t, e2)
				assert.NotContains(t, work2.Labels[util.EndpointSliceWorkManagedByLabel], util.ServiceExportKind)
				assert.NotContains(t, work2.Labels, util.ServiceNameLabel)
				assert.NotContains(t, work2.Labels, util.ServiceNamespaceLabel)
			},
		},
		{
			name: "works not matching service labels should be ignored",
			prepare: func() client.Client {
				work1 := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-1",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                name,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				work2 := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-2",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           "other-ns",
							util.ServiceNameLabel:                "other-svc",
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work1, work2).Build()
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// work1 should be deleted (matches our service)
				work1 := &workv1alpha1.Work{}
				e1 := c.Get(ctx, types.NamespacedName{Name: "work-1", Namespace: execNS}, work1)
				assert.True(t, apierrors.IsNotFound(e1), "work-1 should be deleted")

				// work2 should still exist (doesn't match our service)
				work2 := &workv1alpha1.Work{}
				e2 := c.Get(ctx, types.NamespacedName{Name: "work-2", Namespace: execNS}, work2)
				assert.NoError(t, e2, "work-2 should still exist")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()
			err := cleanupWorkWithServiceExportDelete(ctx, c, key)
			tt.verify(t, c, err)
		})
	}
}

func TestGetEndpointSliceWorkMeta(t *testing.T) {
	ctx := context.TODO()

	namespace := "test-ns"
	workName := "test-work"
	serviceName := "test-service"
	epsNamespace := "eps-ns"

	constructEndpointSlice := func() *unstructured.Unstructured {
		eps := &unstructured.Unstructured{}
		eps.SetAPIVersion("discovery.k8s.io/v1")
		eps.SetKind("EndpointSlice")
		eps.SetNamespace(epsNamespace)
		eps.SetName("test-eps")
		eps.SetLabels(map[string]string{
			discoveryv1.LabelServiceName: serviceName,
		})
		return eps
	}

	tests := []struct {
		name    string
		prepare func() client.Client
		verify  func(t *testing.T, meta metav1.ObjectMeta, err error)
	}{
		{
			name: "work not found should create new meta with basic labels",
			prepare: func() client.Client {
				return fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
			},
			verify: func(t *testing.T, meta metav1.ObjectMeta, err error) {
				assert.NoError(t, err)
				assert.Equal(t, workName, meta.Name)
				assert.Equal(t, namespace, meta.Namespace)
				assert.Contains(t, meta.Finalizers, util.EndpointSliceControllerFinalizer)
				assert.Equal(t, epsNamespace, meta.Labels[util.ServiceNamespaceLabel])
				assert.Equal(t, serviceName, meta.Labels[util.ServiceNameLabel])
				assert.Equal(t, util.ServiceExportKind, meta.Labels[util.EndpointSliceWorkManagedByLabel])
			},
		},
		{
			name: "get work returns error should return error",
			prepare: func() client.Client {
				return &mockErrorClient{
					Client:   fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					getError: fmt.Errorf("get failed"),
				}
			},
			verify: func(t *testing.T, _ metav1.ObjectMeta, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "get failed")
			},
		},
		{
			name: "work exists without labels should merge with existing finalizers",
			prepare: func() client.Client {
				existingWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:       workName,
						Namespace:  namespace,
						Finalizers: []string{"existing-finalizer"},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(existingWork).Build()
			},
			verify: func(t *testing.T, meta metav1.ObjectMeta, err error) {
				assert.NoError(t, err)
				assert.Equal(t, workName, meta.Name)
				assert.Equal(t, namespace, meta.Namespace)
				assert.Contains(t, meta.Finalizers, util.EndpointSliceControllerFinalizer)
				assert.Contains(t, meta.Finalizers, "existing-finalizer")
				assert.Equal(t, util.ServiceExportKind, meta.Labels[util.EndpointSliceWorkManagedByLabel])
			},
		},
		{
			name: "work exists with existing managed-by label should merge controllers",
			prepare: func() client.Client {
				existingWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workName,
						Namespace: namespace,
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: "OtherController",
							"existing-label":                     "existing-value",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(existingWork).Build()
			},
			verify: func(t *testing.T, meta metav1.ObjectMeta, err error) {
				assert.NoError(t, err)
				assert.Equal(t, workName, meta.Name)
				assert.Equal(t, namespace, meta.Namespace)

				// Should merge existing labels
				assert.Equal(t, "existing-value", meta.Labels["existing-label"])
				assert.Equal(t, epsNamespace, meta.Labels[util.ServiceNamespaceLabel])
				assert.Equal(t, serviceName, meta.Labels[util.ServiceNameLabel])

				// Should merge managed-by controllers
				managedBy := meta.Labels[util.EndpointSliceWorkManagedByLabel]
				assert.Contains(t, managedBy, "OtherController")
				assert.Contains(t, managedBy, util.ServiceExportKind)
			},
		},
		{
			name: "work exists with ServiceExport already in managed-by should not duplicate",
			prepare: func() client.Client {
				existingWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workName,
						Namespace: namespace,
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: "ServiceExport.OtherController",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(existingWork).Build()
			},
			verify: func(t *testing.T, meta metav1.ObjectMeta, err error) {
				assert.NoError(t, err)
				managedBy := meta.Labels[util.EndpointSliceWorkManagedByLabel]
				assert.Contains(t, managedBy, "OtherController")
				// Should not have duplicate ServiceExport
				assert.Equal(t, 1, strings.Count(managedBy, "ServiceExport"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()
			eps := constructEndpointSlice()
			workMeta, err := getEndpointSliceWorkMeta(ctx, c, namespace, workName, eps)
			tt.verify(t, workMeta, err)
		})
	}
}

func TestReportEndpointSliceWithEndpointSliceCreateOrUpdate(t *testing.T) {
	ctx := context.TODO()
	clusterName := "member1"
	serviceName := "test-service"
	epsNamespace := "test-ns"

	constructEndpointSlice := func() *unstructured.Unstructured {
		eps := &unstructured.Unstructured{}
		eps.SetAPIVersion("discovery.k8s.io/v1")
		eps.SetKind("EndpointSlice")
		eps.SetNamespace(epsNamespace)
		eps.SetName("test-eps")
		eps.SetLabels(map[string]string{
			discoveryv1.LabelServiceName: serviceName,
		})
		return eps
	}

	tests := []struct {
		name    string
		prepare func() (*ServiceExportController, client.Client)
		verify  func(t *testing.T, c client.Client, err error)
	}{
		{
			name: "single cluster manager not found should return nil",
			prepare: func() (*ServiceExportController, client.Client) {
				client := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
				controller := &ServiceExportController{
					Client:          client,
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(nil),
				}
				return controller, client
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "informer not synced should return error",
			prepare: func() (*ServiceExportController, client.Client) {
				client := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
				controller := &ServiceExportController{
					Client: client,
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(false, false, func(_ schema.GroupVersionResource) cache.GenericLister {
							return &mockLister{
								getFunc: func(name string) (runtime.Object, error) {
									return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
								},
							}
						}),
					}),
				}
				return controller, client
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "has not been synced")
			},
		},
		{
			name: "ServiceExport not found should return nil",
			prepare: func() (*ServiceExportController, client.Client) {
				client := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
				controller := &ServiceExportController{
					Client: client,
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(_ schema.GroupVersionResource) cache.GenericLister {
							return &mockLister{
								getFunc: func(name string) (runtime.Object, error) {
									return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
								},
							}
						}),
					}),
				}
				return controller, client
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "lister returns other error should return error",
			prepare: func() (*ServiceExportController, client.Client) {
				client := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
				controller := &ServiceExportController{
					Client: client,
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(_ schema.GroupVersionResource) cache.GenericLister {
							return &mockLister{
								getFunc: func(_ string) (runtime.Object, error) {
									return nil, fmt.Errorf("lister error")
								},
							}
						}),
					}),
				}
				return controller, client
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "lister error")
			},
		},
		{
			name: "ServiceExport found should call reportEndpointSlice",
			prepare: func() (*ServiceExportController, client.Client) {
				client := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()
				controller := &ServiceExportController{
					Client: client,
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(_ schema.GroupVersionResource) cache.GenericLister {
							return &mockLister{
								getFunc: func(name string) (runtime.Object, error) {
									// Return a ServiceExport object
									obj := &unstructured.Unstructured{}
									obj.SetAPIVersion("multicluster.x-k8s.io/v1alpha1")
									obj.SetKind("ServiceExport")
									obj.SetName(name)
									obj.SetNamespace(epsNamespace)
									return obj, nil
								},
							}
						}),
					}),
				}
				return controller, client
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify that a Work resource was created
				execNS := names.GenerateExecutionSpaceName(clusterName)
				workName := names.GenerateWorkName(util.EndpointSliceKind, "test-eps", epsNamespace)
				work := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, work)
				assert.NoError(t, getErr, "Work should be created")
				assert.NotNil(t, work)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, client := tt.prepare()
			eps := constructEndpointSlice()
			err := controller.reportEndpointSliceWithEndpointSliceCreateOrUpdate(ctx, clusterName, eps)
			tt.verify(t, client, err)
		})
	}
}

func TestReportEndpointSliceWithServiceExportCreate(t *testing.T) {
	ctx := context.TODO()
	clusterName := "member1"
	namespace := "test-ns"
	serviceName := "test-service"

	serviceExportKey := keys.FederatedKey{
		Cluster: clusterName,
		ClusterWideKey: keys.ClusterWideKey{
			Group:     "multicluster.x-k8s.io",
			Version:   "v1alpha1",
			Kind:      util.ServiceExportKind,
			Namespace: namespace,
			Name:      serviceName,
		},
	}

	execNS := names.GenerateExecutionSpaceName(clusterName)

	// Helper function to create EndpointSlice objects
	createEndpointSlice := func(name, ns string) *unstructured.Unstructured {
		eps := &unstructured.Unstructured{}
		eps.SetAPIVersion("discovery.k8s.io/v1")
		eps.SetKind("EndpointSlice")
		eps.SetName(name)
		eps.SetNamespace(ns)
		eps.SetLabels(map[string]string{
			discoveryv1.LabelServiceName: serviceName,
		})
		return eps
	}

	// Helper function that creates a mock lister with specified EndpointSlice objects
	createMockListerWithEndpointSlices := func(endpointSlices []runtime.Object) func(gvr schema.GroupVersionResource) cache.GenericLister {
		return func(gvr schema.GroupVersionResource) cache.GenericLister {
			if gvr == endpointSliceGVR {
				return &mockLister{
					listFunc: func(_ labels.Selector) ([]runtime.Object, error) {
						return endpointSlices, nil
					},
				}
			}
			return &mockLister{}
		}
	}

	tests := []struct {
		name    string
		prepare func() *ServiceExportController
		verify  func(t *testing.T, c client.Client, err error)
	}{
		{
			name: "single cluster manager not found should return nil",
			prepare: func() *ServiceExportController {
				controller := &ServiceExportController{
					Client:          fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(nil),
				}
				return controller
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "informer not synced should return error",
			prepare: func() *ServiceExportController {
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(false, false, nil),
					}),
				}
				return controller
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "has not been synced")
			},
		},
		{
			name: "lister returns error should return error",
			prepare: func() *ServiceExportController {
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(gvr schema.GroupVersionResource) cache.GenericLister {
							if gvr == endpointSliceGVR {
								return &mockLister{
									listFunc: func(_ labels.Selector) ([]runtime.Object, error) {
										return nil, fmt.Errorf("lister error")
									},
								}
							}
							return &mockLister{}
						}),
					}),
				}
				return controller
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "lister error")
			},
		},
		{
			name: "removeOrphanWork returns error should return error",
			prepare: func() *ServiceExportController {
				eps1 := createEndpointSlice("eps-1", namespace)
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, createMockListerWithEndpointSlices([]runtime.Object{eps1})),
					}),
					// Provider that returns error
					epsWorkListFunc: func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
						return nil, fmt.Errorf("provider error")
					},
				}
				return controller
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "provider error")
			},
		},
		{
			name: "successful execution should report EndpointSlices and remove orphan works",
			prepare: func() *ServiceExportController {
				eps1 := createEndpointSlice("eps-1", namespace)
				eps2 := createEndpointSlice("eps-2", namespace)

				// Create some existing works, including an orphan
				orphanWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName(util.EndpointSliceKind, "eps-orphan", namespace),
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                serviceName,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte(`{
											"apiVersion": "discovery.k8s.io/v1",
											"kind": "EndpointSlice",
											"metadata": {
												"name": "eps-orphan",
												"namespace": "` + namespace + `"
											}
										}`),
									},
								},
							},
						},
					},
				}

				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(orphanWork).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, createMockListerWithEndpointSlices([]runtime.Object{eps1, eps2})),
					}),
					// Provider that returns the orphan work
					epsWorkListFunc: func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
						return &workv1alpha1.WorkList{
							Items: []workv1alpha1.Work{*orphanWork},
						}, nil
					},
				}
				return controller
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify that Work resources are created for current EndpointSlices
				eps1WorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-1", namespace)
				eps2WorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-2", namespace)

				work1 := &workv1alpha1.Work{}
				err1 := c.Get(ctx, types.NamespacedName{Name: eps1WorkName, Namespace: execNS}, work1)
				assert.NoError(t, err1, "Work for eps-1 should be created")

				work2 := &workv1alpha1.Work{}
				err2 := c.Get(ctx, types.NamespacedName{Name: eps2WorkName, Namespace: execNS}, work2)
				assert.NoError(t, err2, "Work for eps-2 should be created")

				// Verify orphan work is removed
				orphanWorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-orphan", namespace)
				orphanWork := &workv1alpha1.Work{}
				orphanErr := c.Get(ctx, types.NamespacedName{Name: orphanWorkName, Namespace: execNS}, orphanWork)
				assert.True(t, apierrors.IsNotFound(orphanErr), "Orphan work should be deleted")
			},
		},
		{
			name: "no EndpointSlices should only perform cleanup",
			prepare: func() *ServiceExportController {
				// Create an orphan work
				orphanWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName(util.EndpointSliceKind, "eps-orphan", namespace),
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                serviceName,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte(`{
											"apiVersion": "discovery.k8s.io/v1",
											"kind": "EndpointSlice",
											"metadata": {
												"name": "eps-orphan",
												"namespace": "` + namespace + `"
											}
										}`),
									},
								},
							},
						},
					},
				}

				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(orphanWork).Build(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, createMockListerWithEndpointSlices([]runtime.Object{})),
					}),
					// Provider that returns the orphan work
					epsWorkListFunc: func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
						return &workv1alpha1.WorkList{
							Items: []workv1alpha1.Work{*orphanWork},
						}, nil
					},
				}
				return controller
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify orphan work is removed
				orphanWorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-orphan", namespace)
				orphanWork := &workv1alpha1.Work{}
				orphanErr := c.Get(ctx, types.NamespacedName{Name: orphanWorkName, Namespace: execNS}, orphanWork)
				assert.True(t, apierrors.IsNotFound(orphanErr), "Orphan work should be deleted")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := tt.prepare()
			err := controller.reportEndpointSliceWithServiceExportCreate(ctx, serviceExportKey)
			tt.verify(t, controller.Client, err)
		})
	}
}

func TestRemoveOrphanWork(t *testing.T) {
	ctx := context.TODO()

	cluster := "member1"
	namespace := "test-ns"
	serviceName := "test-service"

	serviceExportKey := keys.FederatedKey{
		Cluster: cluster,
		ClusterWideKey: keys.ClusterWideKey{
			Group:     "multicluster.x-k8s.io",
			Version:   "v1alpha1",
			Kind:      util.ServiceExportKind,
			Namespace: namespace,
			Name:      serviceName,
		},
	}

	execNS := names.GenerateExecutionSpaceName(cluster)

	// Pre-calculate work names that are used in multiple tests
	expectedWorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-1", namespace)
	orphanWorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-orphan", namespace)

	// Helper function to create EndpointSlice objects
	createEndpointSlice := func(name, namespace string) *unstructured.Unstructured {
		eps := &unstructured.Unstructured{}
		eps.SetAPIVersion("discovery.k8s.io/v1")
		eps.SetKind("EndpointSlice")
		eps.SetName(name)
		eps.SetNamespace(namespace)
		eps.SetLabels(map[string]string{
			discoveryv1.LabelServiceName: serviceName,
		})
		return eps
	}

	// Helper function to create Work objects with suspended dispatching
	createWork := func(name string, managedBy string, withManifests bool) *workv1alpha1.Work {
		suspendDispatching := true
		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: execNS,
				Labels: map[string]string{
					util.ServiceNamespaceLabel:           namespace,
					util.ServiceNameLabel:                serviceName,
					util.EndpointSliceWorkManagedByLabel: managedBy,
				},
			},
			Spec: workv1alpha1.WorkSpec{
				Workload:           workv1alpha1.WorkloadTemplate{},
				SuspendDispatching: &suspendDispatching,
			},
		}

		if withManifests {
			work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
				{
					RawExtension: runtime.RawExtension{
						Raw: []byte(`{
							"apiVersion": "discovery.k8s.io/v1",
							"kind": "EndpointSlice",
							"metadata": {
								"name": "test-eps",
								"namespace": "` + namespace + `"
							}
						}`),
					},
				},
			}
		}
		return work
	}

	// Helper function to create controller and provider
	createControllerWithProvider := func(works []*workv1alpha1.Work, providerWorks []*workv1alpha1.Work) (*ServiceExportController, workListProvider) {
		objs := make([]client.Object, 0, len(works))
		for _, w := range works {
			objs = append(objs, w)
		}
		controller := &ServiceExportController{Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(objs...).Build()}

		provider := func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
			items := make([]workv1alpha1.Work, 0, len(providerWorks))
			for _, w := range providerWorks {
				items = append(items, *w)
			}
			return &workv1alpha1.WorkList{Items: items}, nil
		}
		return controller, provider
	}

	// Helper function to create error provider
	createErrorProvider := func(err error) workListProvider {
		return func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
			return nil, err
		}
	}

	// Helper function to verify work existence
	assertWorkExists := func(t *testing.T, c client.Client, workName string, shouldExist bool, message string) {
		work := &workv1alpha1.Work{}
		err := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, work)
		if shouldExist {
			assert.NoError(t, err, message)
		} else {
			assert.True(t, apierrors.IsNotFound(err), message)
		}
	}

	tests := []struct {
		name                 string
		endpointSliceObjects []runtime.Object
		prepare              func() (*ServiceExportController, workListProvider)
		verify               func(t *testing.T, c client.Client, err error)
	}{
		{
			name:                 "no endpointSlice objects should remove all orphaned works",
			endpointSliceObjects: []runtime.Object{},
			prepare: func() (*ServiceExportController, workListProvider) {
				work1 := createWork("work-1", util.ServiceExportKind, true)
				work2 := createWork("work-2", "OtherController.ServiceExport", true)
				return createControllerWithProvider([]*workv1alpha1.Work{work1, work2}, []*workv1alpha1.Work{work1, work2})
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				assertWorkExists(t, c, "work-1", false, "work-1 should be deleted as orphan")

				work2 := &workv1alpha1.Work{}
				err2 := c.Get(ctx, types.NamespacedName{Name: "work-2", Namespace: execNS}, work2)
				assert.NoError(t, err2, "work-2 should be updated")
				assert.NotContains(t, work2.Labels[util.EndpointSliceWorkManagedByLabel], util.ServiceExportKind)
			},
		},
		{
			name: "list works returns error should return error",
			endpointSliceObjects: []runtime.Object{
				createEndpointSlice("eps-1", namespace),
			},
			prepare: func() (*ServiceExportController, workListProvider) {
				controller := &ServiceExportController{Client: fake.NewClientBuilder().WithScheme(newTestScheme()).Build()}
				return controller, createErrorProvider(fmt.Errorf("list failed"))
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "list failed")
			},
		},
		{
			name: "matching work should be kept, orphan work should be removed",
			endpointSliceObjects: []runtime.Object{
				createEndpointSlice("eps-1", namespace),
			},
			prepare: func() (*ServiceExportController, workListProvider) {
				work1 := createWork(expectedWorkName, util.ServiceExportKind, true)
				work2 := createWork(orphanWorkName, util.ServiceExportKind, true)
				return createControllerWithProvider([]*workv1alpha1.Work{work1, work2}, []*workv1alpha1.Work{work1, work2})
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				assertWorkExists(t, c, expectedWorkName, true, "work1 should still exist")
				assertWorkExists(t, c, orphanWorkName, false, "orphan work should be deleted")
			},
		},
		{
			name: "work not managed by ServiceExport should be skipped",
			endpointSliceObjects: []runtime.Object{
				createEndpointSlice("eps-1", namespace),
			},
			prepare: func() (*ServiceExportController, workListProvider) {
				work := createWork(orphanWorkName, "OtherController", true) // Not managed by ServiceExport
				return createControllerWithProvider([]*workv1alpha1.Work{work}, []*workv1alpha1.Work{work})
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				assertWorkExists(t, c, orphanWorkName, true, "work should still exist (skipped)")
			},
		},
		{
			name: "orphan work managed by multiple controllers should be skipped",
			endpointSliceObjects: []runtime.Object{
				createEndpointSlice("eps-1", namespace),
			},
			prepare: func() (*ServiceExportController, workListProvider) {
				work := createWork(orphanWorkName, "OtherController.ServiceExport", true)
				return createControllerWithProvider([]*workv1alpha1.Work{work}, []*workv1alpha1.Work{work})
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				assertWorkExists(t, c, orphanWorkName, true, "work should still exist (skipped)")
			},
		},
		{
			name: "work with SuspendDispatching=false should be ignored",
			endpointSliceObjects: []runtime.Object{
				createEndpointSlice("eps-1", namespace),
			},
			prepare: func() (*ServiceExportController, workListProvider) {
				// Create work with SuspendDispatching=false (this simulates field selector filtering)
				suspendDispatching := false
				work := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "work-not-suspended",
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                serviceName,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload:           workv1alpha1.WorkloadTemplate{},
						SuspendDispatching: &suspendDispatching,
					},
				}
				work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "discovery.k8s.io/v1",
								"kind": "EndpointSlice",
								"metadata": {
									"name": "test-eps",
									"namespace": "` + namespace + `"
								}
							}`),
						},
					},
				}

				// work2 is not included in provider result (simulates field selector)
				return createControllerWithProvider([]*workv1alpha1.Work{work}, []*workv1alpha1.Work{})
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)
				assertWorkExists(t, c, "work-not-suspended", true, "work should still exist (ignored)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, provider := tt.prepare()
			controller.epsWorkListFunc = provider
			err := controller.removeOrphanWork(ctx, tt.endpointSliceObjects, serviceExportKey)
			tt.verify(t, controller.Client, err)
		})
	}
}

func TestSyncServiceExportOrEndpointSlice(t *testing.T) {
	ctx := context.TODO()
	clusterName := "member1"
	namespace := "test-ns"
	serviceName := "test-service"
	endpointSliceName := "test-eps"

	// Helper function to create ServiceExport FederatedKey
	createServiceExportKey := func() keys.FederatedKey {
		return keys.FederatedKey{
			Cluster: clusterName,
			ClusterWideKey: keys.ClusterWideKey{
				Group:     "multicluster.x-k8s.io",
				Version:   "v1alpha1",
				Kind:      util.ServiceExportKind,
				Namespace: namespace,
				Name:      serviceName,
			},
		}
	}

	// Helper function to create EndpointSlice FederatedKey
	createEndpointSliceKey := func() keys.FederatedKey {
		return keys.FederatedKey{
			Cluster: clusterName,
			ClusterWideKey: keys.ClusterWideKey{
				Group:     "discovery.k8s.io",
				Version:   "v1",
				Kind:      util.EndpointSliceKind,
				Namespace: namespace,
				Name:      endpointSliceName,
			},
		}
	}

	// Helper function to create EndpointSlice objects
	createEndpointSlice := func(name, ns string) *unstructured.Unstructured {
		eps := &unstructured.Unstructured{}
		eps.SetAPIVersion("discovery.k8s.io/v1")
		eps.SetKind("EndpointSlice")
		eps.SetName(name)
		eps.SetNamespace(ns)
		eps.SetLabels(map[string]string{
			discoveryv1.LabelServiceName: serviceName,
		})
		return eps
	}

	// Helper function to create ServiceExport objects
	createServiceExport := func(name, ns string) *unstructured.Unstructured {
		se := &unstructured.Unstructured{}
		se.SetAPIVersion("multicluster.x-k8s.io/v1alpha1")
		se.SetKind("ServiceExport")
		se.SetName(name)
		se.SetNamespace(ns)
		return se
	}

	execNS := names.GenerateExecutionSpaceName(clusterName)

	// Helper function to create RESTMapper
	createRESTMapper := func() meta.RESTMapper {
		restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
			{Group: "multicluster.x-k8s.io", Version: "v1alpha1"},
			{Group: "discovery.k8s.io", Version: "v1"},
		})
		restMapper.Add(schema.GroupVersionKind{
			Group:   "multicluster.x-k8s.io",
			Version: "v1alpha1",
			Kind:    util.ServiceExportKind,
		}, meta.RESTScopeNamespace)
		restMapper.Add(schema.GroupVersionKind{
			Group:   "discovery.k8s.io",
			Version: "v1",
			Kind:    util.EndpointSliceKind,
		}, meta.RESTScopeNamespace)
		return restMapper
	}

	tests := []struct {
		name    string
		prepare func() (*ServiceExportController, util.QueueKey)
		verify  func(t *testing.T, c client.Client, err error)
	}{
		{
			name: "invalid key type should return error",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				return &ServiceExportController{}, "invalid-key"
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid key")
			},
		},
		{
			name: "unknown kind should be ignored without error",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				return &ServiceExportController{}, keys.FederatedKey{
					Cluster: clusterName,
					ClusterWideKey: keys.ClusterWideKey{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment", // Unknown kind to this controller
						Namespace: namespace,
						Name:      "test-deployment",
					},
				}
			},
			verify: func(t *testing.T, _ client.Client, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "ServiceExport event should create Work when ServiceExport exists and EndpointSlices are found",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				eps1 := createEndpointSlice("eps-1", namespace)
				eps2 := createEndpointSlice("eps-2", namespace)
				serviceExport := createServiceExport(serviceName, namespace)

				controller := &ServiceExportController{
					Client:     fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					RESTMapper: createRESTMapper(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(gvr schema.GroupVersionResource) cache.GenericLister {
							if gvr == endpointSliceGVR {
								return &mockLister{
									listFunc: func(_ labels.Selector) ([]runtime.Object, error) {
										return []runtime.Object{eps1, eps2}, nil
									},
								}
							}
							if gvr == serviceExportGVR {
								return &mockLister{
									getFunc: func(name string) (runtime.Object, error) {
										// Handle both "namespace/name" format (for helper.GetObjectFromCache)
										// and "name" format (for ByNamespace().Get())
										expectedKey := namespace + "/" + serviceName
										if name == expectedKey || name == serviceName {
											return serviceExport, nil
										}
										return nil, apierrors.NewNotFound(schema.GroupResource{Group: "multicluster.x-k8s.io", Resource: "serviceexports"}, name)
									},
								}
							}
							return &mockLister{}
						}),
					}),
					epsWorkListFunc: func(_ context.Context, _ keys.FederatedKey) (*workv1alpha1.WorkList, error) {
						return &workv1alpha1.WorkList{}, nil
					},
				}

				key := createServiceExportKey()
				return controller, key
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// List all works in the execution namespace to debug
				workList := &workv1alpha1.WorkList{}
				listErr := c.List(ctx, workList, client.InNamespace(execNS))
				assert.NoError(t, listErr, "Should be able to list works")

				// Verify that Work resources are created for EndpointSlices
				eps1WorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-1", namespace)
				eps2WorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-2", namespace)

				work1 := &workv1alpha1.Work{}
				err1Get := c.Get(ctx, types.NamespacedName{Name: eps1WorkName, Namespace: execNS}, work1)
				assert.NoError(t, err1Get, "Work for eps-1 should be created")

				work2 := &workv1alpha1.Work{}
				err2Get := c.Get(ctx, types.NamespacedName{Name: eps2WorkName, Namespace: execNS}, work2)
				assert.NoError(t, err2Get, "Work for eps-2 should be created")
			},
		},
		{
			name: "ServiceExport event should cleanup works when ServiceExport is deleted",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				// Create an existing work that should be cleaned up
				existingWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName(util.EndpointSliceKind, "eps-1", namespace),
						Namespace: execNS,
						Labels: map[string]string{
							util.ServiceNamespaceLabel:           namespace,
							util.ServiceNameLabel:                serviceName,
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte(`{
											"apiVersion": "discovery.k8s.io/v1",
											"kind": "EndpointSlice",
											"metadata": {
												"name": "eps-1",
												"namespace": "` + namespace + `"
											}
										}`),
									},
								},
							},
						},
					},
				}

				controller := &ServiceExportController{
					Client:     fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(existingWork).Build(),
					RESTMapper: createRESTMapper(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(gvr schema.GroupVersionResource) cache.GenericLister {
							if gvr == serviceExportGVR {
								return &mockLister{
									getFunc: func(name string) (runtime.Object, error) {
										// ServiceExport not found (deleted)
										return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
									},
								}
							}
							return &mockLister{}
						}),
					}),
				}

				key := createServiceExportKey()
				return controller, key
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify that the existing work is deleted
				existingWorkName := names.GenerateWorkName(util.EndpointSliceKind, "eps-1", namespace)
				work := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: existingWorkName, Namespace: execNS}, work)
				assert.True(t, apierrors.IsNotFound(getErr), "Work should be deleted when ServiceExport is deleted")
			},
		},
		{
			name: "EndpointSlice event should create Work when ServiceExport exists",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				endpointSlice := createEndpointSlice(endpointSliceName, namespace)
				serviceExport := createServiceExport(serviceName, namespace)

				controller := &ServiceExportController{
					Client:     fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					RESTMapper: createRESTMapper(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(gvr schema.GroupVersionResource) cache.GenericLister {
							if gvr == endpointSliceGVR {
								return &mockLister{
									getFunc: func(name string) (runtime.Object, error) {
										// The key should be "namespace/name" format for namespaced resources
										expectedKey := namespace + "/" + endpointSliceName
										if name == expectedKey {
											return endpointSlice, nil
										}
										return nil, apierrors.NewNotFound(schema.GroupResource{Group: "discovery.k8s.io", Resource: "endpointslices"}, name)
									},
								}
							}
							if gvr == serviceExportGVR {
								return &mockLister{
									getFunc: func(name string) (runtime.Object, error) {
										// Handle both "namespace/name" format (for helper.GetObjectFromCache)
										// and "name" format (for ByNamespace().Get())
										expectedKey := namespace + "/" + serviceName
										if name == expectedKey || name == serviceName {
											return serviceExport, nil
										}
										return nil, apierrors.NewNotFound(schema.GroupResource{Group: "multicluster.x-k8s.io", Resource: "serviceexports"}, name)
									},
								}
							}
							return &mockLister{}
						}),
					}),
				}

				key := createEndpointSliceKey()
				return controller, key
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify that Work resource is created for the EndpointSlice
				workName := names.GenerateWorkName(util.EndpointSliceKind, endpointSliceName, namespace)
				work := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, work)
				assert.NoError(t, getErr, "Work should be created for EndpointSlice")
			},
		},
		{
			name: "EndpointSlice event should cleanup work when EndpointSlice is deleted",
			prepare: func() (*ServiceExportController, util.QueueKey) {
				// Create an existing work that should be cleaned up
				existingWork := &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.GenerateWorkName(util.EndpointSliceKind, endpointSliceName, namespace),
						Namespace: execNS,
						Labels: map[string]string{
							util.EndpointSliceWorkManagedByLabel: util.ServiceExportKind,
						},
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte(`{
											"apiVersion": "discovery.k8s.io/v1",
											"kind": "EndpointSlice",
											"metadata": {
												"name": "` + endpointSliceName + `",
												"namespace": "` + namespace + `"
											}
										}`),
									},
								},
							},
						},
					},
				}

				controller := &ServiceExportController{
					Client:     fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(existingWork).Build(),
					RESTMapper: createRESTMapper(),
					InformerManager: testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
						clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, false, func(gvr schema.GroupVersionResource) cache.GenericLister {
							if gvr == endpointSliceGVR {
								return &mockLister{
									getFunc: func(name string) (runtime.Object, error) {
										// EndpointSlice not found (deleted)
										return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
									},
								}
							}
							return &mockLister{}
						}),
					}),
				}

				key := createEndpointSliceKey()
				return controller, key
			},
			verify: func(t *testing.T, c client.Client, err error) {
				assert.NoError(t, err)

				// Verify that the existing work is deleted
				workName := names.GenerateWorkName(util.EndpointSliceKind, endpointSliceName, namespace)
				work := &workv1alpha1.Work{}
				getErr := c.Get(ctx, types.NamespacedName{Name: workName, Namespace: execNS}, work)
				assert.True(t, apierrors.IsNotFound(getErr), "Work should be deleted when EndpointSlice is deleted")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, key := tt.prepare()
			err := controller.syncServiceExportOrEndpointSlice(key)
			tt.verify(t, controller.Client, err)
		})
	}
}

func TestReconcile(t *testing.T) {
	ctx := context.TODO()
	clusterName := "member1"
	workName := "test-work"

	// Helper function to create Work objects
	createWork := func(name, ns string, deletionTimestamp *metav1.Time, applied bool, withServiceExport bool) *workv1alpha1.Work {
		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Namespace:         ns,
				DeletionTimestamp: deletionTimestamp,
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{},
			},
		}

		// Add applied condition if specified
		if applied {
			work.Status.Conditions = []metav1.Condition{
				{
					Type:   workv1alpha1.WorkApplied,
					Status: metav1.ConditionTrue,
				},
			}
		}

		// Add ServiceExport manifest if specified
		if withServiceExport {
			serviceExportManifest := runtime.RawExtension{
				Raw: []byte(`{
					"apiVersion": "multicluster.x-k8s.io/v1alpha1",
					"kind": "ServiceExport",
					"metadata": {
						"name": "test-service",
						"namespace": "test-ns"
					}
				}`),
			}
			work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
				{RawExtension: serviceExportManifest},
			}
		}

		return work
	}

	// Helper function to create Cluster objects
	createCluster := func(name string, ready bool) *clusterv1alpha1.Cluster {
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if ready {
			cluster.Status.Conditions = []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			}
		}

		return cluster
	}

	tests := []struct {
		name    string
		prepare func() (*ServiceExportController, controllerruntime.Request)
		verify  func(t *testing.T, result controllerruntime.Result, err error)
	}{
		{
			name: "Work not found should return no error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.NoError(t, err)
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Work Get error should return error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				client := &mockErrorClient{
					Client:   fake.NewClientBuilder().WithScheme(newTestScheme()).Build(),
					getError: fmt.Errorf("client error"),
				}
				controller := &ServiceExportController{
					Client: client,
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "client error")
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Work with deletion timestamp should return no error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				deletionTime := metav1.Now()
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), &deletionTime, false, false)
				// Add finalizers to make the fake client accept the object with deletionTimestamp
				work.Finalizers = []string{"test-finalizer"}
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.NoError(t, err)
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Work not applied should return no error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), nil, false, false)
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.NoError(t, err)
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Work without ServiceExport should return no error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), nil, true, false)
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.NoError(t, err)
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Invalid cluster name should return error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, "invalid-namespace", nil, true, true)
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: "invalid-namespace",
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "wrong format")
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Cluster not found should return error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), nil, true, true)
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.Error(t, err)
				// The error could be "not found" or scheme-related
				assert.True(t, strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no kind is registered"))
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Cluster not ready should return error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), nil, true, true)
				cluster := createCluster(clusterName, false) // not ready
				controller := &ServiceExportController{
					Client: fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work, cluster).Build(),
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not ready")
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
		{
			name: "Successful reconcile should return no error",
			prepare: func() (*ServiceExportController, controllerruntime.Request) {
				work := createWork(workName, names.GenerateExecutionSpaceName(clusterName), nil, true, true)
				cluster := createCluster(clusterName, true)

				// Create a mock informer manager with all informers already synced
				mockManager := testinggenericmanager.NewFakeMultiClusterInformerManager(map[string]genericmanager.SingleClusterInformerManager{
					clusterName: testinggenericmanager.NewFakeSingleClusterManager(true, true, func(_ schema.GroupVersionResource) cache.GenericLister {
						return &mockLister{}
					}),
				})

				controller := &ServiceExportController{
					Client:          fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(work, cluster).Build(),
					InformerManager: mockManager,
				}

				req := controllerruntime.Request{
					NamespacedName: types.NamespacedName{
						Name:      workName,
						Namespace: names.GenerateExecutionSpaceName(clusterName),
					},
				}

				return controller, req
			},
			verify: func(t *testing.T, result controllerruntime.Result, err error) {
				assert.NoError(t, err)
				assert.Equal(t, controllerruntime.Result{}, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, req := tt.prepare()
			result, err := controller.Reconcile(ctx, req)
			tt.verify(t, result, err)
		})
	}
}
