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

package applicationfailover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
)

func TestApplicationFailoverReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	tests := []struct {
		name           string
		applications   []client.Object
		request        ctrl.Request
		expectedResult ctrl.Result
		expectedError  bool
	}{
		{
			name: "successful failover with related applications",
			applications: []client.Object{
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-app",
						Namespace: "default",
					},
					Spec: v1alpha1.ApplicationSpec{
						RelatedApplications: []string{"related-app-1", "related-app-2"},
					},
				},
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "related-app-1",
						Namespace: "default",
					},
				},
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "related-app-2",
						Namespace: "default",
					},
				},
			},
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "main-app",
					Namespace: "default",
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "application not found",
			applications: []client.Object{},
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent-app",
					Namespace: "default",
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
		{
			name: "failover with missing related application",
			applications: []client.Object{
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main-app",
						Namespace: "default",
					},
					Spec: v1alpha1.ApplicationSpec{
						RelatedApplications: []string{"existing-app", "missing-app"},
					},
				},
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-app",
						Namespace: "default",
					},
				},
				// missing-app is intentionally not included
			},
			request: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "main-app",
					Namespace: "default",
				},
			},
			expectedResult: ctrl.Result{},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.applications...).
				Build()

			reconciler := &ApplicationFailoverReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			result, err := reconciler.Reconcile(context.Background(), tt.request)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestApplicationFailoverReconciler_migrateApplicationAndRelated(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	tests := []struct {
		name         string
		applications []client.Object
		mainApp      *v1alpha1.Application
		expectedErr  bool
	}{
		{
			name: "successful migration with related applications",
			applications: []client.Object{
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "related-app-1",
						Namespace: "default",
					},
				},
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "related-app-2",
						Namespace: "default",
					},
				},
			},
			mainApp: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "main-app",
					Namespace: "default",
				},
				Spec: v1alpha1.ApplicationSpec{
					RelatedApplications: []string{"related-app-1", "related-app-2"},
				},
			},
			expectedErr: false,
		},
		{
			name:         "migration with no related applications",
			applications: []client.Object{},
			mainApp: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "main-app",
					Namespace: "default",
				},
				Spec: v1alpha1.ApplicationSpec{
					RelatedApplications: []string{},
				},
			},
			expectedErr: false,
		},
		{
			name: "migration with missing related application",
			applications: []client.Object{
				&v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-app",
						Namespace: "default",
					},
				},
				// missing-app is intentionally not included
			},
			mainApp: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "main-app",
					Namespace: "default",
				},
				Spec: v1alpha1.ApplicationSpec{
					RelatedApplications: []string{"existing-app", "missing-app"},
				},
			},
			expectedErr: false, // Should not error, just skip missing apps
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.applications...).
				Build()

			reconciler := &ApplicationFailoverReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			err := reconciler.migrateApplicationAndRelated(context.Background(), tt.mainApp)

			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestApplicationFailoverReconciler_getApplicationByName(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	existingApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-app",
			Namespace: "default",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingApp).
		Build()

	reconciler := &ApplicationFailoverReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	tests := []struct {
		name        string
		appName     string
		namespace   string
		expectedErr bool
	}{
		{
			name:        "existing application",
			appName:     "existing-app",
			namespace:   "default",
			expectedErr: false,
		},
		{
			name:        "non-existent application",
			appName:     "non-existent-app",
			namespace:   "default",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := reconciler.getApplicationByName(context.Background(), tt.appName, tt.namespace)

			if tt.expectedErr {
				require.Error(t, err)
				require.Nil(t, app)
			} else {
				require.NoError(t, err)
				require.NotNil(t, app)
				require.Equal(t, tt.appName, app.Name)
				require.Equal(t, tt.namespace, app.Namespace)
			}
		})
	}
}