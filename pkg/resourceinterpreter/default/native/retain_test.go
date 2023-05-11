package native

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRetainPodFields(t *testing.T) {
	tests := []struct {
		name        string
		desiredPod  *unstructured.Unstructured
		clusterPod  *unstructured.Unstructured
		expectedPod *unstructured.Unstructured
	}{
		{
			"cluster pod equals desired pod",
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
					},
					"spec": map[string]interface{}{
						"containers": []map[string]interface{}{
							{
								"name":  "web",
								"image": "nginx",
							},
						},
					},
				},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
					},
					"spec": map[string]interface{}{
						"containers": []map[string]interface{}{
							{
								"name":  "web",
								"image": "nginx",
							},
						},
					},
				},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":      "web",
								"image":     "nginx",
								"resources": map[string]interface{}{},
							},
						},
					},
					"status": map[string]interface{}{},
				},
			},
		},
		{
			"desired pod updates available fields",
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
						"labels": map[string]interface{}{
							"test-env": "foo",
						},
						"annotations": map[string]interface{}{
							"test-env": "bar",
						},
					},
					"spec": map[string]interface{}{
						"initContainers": []interface{}{
							map[string]interface{}{
								"name":  "check",
								"image": "busybox:1.0.1",
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "web",
								"image": "nginx:1.0.1",
							},
						},
						"nodeName":              "master1",
						"activeDeadlineSeconds": 10,
						"tolerations": []interface{}{
							map[string]interface{}{
								"key":      "test-env",
								"operator": "Exists",
							},
							map[string]interface{}{
								"key":      "dev-env",
								"operator": "Exists",
							},
							map[string]interface{}{
								"key":               "Unschedulable",
								"operator":          "Exists",
								"effect":            "NoExecute",
								"tolerationSeconds": 30,
							},
						},
					},
				},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
					},
					"spec": map[string]interface{}{
						"initContainers": []interface{}{
							map[string]interface{}{
								"name":  "check",
								"image": "busybox:1.0.0",
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "web",
								"image": "nginx:1.0.0",
							},
						},
						"nodeName": "master0",
						"tolerations": []interface{}{
							map[string]interface{}{
								"key":      "test-env",
								"operator": "Exists",
							},
							map[string]interface{}{
								"key":      "Unschedulable",
								"operator": "Exists",
								"effect":   "NoExecute",
							},
						},
					},
				},
			},
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":              "test-pod",
						"namespace":         "test",
						"creationTimestamp": "2023-05-11T05:00:41Z",
						"labels": map[string]interface{}{
							"test-env": "foo",
						},
						"annotations": map[string]interface{}{
							"test-env": "bar",
						},
					},
					"spec": map[string]interface{}{
						"initContainers": []interface{}{
							map[string]interface{}{
								"name":      "check",
								"image":     "busybox:1.0.1",
								"resources": map[string]interface{}{},
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"name":      "web",
								"image":     "nginx:1.0.1",
								"resources": map[string]interface{}{},
							},
						},
						"nodeName":              "master0",
						"activeDeadlineSeconds": int64(10),
						"tolerations": []interface{}{
							map[string]interface{}{
								"key":      "test-env",
								"operator": "Exists",
							},
							map[string]interface{}{
								"key":      "Unschedulable",
								"operator": "Exists",
								"effect":   "NoExecute",
							},
							map[string]interface{}{
								"key":      "dev-env",
								"operator": "Exists",
							},
						},
					},
					"status": map[string]interface{}{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retainedPod, err := retainPodFields(tt.desiredPod, tt.clusterPod)
			if err != nil {
				t.Errorf("test %q failed: unexpected error %v", tt.name, err)
			}

			if !reflect.DeepEqual(*retainedPod, *tt.expectedPod) {
				t.Errorf("test %q failed: retained pod is unexpected: %v", tt.name, retainedPod)
			}
		})
	}
}
