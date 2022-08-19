package defaultinterpreter

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
			name: "replicas not equal to availableReplicas",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas": 2,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			name: "updatedNumberScheduled < desiredNumberSchedulerd",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
