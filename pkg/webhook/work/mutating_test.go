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

package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

type fakeMutationDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeMutationDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeMutationDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

func TestMutatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    admission.Response
	}{
		{
			name: "Handle_DecodeError_DeniesAdmission",
			decoder: &fakeMutationDecoder{
				err: errors.New("decode error"),
			},
			req:  admission.Request{},
			want: admission.Errored(http.StatusBadRequest, errors.New("decode error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := MutatingAdmission{
				Decoder: tt.decoder,
			}
			got := m.Handle(context.Background(), tt.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutatingAdmission_Handle_FullCoverage(t *testing.T) {
	// Define the work object name and namespace to be used in the test.
	name := "test-work"
	namespace := "test-namespace"

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-deployment",
			Namespace:                  namespace,
			CreationTimestamp:          metav1.Time{Time: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: func(i int64) *int64 { return &i }(30),
			Generation:                 2,
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "kube-controller-manager",
					Operation:  "Apply",
					APIVersion: "apps/v1",
				},
			},
			ResourceVersion: "123456",
			SelfLink:        fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/test-deployment", namespace),
			UID:             "abcd-1234-efgh-5678",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "owner-deployment",
					UID:        "owner-uid-1234",
				},
			},
			Finalizers:  []string{"foregroundDeletion"},
			Labels:      map[string]string{},
			Annotations: nil,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable-alpine-perl",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{},
	}

	// Marshal the Deployment object to JSON
	deploymentRaw, err := json.Marshal(deployment)
	if err != nil {
		fmt.Println("Error marshaling deployment:", err)
		return
	}

	// Mock a request with a specific namespace.
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: namespace,
			Operation: admissionv1.Create,
		},
	}

	// Create the initial work object with default values for testing.
	workObj := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: deploymentRaw,
						},
					},
				},
			},
		},
	}

	// Define the expected work object after mutations.
	wantDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: namespace,
			Annotations: map[string]string{
				workv1alpha2.ManagedAnnotation: strings.Join(
					[]string{workv1alpha2.ManagedAnnotation, workv1alpha2.ManagedLabels}, ",",
				),
				workv1alpha2.ManagedLabels: workv1alpha2.WorkPermanentIDLabel,
			},
			Labels: map[string]string{
				workv1alpha2.WorkPermanentIDLabel: "some-unique-id",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:stable-alpine-perl",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	// Marshal appsv1.Deployment object into JSON.
	wantDeploymentJSON, err := json.Marshal(wantDeployment)
	if err != nil {
		t.Errorf("Error marshaling deployment: %v", err)
	}

	// Convert the deployment object to unstructured to be able to delete some keys
	// expected after mutations.
	wantDeploymentUnstructured := &unstructured.Unstructured{}
	wantDeploymentUnstructured.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})

	// Unmarshal JSON into unstructured object.
	if err = json.Unmarshal(wantDeploymentJSON, wantDeploymentUnstructured); err != nil {
		t.Errorf("Error unmarshalling to unstructured: %v", err)
	}

	// Remove the status and creationTimestamp fields to simulate what happen after mutations.
	unstructured.RemoveNestedField(wantDeploymentUnstructured.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(wantDeploymentUnstructured.Object, "status")

	// Marshal the unstructured object back to JSON.
	wantDeploymentJSON, err = json.Marshal(wantDeploymentUnstructured)
	if err != nil {
		fmt.Println("Error marshaling modified deployment:", err)
		return
	}

	// Define the expected work object after mutations.
	wantWorkObj := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				workv1alpha2.WorkPermanentIDLabel: "some-unique-id",
			},
		},
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{
						RawExtension: runtime.RawExtension{
							Raw: wantDeploymentJSON,
						},
					},
				},
			},
		},
	}

	// Mock decoder that decodes the request into the work object.
	decoder := &fakeMutationDecoder{
		obj: workObj,
	}

	// Marshal the expected work object to simulate the final mutated object.
	wantBytes, err := json.Marshal(wantWorkObj)
	if err != nil {
		t.Fatalf("Failed to marshal expected work object: %v", err)
	}
	req.Object.Raw = wantBytes

	// Instantiate the mutating handler.
	mutatingHandler := MutatingAdmission{
		Decoder: decoder,
	}

	// Call the Handle function.
	got := mutatingHandler.Handle(context.Background(), req)

	// Check if exactly two patches are applied.
	if len(got.Patches) != 2 {
		t.Errorf("Handle() returned an unexpected number of patches. Expected 2 patches, received: %v", got.Patches)
	}

	// Verify that the patches are for the UUID label.
	// If any other patches are present, it indicates that the work object was not handled as expected.
	for _, patch := range got.Patches {
		if patch.Operation != "replace" ||
			(patch.Path != "/metadata/labels/work.karmada.io~1permanent-id" &&
				patch.Path != "/spec/workload/manifests/0/metadata/labels/work.karmada.io~1permanent-id") {
			t.Errorf("Handle() returned unexpected patches. Only the two UUID patches were expected. Received patches: %v", got.Patches)
		}
	}

	// Check if the admission request was allowed.
	if !got.Allowed {
		t.Errorf("Handle() got.Allowed = false, want true")
	}
}
