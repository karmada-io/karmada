package native

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
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
						"availableReplicas":  3,
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
						"availableReplicas":  3,
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
		{
			name: "availableReplicas not equal to updatedReplicas",
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
						"availableReplicas":  0,
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
						"availableReplicas":  3,
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
						"availableReplicas":  3,
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
		{
			name: "availableReplicas not equal to updatedReplicas",
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
						"availableReplicas":  1,
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func Test_interpretReplicaSetHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "replicaSet healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas":  3,
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
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas":  3,
						"observedGeneration": 2,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "replicas not equal to availableReplicas",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas":  2,
						"observedGeneration": 2,
					},
				},
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretReplicaSetHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretReplicaSetHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretReplicaSetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretDaemonSetHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "daemonSet healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"status": map[string]interface{}{
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
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"status": map[string]interface{}{
						"observedGeneration": 2,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "updatedNumberScheduled < desiredNumberScheduled",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"updatedNumberScheduled": 3,
						"desiredNumberScheduled": 5,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "numberAvailable < updatedNumberScheduled",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"updatedNumberScheduled": 5,
						"numberAvailable":        3,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretDaemonSetHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretDaemonSetHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretDaemonSetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretServiceHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "service healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": corev1.ServiceTypeLoadBalancer,
					},
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []map[string]interface{}{{"ip": "localhost"}},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not loadBalancer type",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": corev1.ServiceTypeNodePort,
					},
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []map[string]interface{}{{"ip": "localhost"}},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "status.loadBalancer.ingress list is empty",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"type": corev1.ServiceTypeLoadBalancer,
					},
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []map[string]interface{}{},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretServiceHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretServiceHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretServiceHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretIngressHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "ingress healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []map[string]interface{}{{"ip": "localhost"}},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "status.loadBalancer.ingress list is empty",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"loadBalancer": map[string]interface{}{
							"ingress": []map[string]interface{}{},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretIngressHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretIngressHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretIngressHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretPersistentVolumeClaimHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "persistentVolumeClaim healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": corev1.ClaimBound,
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "status.phase not equals to Bound",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": corev1.ClaimPending,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretPersistentVolumeClaimHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretPersistentVolumeClaimHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretPersistentVolumeClaimHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpretPodHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "service type pod healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "fake-pod",
					},
					"status": map[string]interface{}{
						"conditions": []map[string]string{
							{
								"type":   "Ready",
								"status": "True",
							},
						},
						"phase": "Running",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "job type pod healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "fake-pod",
					},
					"status": map[string]interface{}{
						"phase": "Succeeded",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "pod condition ready false",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "fake-pod",
					},
					"status": map[string]interface{}{
						"conditions": []map[string]string{
							{
								"type":   "Ready",
								"status": "Unknown",
							},
						},
						"phase": "Running",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pod phase not running and not succeeded",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "fake-pod",
					},
					"status": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "condition or phase nil",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "fake-pod",
					},
				},
			},
			want:    false,
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := interpretPodHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("interpretPodHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("interpretPodHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_interpretPodDisruptionBudgetHealth(t *testing.T) {
	tests := []struct {
		name   string
		object *unstructured.Unstructured
		want   bool
	}{
		{
			name: "podDisruptionBudget healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"desiredHealthy": 2,
						"currentHealthy": 3,
					},
				},
			},
			want: true,
		},
		{
			name: "podDisruptionBudget healthy when desired healthy equals to current healthy pods",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"desiredHealthy": 2,
						"currentHealthy": 2,
					},
				},
			},
			want: true,
		},
		{
			name: "podDisruptionBudget does not allow further disruption when number of healthy pods is less than desired",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"desiredHealthy": 2,
						"currentHealthy": 1,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := interpretPodDisruptionBudgetHealth(tt.object)
			if got != tt.want {
				t.Errorf("interpretPodDisruptionBudgetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
