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

package helper

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
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
			name: "create work with PropagationInstruction",
			workMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-work",
				Labels: map[string]string{
					util.PropagationInstruction: "some-value",
				},
				Annotations: map[string]string{
					workv1alpha2.ResourceConflictResolutionAnnotation: "overwrite",
				},
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

				// Get the resource from manifests
				manifest := &unstructured.Unstructured{}
				err = manifest.UnmarshalJSON(work.Spec.Workload.Manifests[0].Raw)
				assert.NoError(t, err)

				// Verify labels and annotations were set
				labels := manifest.GetLabels()
				assert.Equal(t, util.ManagedByKarmadaLabelValue, labels[util.ManagedByKarmadaLabel])

				annotations := manifest.GetAnnotations()
				assert.Equal(t, "test-uid", annotations[workv1alpha2.ResourceTemplateUIDAnnotation])
				assert.Equal(t, "test-work", annotations[workv1alpha2.WorkNameAnnotation])
				assert.Equal(t, "default", annotations[workv1alpha2.WorkNamespaceAnnotation])
				assert.Equal(t, "overwrite", annotations[workv1alpha2.ResourceConflictResolutionAnnotation])
			},
		},
		{
			name: "create work with PropagationInstructionSuppressed",
			workMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test-work",
				Labels: map[string]string{
					util.PropagationInstruction: util.PropagationInstructionSuppressed,
				},
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

				// Get the resource from manifests
				manifest := &unstructured.Unstructured{}
				err = manifest.UnmarshalJSON(work.Spec.Workload.Manifests[0].Raw)
				assert.NoError(t, err)

				// Verify labels and annotations were NOT set
				labels := manifest.GetLabels()
				assert.Empty(t, labels[util.ManagedByKarmadaLabel])

				annotations := manifest.GetAnnotations()
				assert.Empty(t, annotations[workv1alpha2.ResourceTemplateUIDAnnotation])
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

func TestGetWorksByLabelsSet(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1alpha1.Install(scheme))

	work1 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work1",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}
	work2 := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work2",
			Labels: map[string]string{
				"app": "other",
			},
		},
	}

	tests := []struct {
		name      string
		works     []client.Object
		labels    labels.Set
		wantCount int
	}{
		{
			name:      "no works exist",
			works:     []client.Object{},
			labels:    labels.Set{"app": "test"},
			wantCount: 0,
		},
		{
			name:      "find work by label",
			works:     []client.Object{work1, work2},
			labels:    labels.Set{"app": "test"},
			wantCount: 1,
		},
		{
			name:      "find multiple works",
			works:     []client.Object{work1, work2},
			labels:    labels.Set{},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.works...).
				Build()

			workList, err := GetWorksByLabelsSet(context.TODO(), client, tt.labels)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(workList.Items))
		})
	}
}

func TestGetWorksByBindingID(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, workv1alpha1.Install(scheme))

	bindingID := "test-binding-id"
	workWithRBID := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workWithRBID",
			Labels: map[string]string{
				workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
			},
		},
	}
	workWithCRBID := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workWithCRBID",
			Labels: map[string]string{
				workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
			},
		},
	}

	tests := []struct {
		name           string
		works          []client.Object
		bindingID      string
		permanentIDKey string
		namespaced     bool
		wantWorks      []string
	}{
		{
			name:           "find namespaced binding works",
			works:          []client.Object{workWithRBID, workWithCRBID},
			bindingID:      bindingID,
			permanentIDKey: workv1alpha2.ResourceBindingPermanentIDLabel,
			namespaced:     true,
			wantWorks:      []string{"workWithRBID"},
		},
		{
			name:           "find cluster binding works",
			works:          []client.Object{workWithRBID, workWithCRBID},
			bindingID:      bindingID,
			permanentIDKey: workv1alpha2.ClusterResourceBindingPermanentIDLabel,
			namespaced:     false,
			wantWorks:      []string{"workWithCRBID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(
					&workv1alpha1.Work{},
					tt.permanentIDKey,
					IndexerFuncBasedOnLabel(tt.permanentIDKey),
				).
				WithObjects(tt.works...).
				Build()

			workList, err := GetWorksByBindingID(context.TODO(), fakeClient, tt.bindingID, tt.namespaced)

			assert.NoError(t, err)
			assert.Equal(t, len(tt.wantWorks), len(workList.Items))

			// Verify the correct works were returned
			foundWorkNames := make([]string, len(workList.Items))
			for i, work := range workList.Items {
				foundWorkNames[i] = work.Name
			}
			assert.ElementsMatch(t, tt.wantWorks, foundWorkNames)
		})
	}
}

func TestGenEventRef(t *testing.T) {
	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		want    *corev1.ObjectReference
		wantErr bool
	}{
		{
			name: "has metadata.uid",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
						"uid":  "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid but has resourcetemplate.karmada.io/uid annotation",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"resourcetemplate.karmada.io/uid": "9249d2e7-3169-4c5f-be82-163bd80aa3cf"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid and metadata.annotations",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty kind",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"metadata": map[string]interface{}{
						"name": "test-obj",
						"uid":  "test-uid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"uid": "test-uid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing uid but has annotation",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "test-pod",
						"annotations": map[string]interface{}{
							workv1alpha2.ResourceTemplateUIDAnnotation: "annotation-uid",
						},
					},
				},
			},
			want: &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-pod",
				UID:        "annotation-uid",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GenEventRef(tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, actual)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, actual)
			}
		})
	}
}

func TestIsWorkContains(t *testing.T) {
	deployment := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}
	deploymentData, _ := deployment.MarshalJSON()

	service := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
		},
	}
	serviceData, _ := service.MarshalJSON()

	tests := []struct {
		name           string
		manifests      []workv1alpha1.Manifest
		targetResource schema.GroupVersionKind
		want           bool
	}{
		{
			name: "resource exists in manifests",
			manifests: []workv1alpha1.Manifest{
				{RawExtension: runtime.RawExtension{Raw: deploymentData}},
				{RawExtension: runtime.RawExtension{Raw: serviceData}},
			},
			targetResource: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want:           true,
		},
		{
			name: "resource does not exist in manifests",
			manifests: []workv1alpha1.Manifest{
				{RawExtension: runtime.RawExtension{Raw: serviceData}},
			},
			targetResource: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorkContains(tt.manifests, tt.targetResource)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsWorkSuspendDispatching(t *testing.T) {
	tests := []struct {
		name string
		work *workv1alpha1.Work
		want bool
	}{
		{
			name: "dispatching is suspended",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "dispatching is not suspended",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(false),
				},
			},
			want: false,
		},
		{
			name: "suspend dispatching is nil",
			work: &workv1alpha1.Work{
				Spec: workv1alpha1.WorkSpec{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorkSuspendDispatching(tt.work)
			assert.Equal(t, tt.want, got)
		})
	}
}
