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

package namespace

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
		name              string
		namespace         *corev1.Namespace
		clusters          []clusterv1alpha1.Cluster
		expectedResult    controllerruntime.Result
		expectedError     bool
		expectWorkCreated bool
		namespaceNotFound bool
		namespaceDeleting bool
	}{
		{
			name: "Namespace should be synced",
			namespace: &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
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
			expectedResult:    controllerruntime.Result{},
			expectedError:     false,
			expectWorkCreated: true,
		},
		{
			name: "Namespace should not be synced",
			namespace: &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-system",
				},
			},
			expectedResult:    controllerruntime.Result{},
			expectedError:     false,
			expectWorkCreated: false,
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
		{
			name: "Namespace should not be synced - kube-public",
			namespace: &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-public",
				},
			},
			expectedResult:    controllerruntime.Result{},
			expectedError:     false,
			expectWorkCreated: false,
		},
		{
			name:              "Namespace not found",
			namespace:         &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "non-existent"}},
			namespaceNotFound: true,
			expectedResult:    controllerruntime.Result{},
			expectedError:     false,
			expectWorkCreated: false,
		},
		{
			name: "Namespace is being deleted",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"some-finalizer"},
				},
			},
			namespaceDeleting: true,
			expectedResult:    controllerruntime.Result{},
			expectedError:     false,
			expectWorkCreated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme)

			if !tt.namespaceNotFound {
				fakeClientBuilder = fakeClientBuilder.WithObjects(tt.namespace)
			}

			fakeClientBuilder = fakeClientBuilder.WithLists(&clusterv1alpha1.ClusterList{Items: tt.clusters})
			fakeClient := fakeClientBuilder.Build()
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

			if tt.namespaceNotFound {
				ns := &corev1.Namespace{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{Name: tt.namespace.Name}, ns)
				assert.True(t, apierrors.IsNotFound(err))
			} else if tt.namespaceDeleting {
				ns := &corev1.Namespace{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{Name: tt.namespace.Name}, ns)
				assert.NoError(t, err)
				assert.NotNil(t, ns.DeletionTimestamp)
			}

			if tt.expectWorkCreated {
				if len(tt.clusters) == 0 {
					t.Fatal("Test case expects work to be created but no clusters are defined")
				}
				work := &workv1alpha1.Work{}
				err = fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: names.GenerateExecutionSpaceName(tt.clusters[0].Name),
					Name:      names.GenerateWorkName(tt.namespace.Kind, tt.namespace.Name, tt.namespace.Namespace),
				}, work)
				assert.NoError(t, err)
				assert.NotNil(t, work)
			} else if len(tt.clusters) > 0 {
				work := &workv1alpha1.Work{}
				err = fakeClient.Get(context.Background(), types.NamespacedName{
					Namespace: names.GenerateExecutionSpaceName(tt.clusters[0].Name),
					Name:      names.GenerateWorkName(tt.namespace.Kind, tt.namespace.Name, tt.namespace.Namespace),
				}, work)
				assert.Error(t, err)
				assert.True(t, apierrors.IsNotFound(err))
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
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"overridden": "false",
			},
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

	overridePolicy := &policyv1alpha1.ClusterOverridePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-override",
		},
		Spec: policyv1alpha1.OverrideSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: corev1.SchemeGroupVersion.String(),
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
								Operator: policyv1alpha1.OverriderOpAdd,
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

	tests := []struct {
		name           string
		withOverride   bool
		overrideResult map[string]string
	}{
		{
			name:         "Build works without override",
			withOverride: false,
			overrideResult: map[string]string{
				"cluster1": "false",
				"cluster2": "false",
			},
		},
		{
			name:         "Build works with override",
			withOverride: true,
			overrideResult: map[string]string{
				"cluster1": "true",
				"cluster2": "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace, &clusters[0], &clusters[1])
			if tt.withOverride {
				fakeClientBuilder = fakeClientBuilder.WithObjects(overridePolicy)
			}
			fakeClient := fakeClientBuilder.Build()
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

				assert.NotEmpty(t, work.Spec.Workload.Manifests)

				namespaceToExec := &corev1.Namespace{}
				err = json.Unmarshal(work.Spec.Workload.Manifests[0].Raw, namespaceToExec)
				assert.NoError(t, err)

				assert.Equal(t, namespaceToExec.Labels["overridden"], tt.overrideResult[cluster.Name])
			}
		})
	}
}

func TestController_SetupWithManager(t *testing.T) {
	tests := []struct {
		name        string
		setupScheme func(*runtime.Scheme)
		expectError bool
	}{
		{
			name: "Successful setup",
			setupScheme: func(scheme *runtime.Scheme) {
				_ = corev1.AddToScheme(scheme)
				_ = clusterv1alpha1.Install(scheme)
				_ = workv1alpha1.Install(scheme)
				_ = policyv1alpha1.Install(scheme)
			},
			expectError: false,
		},
		{
			name:        "Setup with error",
			setupScheme: func(_ *runtime.Scheme) {}, // Intentionally empty to trigger error
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			tt.setupScheme(scheme)

			cfg := &rest.Config{Host: "http://localhost:8080"}
			mgr, err := controllerruntime.NewManager(cfg, controllerruntime.Options{
				Scheme: scheme,
			})
			assert.NoError(t, err)

			c := &Controller{
				Client:          mgr.GetClient(),
				EventRecorder:   mgr.GetEventRecorderFor("test-controller"),
				OverrideManager: overridemanager.New(mgr.GetClient(), mgr.GetEventRecorderFor("test-controller")),
			}
			err = c.SetupWithManager(mgr)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, c.Client, "Controller's Client should not be nil")
				assert.NotNil(t, c.EventRecorder, "Controller's EventRecorder should not be nil")
				assert.NotNil(t, c.OverrideManager, "Controller's OverrideManager should not be nil")
			}
		})
	}
}
