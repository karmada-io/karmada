package defaultinterpreter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_interpretDeploymentHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "failed convert to Deployment object",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec":   "format error",
					"status": "format error",
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "deployment healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "generation not equal to observedGeneration",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    3,
						"observedGeneration": 2,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "replicas not equal to updatedReplicas",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    1,
						"observedGeneration": 1,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpretDeploymentHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretDeploymentHealth() err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("interpretDeploymentHealth() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretStatefulSetHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "failed convert to StatefulSet object",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name": "fake-statefulSet",
					},
					"spec":   "format error",
					"status": "format error",
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "statefulSet healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":       "fake-statefulSet",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "generation not equal to observedGeneration",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":       "fake-statefulSet",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    3,
						"observedGeneration": 2,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "replicas not equal to updatedReplicas",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":       "fake-statefulSet",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    1,
						"observedGeneration": 1,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpretStatefulSetHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretStatefulSetHealth() err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("interpretStatefulSetHealth() got = %v, want %v", got, tt.want)
			}
		})
	}
}
