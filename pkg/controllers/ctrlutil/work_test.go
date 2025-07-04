/*
Copyright 2022 The Karmada Authors.

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

package ctrlutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestCreateOrUpdateWork(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1alpha1.Install(scheme))
	assert.NoError(t, workv1alpha2.Install(scheme))

	tests := []struct {
		name         string
		existingWork *workv1alpha1.Work
		workMeta     metav1.ObjectMeta
		resource     *unstructured.Unstructured
		wantErr      bool
		verify       func(*testing.T, client.Client)
	}{
		{
			name: "create new work",
			workMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-work",
			},
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-deployment",
						"uid":  "test-uid",
					},
				},
			},
			verify: func(t *testing.T, c client.Client) {
				work := &workv1alpha1.Work{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-work"}, work)
				assert.NoError(t, err)
				assert.Equal(t, "test-work", work.Name)
				assert.Equal(t, 1, len(work.Spec.Workload.Manifests))
			},
		},
		{
			name: "update existing work",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test-work",
				},
			},
			workMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-work",
			},
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-deployment",
						"uid":  "test-uid",
					},
				},
			},
			verify: func(t *testing.T, c client.Client) {
				work := &workv1alpha1.Work{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: "default", Name: "test-work"}, work)
				assert.NoError(t, err)
				assert.Equal(t, 1, len(work.Spec.Workload.Manifests))
			},
		},
		{
			name: "error when work is being deleted",
			existingWork: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "default",
					Name:              "test-work",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test.finalizer.io"}, // Finalizer to satisfy fake client requirement
				},
				Spec: workv1alpha1.WorkSpec{
					Workload: workv1alpha1.WorkloadTemplate{
						Manifests: []workv1alpha1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`),
								},
							},
						},
					},
				},
			},
			workMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-work",
			},
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "test-deployment",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme)
			if tt.existingWork != nil {
				c = c.WithObjects(tt.existingWork)
			}
			client := c.Build()

			err := CreateOrUpdateWork(context.TODO(), client, tt.workMeta, tt.resource)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, client)
			}
		})
	}
}
