package native

import (
	"reflect"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
				Object: map[string]interface{}{
					"serviceAccountName": "test-serviceaccount",
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "initcontainer-volume-mount",
							"configMap": map[string]interface{}{
								"name": "test-configmap",
							},
						},
						map[string]interface{}{
							"name": "cloud-secret-volume",
							"secret": map[string]interface{}{
								"secretName": "cloud-secret",
							},
						},
						map[string]interface{}{
							"name": "container-volume-mount",
							"persistentVolumeClaim": map[string]interface{}{
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
				Object: map[string]interface{}{
					"serviceAccountName": "test-serviceaccount",
					"imagePullSecrets": []interface{}{
						map[string]interface{}{
							"name": "test-secret",
						},
					},
					"initContainers": []interface{}{
						map[string]interface{}{
							"envFrom": []interface{}{
								map[string]interface{}{
									"configMapRef": map[string]interface{}{
										"name": "initcontainer-envfrom-configmap",
									},
									"secretRef": map[string]interface{}{
										"name": "test-secret",
									},
								},
							},
							"env": []interface{}{
								map[string]interface{}{
									"valueFrom": map[string]interface{}{
										"secretKeyRef": map[string]interface{}{
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
				Object: map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"envFrom": []interface{}{
								map[string]interface{}{
									"configMapRef": map[string]interface{}{
										"name": "test-configmap",
									},
								},
							},
							"env": []interface{}{
								map[string]interface{}{
									"valueFrom": map[string]interface{}{
										"configMapKeyRef": map[string]interface{}{
											"name": "test-configmap",
										},
									},
								},
							},
						},
					},
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "container-volume-mount",
							"persistentVolumeClaim": map[string]interface{}{
								"claimName": "test-pvc",
							},
							"azureFile": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
						"namespace":  namespace,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "fake-deployment",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "fake-deployment",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"parallelism": 1,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"parallelism": 1,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name":      "fake-job",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"parallelism": 1,
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1beta1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"schedule": "*/1 * * * *",
						"jobTemplate": map[string]interface{}{
							"spec": map[string]interface{}{
								"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "fake-pod",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "example-container",
								"image": "busybox",
								"command": []interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "batch/v1beta1",
					"kind":       "pod",
					"metadata": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"metadata": map[string]interface{}{
						"name":      "fake-daemonset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"serviceName": "fake-service",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"serviceName": "fake-service",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"serviceName": "fake-service",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]interface{}{
						"name":      "fake-statefulset",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"serviceName": "fake-service",
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "fake",
							},
						},
						"template": map[string]interface{}{
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
					Object: map[string]interface{}{
						"apiVersion": mcsv1alpha1.GroupVersion.String(),
						"kind":       util.ServiceImportKind,
						"metadata": map[string]interface{}{
							"name":      "fake-serviceImport",
							"namespace": namespace,
						},
						"spec": map[string]interface{}{
							"type": "ClusterSetIP",
							"ports": []interface{}{
								map[string]interface{}{
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
