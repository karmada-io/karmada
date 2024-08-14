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

package namespace

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
)

func TestController_namespaceShouldBeSynced(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		skipped   []*regexp.Regexp
		want      bool
	}{
		{
			name:      "Reserved namespace",
			namespace: "kube-system",
			want:      true,
		},
		{
			name:      "Default namespace",
			namespace: "default",
			want:      false,
		},
		{
			name:      "Regular namespace",
			namespace: "test-namespace",
			want:      true,
		},
		{
			name:      "Skipped namespace",
			namespace: "skip-me",
			skipped:   []*regexp.Regexp{regexp.MustCompile("skip-.*")},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				SkippedPropagatingNamespaces: tt.skipped,
			}
			got := c.namespaceShouldBeSynced(tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestController_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1alpha1.Install(scheme)
	_ = policyv1alpha1.Install(scheme)
	_ = workv1alpha1.Install(scheme)

	tests := []struct {
		name           string
		namespace      *corev1.Namespace
		clusters       []clusterv1alpha1.Cluster
		expectedResult controllerruntime.Result
		expectedError  bool
	}{
		{
			name: "Namespace should be synced",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			},
			clusters: []clusterv1alpha1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			expectedResult: controllerruntime.Result{},
			expectedError:  false,
		},
		{
			name: "Namespace should not be synced",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-system",
				},
			},
			expectedResult: controllerruntime.Result{},
			expectedError:  false,
		},
		{
			name: "Namespace with skip auto propagation label",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "skip-namespace",
					Labels: map[string]string{
						policyv1alpha1.NamespaceSkipAutoPropagationLabel: "true",
					},
				},
			},
			expectedResult: controllerruntime.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.namespace).WithLists(&clusterv1alpha1.ClusterList{Items: tt.clusters}).Build()
			fakeRecorder := record.NewFakeRecorder(100)

			c := &Controller{
				Client:          fakeClient,
				EventRecorder:   fakeRecorder,
				OverrideManager: overridemanager.New(fakeClient, fakeRecorder),
			}

			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name: tt.namespace.Name,
				},
			}

			result, err := c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedError, err != nil)

			if tt.name == "Namespace should be synced" {
				work := &workv1alpha1.Work{}
				err = fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: names.GenerateExecutionSpaceName(tt.clusters[0].Name),
					Name:      names.GenerateWorkName(tt.namespace.Kind, tt.namespace.Name, tt.namespace.Namespace),
				}, work)
				assert.NoError(t, err)
				assert.NotNil(t, work)
			}
		})
	}
}

