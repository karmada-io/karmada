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

package native

import (
	"reflect"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

var (
	namespace = "karmada-test"
	testPairs = []struct {
		podSpecsWithDependencies *unstructured.Unstructured
		dependentObjectReference []configv1alpha1.DependentObjectReference
	}{
		{
			podSpecsWithDependencies: &unstructured.Unstructured{
				Object: map[string]any{
					"serviceAccountName": "test-serviceaccount",
					"volumes": []any{
						map[string]any{
							"name": "initcontainer-volume-mount",
							"configMap": map[string]any{
								"name": "test-configmap",
							},
						},
						map[string]any{
							"name": "cloud-secret-volume",
							"secret": map[string]any{
								"secretName": "cloud-secret",
							},
						},
						map[string]any{
							"name": "container-volume-mount",
							"persistentVolumeClaim": map[string]any{
								"claimName": "test-pvc",
							},
						},
					},
				},
			},
			dependentObjectReference: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  namespace,
					Name:       "test-configmap",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "cloud-secret",
				},
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Namespace:  namespace,
					Name:       "test-serviceaccount",
				},
				{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
					Namespace:  namespace,
					Name:       "test-pvc",
				},
			},
		},
		{
			podSpecsWithDependencies: &unstructured.Unstructured{
				Object: map[string]any{
					"serviceAccountName": "test-serviceaccount",
					"imagePullSecrets": []any{
						map[string]any{
							"name": "test-secret",
						},
					},
					"initContainers": []any{
						map[string]any{
							"envFrom": []any{
								map[string]any{
									"configMapRef": map[string]any{
										"name": "initcontainer-envfrom-configmap",
									},
									"secretRef": map[string]any{
										"name": "test-secret",
									},
								},
							},
							"env": []any{
								map[string]any{
									"valueFrom": map[string]any{
										"secretKeyRef": map[string]any{
											"name": "test-secret",
										},
									},
								},
							},
						},
					},
				},
			},
			dependentObjectReference: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  namespace,
					Name:       "initcontainer-envfrom-configmap",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "test-secret",
				},
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Namespace:  namespace,
					Name:       "test-serviceaccount",
				},
			},
		},
		{
			podSpecsWithDependencies: &unstructured.Unstructured{
				Object: map[string]any{
					"containers": []any{
						map[string]any{
							"envFrom": []any{
								map[string]any{
									"configMapRef": map[string]any{
										"name": "test-configmap",
									},
								},
							},
							"env": []any{
								map[string]any{
									"valueFrom": map[string]any{
										"configMapKeyRef": map[string]any{
											"name": "test-configmap",
										},
									},
								},
							},
						},
					},
					"volumes": []any{
						map[string]any{
							"name": "container-volume-mount",
							"persistentVolumeClaim": map[string]any{
								"claimName": "test-pvc",
							},
							"azureFile": map[string]any{
								"secretName": "test-secret",
							},
						},
					},
				},
			},
			dependentObjectReference: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  namespace,
					Name:       "test-configmap",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "test-secret",
				},
				{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
					Namespace:  namespace,
					Name:       "test-pvc",
				},
			},
		},
	}
)

