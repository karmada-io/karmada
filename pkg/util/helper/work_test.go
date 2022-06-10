package helper

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
			name: "missing metadata.uid but has resourcetemplate.karmada.io/uid annontation",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GenEventRef(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenEventRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("GenEventRef() = %v, want %v", actual, tt.want)
			}
		})
	}
}