func TestController_buildWorks(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1alpha1.Install(scheme)
	_ = workv1alpha1.Install(scheme)
	_ = policyv1alpha1.Install(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	clusters := []clusterv1alpha1.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster2",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace, &clusters[0], &clusters[1]).Build()
	fakeRecorder := record.NewFakeRecorder(100)

	c := &Controller{
		Client:          fakeClient,
		EventRecorder:   fakeRecorder,
		OverrideManager: overridemanager.New(fakeClient, fakeRecorder),
	}

	err := c.buildWorks(context.Background(), namespace, clusters)
	assert.NoError(t, err)

	for _, cluster := range clusters {
		work := &workv1alpha1.Work{}
		err = fakeClient.Get(context.Background(), types.NamespacedName{
			Namespace: names.GenerateExecutionSpaceName(cluster.Name),
			Name:      names.GenerateWorkName(namespace.Kind, namespace.Name, namespace.Namespace),
		}, work)
		assert.NoError(t, err)
		assert.NotNil(t, work)
		assert.Equal(t, namespace.Name, work.OwnerReferences[0].Name)
	}
}

func TestController_buildWorksWithOverridePolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1alpha1.Install(scheme)
	_ = workv1alpha1.Install(scheme)
	_ = policyv1alpha1.Install(scheme)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}

	clusters := []clusterv1alpha1.Cluster{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster1",
			},
		},
	}

	overridePolicy := &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-override",
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: "v1",
					Kind:       "Namespace",
					Name:       "test-namespace",
				},
			},
			OverrideRules: []policyv1alpha1.RuleWithCluster{
				{
					TargetCluster: &policyv1alpha1.ClusterAffinity{
						ClusterNames: []string{"cluster1"},
					},
					Overriders: policyv1alpha1.Overriders{
						Plaintext: []policyv1alpha1.PlaintextOverrider{
							{
								Path:     "/metadata/labels/overridden",
								Operator: "add",
								Value: apiextensionsv1.JSON{
									Raw: []byte(`"true"`),
								},
							},
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace, &clusters[0], overridePolicy).Build()
	fakeRecorder := record.NewFakeRecorder(100)

	c := &Controller{
		Client:          fakeClient,
		EventRecorder:   fakeRecorder,
		OverrideManager: overridemanager.New(fakeClient, fakeRecorder),
	}

	err := c.buildWorks(context.Background(), namespace, clusters)
	assert.NoError(t, err)

	work := &workv1alpha1.Work{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: names.GenerateExecutionSpaceName(clusters[0].Name),
		Name:      names.GenerateWorkName(namespace.Kind, namespace.Name, namespace.Namespace),
	}, work)
	assert.NoError(t, err)
	assert.NotNil(t, work)

	t.Logf("Work found: %+v", work)
	t.Logf("Work annotations: %v", work.Annotations)
	t.Logf("Work spec: %+v", work.Spec)

	assert.NotNil(t, work)
}

func TestController_SetupWithManager(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = clusterv1alpha1.Install(scheme)
	_ = workv1alpha1.Install(scheme)

	mgr, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{Scheme: scheme})
	assert.NoError(t, err)

	c := &Controller{
		Client:          mgr.GetClient(),
		EventRecorder:   mgr.GetEventRecorderFor("test-controller"),
		OverrideManager: overridemanager.New(mgr.GetClient(), mgr.GetEventRecorderFor("test-controller")),
	}

	err = c.SetupWithManager(mgr)
	assert.NoError(t, err)
}

func TestClusterNamespaceFn(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	namespace1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace1"}}
	namespace2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "namespace2"}}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace1, namespace2).
		Build()

	c := &Controller{Client: fakeClient}

	clusterNamespaceFn := handler.MapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			var requests []reconcile.Request
			namespaceList := &corev1.NamespaceList{}
			if err := c.Client.List(ctx, namespaceList); err != nil {
				klog.Errorf("Failed to list namespace, error: %v", err)
				return nil
			}

			for _, namespace := range namespaceList.Items {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: namespace.Name,
				}})
			}
			return requests
		})

	requests := clusterNamespaceFn(context.Background(), nil)
	assert.Len(t, requests, 2)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "namespace1"}})
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "namespace2"}})
}
func TestClusterOverridePolicyNamespaceFn(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = policyv1alpha1.Install(scheme)

	cop := &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{APIVersion: "v1", Kind: "Namespace", Name: "test-namespace"},
			},
		},
	}

	namespace1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-namespace"}}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(namespace1, cop).
		Build()

	c := &Controller{Client: fakeClient}

	clusterOverridePolicyNamespaceFn := handler.MapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			var requests []reconcile.Request
			cop, ok := obj.(*policyv1alpha1.ClusterOverridePolicy)
			if !ok {
				return requests
			}

			selectedNamespaces := sets.NewString()
			containsAllNamespace := false
			for _, rs := range cop.Spec.ResourceSelectors {
				if rs.APIVersion != "v1" || rs.Kind != "Namespace" {
					continue
				}

				if rs.Name == "" {
					containsAllNamespace = true
					break
				}

				selectedNamespaces.Insert(rs.Name)
			}

			if containsAllNamespace {
				namespaceList := &corev1.NamespaceList{}
				if err := c.Client.List(ctx, namespaceList); err != nil {
					klog.Errorf("Failed to list namespace, error: %v", err)
					return nil
				}

				for _, namespace := range namespaceList.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name: namespace.Name,
					}})
				}

				return requests
			}

			for _, ns := range selectedNamespaces.UnsortedList() {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name: ns,
				}})
			}

			return requests
		})

	requests := clusterOverridePolicyNamespaceFn(context.Background(), cop)
	assert.Len(t, requests, 1)
	assert.Contains(t, requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: "test-namespace"}})
}