func Test_getDeploymentDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "deployment without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":       "fake-deployment",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "deployment with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":       "fake-deployment",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "deployment with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "fake-deployment",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "deployment with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name":      "fake-deployment",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[2].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getDeploymentDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeploymentDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDeploymentDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getReplicaSetDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "replicaset without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]any{
						"name":       "fake-replicaset",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "replicaset with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]any{
						"name":       "fake-replicaset",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "replicaset with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]any{
						"name":      "fake-replicaset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "replicaset with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"metadata": map[string]any{
						"name":      "fake-replicaset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"replicas": 3,
						"template": map[string]any{
							"spec": testPairs[2].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getReplicaSetDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getReplicaSetDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getReplicaSetDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getJobDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "job without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]any{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"parallelism": 1,
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "job with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]any{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"parallelism": 1,
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "job with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]any{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"parallelism": 1,
						"template": map[string]any{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "job with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]any{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"parallelism": 1,
						"template": map[string]any{
							"spec": testPairs[2].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getJobDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getJobDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getJobDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCronJobDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "cronjob without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]any{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"schedule": "*/1 * * * *",
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "cronjob with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]any{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]any{
							"spec": map[string]any{
								"template": map[string]any{
									"spec": testPairs[0].podSpecsWithDependencies.Object,
								},
							},
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "cronjob with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]any{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]any{
							"spec": map[string]any{
								"template": map[string]any{
									"spec": testPairs[1].podSpecsWithDependencies.Object,
								},
							},
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "cronjob with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]any{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]any{
							"spec": map[string]any{
								"template": map[string]any{
									"spec": testPairs[2].podSpecsWithDependencies.Object,
								},
							},
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getCronJobDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCronJobDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCronJobDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPodDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "pod without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "fake-pod",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "example-container",
								"image": "busybox",
								"command": []any{
									"echo",
									"Hello, World!",
								},
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "pod with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "fake-pod",
						"namespace": namespace,
					},
					"spec": testPairs[0].podSpecsWithDependencies.Object,
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "pod with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "fake-pod",
						"namespace": namespace,
					},
					"spec": testPairs[1].podSpecsWithDependencies.Object,
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "pod with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "batch/v1beta1",
					"kind":       "pod",
					"metadata": map[string]any{
						"name":      "fake-pod",
						"namespace": namespace,
					},
					"spec": testPairs[2].podSpecsWithDependencies.Object,
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPodDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPodDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPodDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDaemonSetDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "daemonset without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]any{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "daemonset with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]any{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "daemonset with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]any{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "daemonset with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]any{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[2].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getDaemonSetDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDaemonSetDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDaemonSetDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getStatefulSetDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "statefulset without dependencies",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"serviceName": "fake-service",
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "statefulset with dependencies 1",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"serviceName": "fake-service",
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "statefulset with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"serviceName": "fake-service",
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[1].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "statefulset with dependencies 3",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"serviceName": "fake-service",
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[2].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    testPairs[2].dependentObjectReference,
			wantErr: false,
		},
		{
			name: "statefulset with partial dependencies 4",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"serviceName": "fake-service",
						"selector": map[string]any{
							"matchLabels": map[string]any{
								"app": "fake",
							},
						},
						"template": map[string]any{
							"spec": testPairs[0].podSpecsWithDependencies.Object,
						},
						"volumeClaimTemplates": []any{
							map[string]any{
								"metadata": map[string]any{
									"name": "test-pvc",
								},
							},
						},
					},
				},
			},
			want:    testPairs[0].dependentObjectReference[:3], // remove the pvc dependency because it was found in the volumeClaimTemplates
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getStatefulSetDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getStatefulSetDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getStatefulSetDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getServiceImportDependencies(t *testing.T) {
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "serviceImport get dependencies",
			args: args{
				object: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": mcsv1alpha1.GroupVersion.String(),
						"kind":       util.ServiceImportKind,
						"metadata": map[string]any{
							"name":      "fake-serviceImport",
							"namespace": namespace,
						},
						"spec": map[string]any{
							"type": "ClusterSetIP",
							"ports": []any{
								map[string]any{
									"port":     80,
									"protocol": "TCP",
								},
							},
						},
					},
				},
			},
			want: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       util.ServiceKind,
					Namespace:  namespace,
					Name:       "derived-fake-serviceImport",
				},
				{
					APIVersion: "discovery.k8s.io/v1",
					Kind:       util.EndpointSliceKind,
					Namespace:  namespace,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							discoveryv1.LabelServiceName: "derived-fake-serviceImport",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getServiceImportDependencies(tt.args.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getServiceImportDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getServiceImportDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAllDefaultDependenciesInterpreter(t *testing.T) {
	expectedKinds := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
		{Group: "batch", Version: "v1", Kind: "Job"},
		{Group: "batch", Version: "v1", Kind: "CronJob"},
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
		{Group: "multicluster.x-k8s.io", Version: "v1alpha1", Kind: "ServiceImport"},
	}

	got := getAllDefaultDependenciesInterpreter()

	if len(got) != len(expectedKinds) {
		t.Errorf("getAllDefaultDependenciesInterpreter() length = %d, want %d", len(got), len(expectedKinds))
	}

	for _, key := range expectedKinds {
		_, exists := got[key]
		if !exists {
			t.Errorf("getAllDefaultDependenciesInterpreter() missing key %v", key)
		}
	}
}

func Test_getIngressDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "ingress with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
					"metadata": map[string]any{
						"name":      "test-ingress",
						"namespace": namespace,
					},
					"spec": map[string]any{
						"tls": []any{
							map[string]any{
								"hosts":      []any{"foo1.*.bar.com"},
								"secretName": "cloud-secret",
							},
							map[string]any{
								"hosts":      []any{"foo2.*.bar.com"},
								"secretName": "test-secret",
							},
						},
					},
				},
			},
			want: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "cloud-secret",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "test-secret",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getIngressDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("getIngressDependencies() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getIngressDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}
