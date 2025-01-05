/*
Copyright 2023 The Karmada Authors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_retainK8sWorkloadReplicas(t *testing.T) {
	desiredNum, observedNum := int32(2), int32(4)
	type args struct {
		desired  interface{}
		observed interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "deployment is control by hpa",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
						Labels: map[string]string{
							util.RetainReplicasLabel: util.RetainReplicasValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &desiredNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &observedNum,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
					Labels: map[string]string{
						util.RetainReplicasLabel: util.RetainReplicasValue,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &observedNum,
				},
			},
			wantErr: false,
		},
		{
			name: "deployment is not control by hpa",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &desiredNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &observedNum,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &desiredNum,
				},
			},
			wantErr: false,
		},
		{
			name: "empty observed replicas",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
						Labels: map[string]string{
							util.RetainReplicasLabel: util.RetainReplicasValue,
						},
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: &desiredNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstructuredDesiredObj, err := helper.ToUnstructured(tt.args.desired)
			if err != nil {
				klog.Errorf("Failed to transform desired, error: %v", err)
				return
			}

			unstructuredObservedObj, err := helper.ToUnstructured(tt.args.observed)
			if err != nil {
				klog.Errorf("Failed to transform observed, error: %v", err)
				return
			}
			got, err := retainWorkloadReplicas(unstructuredDesiredObj, unstructuredObservedObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("retainWorkloadReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			unstructuredWantObj := (*unstructured.Unstructured)(nil)
			if tt.want != nil {
				unstructuredWantObj, err = helper.ToUnstructured(tt.want)
				if err != nil {
					klog.Errorf("Failed to transform want, error: %v", err)
					return
				}
			}
			assert.Equalf(t, unstructuredWantObj, got, "retainWorkloadReplicas(%v, %v)", unstructuredDesiredObj, unstructuredObservedObj)
		})
	}
}

func Test_retainSecretServiceAccountToken(t *testing.T) {
	createSecret := func(secretType corev1.SecretType, dataKey, dataValue string) *unstructured.Unstructured {
		ret, _ := helper.ToUnstructured(&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{},
			Data:       map[string][]byte{dataKey: []byte(dataValue)},
			Type:       secretType,
		})
		return ret
	}

	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want *unstructured.Unstructured
	}{
		{
			name: "secret data and uid are retained for type service-account-token",
			args: args{
				desired:  createSecret(corev1.SecretTypeServiceAccountToken, corev1.ServiceAccountTokenKey, "desired-token"),
				observed: createSecret(corev1.SecretTypeServiceAccountToken, corev1.ServiceAccountTokenKey, "observed-token"),
			},
			want: createSecret(corev1.SecretTypeServiceAccountToken, corev1.ServiceAccountTokenKey, "observed-token"),
		},
		{
			name: "ignores missing uid and data for type service-account-token",
			args: args{
				desired:  &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
				observed: &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{"type": string(corev1.SecretTypeServiceAccountToken)}},
		},
		{
			name: "does not retain for type tls",
			args: args{
				desired:  createSecret(corev1.SecretTypeTLS, corev1.TLSCertKey, "desired-cert"),
				observed: createSecret(corev1.SecretTypeTLS, corev1.TLSCertKey, "observed-cert"),
			},
			want: createSecret(corev1.SecretTypeTLS, corev1.TLSCertKey, "desired-cert"),
		},
		{
			name: "does not retain for type basic-auth",
			args: args{
				desired:  createSecret(corev1.SecretTypeBasicAuth, corev1.BasicAuthUsernameKey, "desired-user"),
				observed: createSecret(corev1.SecretTypeBasicAuth, corev1.BasicAuthUsernameKey, "observed-user"),
			},
			want: createSecret(corev1.SecretTypeBasicAuth, corev1.BasicAuthUsernameKey, "desired-user"),
		},
		{
			name: "does not retain for type dockercfg",
			args: args{
				desired:  createSecret(corev1.SecretTypeDockercfg, corev1.DockerConfigKey, "desired-docker-cfg"),
				observed: createSecret(corev1.SecretTypeDockercfg, corev1.DockerConfigKey, "observed-docker-cfg"),
			},
			want: createSecret(corev1.SecretTypeDockercfg, corev1.DockerConfigKey, "desired-docker-cfg"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainSecretServiceAccountToken(tt.args.desired, tt.args.observed)
			assert.Nil(t, err, "retainSecretServiceAccountToken() error = %v", err)
			assert.Equalf(t, tt.want, got, "retainSecretServiceAccountToken(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}

func Test_retainPersistentVolumeFields(t *testing.T) {
	createPV := func(claimRef *corev1.ObjectReference) *unstructured.Unstructured {
		ret, _ := helper.ToUnstructured(&corev1.PersistentVolume{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolume",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv",
			},
			Spec: corev1.PersistentVolumeSpec{
				ClaimRef: claimRef,
			},
		})
		return ret
	}
	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want *unstructured.Unstructured
	}{
		{
			name: "retain claimRef",
			args: args{
				desired:  createPV(&corev1.ObjectReference{Name: "pvc-1", Namespace: "default"}),
				observed: createPV(&corev1.ObjectReference{Name: "pvc-2", Namespace: "default"}),
			},
			want: createPV(&corev1.ObjectReference{Name: "pvc-2", Namespace: "default"}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainPersistentVolumeFields(tt.args.desired, tt.args.observed)
			assert.Nil(t, err, "retainPersistentVolumeFields() error = %v", err)
			assert.Equalf(t, tt.want, got, "retainPersistentVolumeFields(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}

func Test_retainPersistentVolumeClaimFields(t *testing.T) {
	createPVC := func(volumeName string) *unstructured.Unstructured {
		ret, _ := helper.ToUnstructured(&corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeName: volumeName,
			},
		})
		return ret
	}
	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want *unstructured.Unstructured
	}{
		{
			name: "retain observed volume name",
			args: args{
				desired:  createPVC("desired-volume-name"),
				observed: createPVC("observed-volume-name"),
			},
			want: createPVC("observed-volume-name"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainPersistentVolumeClaimFields(tt.args.desired, tt.args.observed)
			assert.Nil(t, err, "retainPersistentVolumeClaimFields() error = %v", err)
			assert.Equalf(t, tt.want, got, "retainPersistentVolumeClaimFields(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}

func Test_retainJobSelectorFields(t *testing.T) {
	replicaNum := int32(2)
	type args struct {
		desired  interface{}
		observed interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "matchLabels and templateLabels exists",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nginx_v1"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "nginx_v1"},
							},
						},
						Replicas: &replicaNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"name": "nginx"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"name": "nginx"},
							},
						},
						Replicas: &replicaNum,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"name": "nginx"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"name": "nginx"},
						},
					},
					Replicas: &replicaNum,
				},
			},
			wantErr: false,
		},
		{
			name: "matchLabels does not exists",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "nginx_v1"},
							},
						},
						Replicas: &replicaNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"name": "nginx"},
							},
						},
						Replicas: &replicaNum,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"name": "nginx"},
						},
					},
					Replicas: &replicaNum,
				},
			},
			wantErr: false,
		},
		{
			name: "templateLabels does not exists",
			args: args{
				desired: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "nginx_v1"},
						},
						Replicas: &replicaNum,
					},
				},
				observed: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nginx",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"name": "nginx"},
						},
						Replicas: &replicaNum,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"name": "nginx"},
					},
					Replicas: &replicaNum,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstructuredDesiredObj, err := helper.ToUnstructured(tt.args.desired)
			if err != nil {
				klog.Errorf("Failed to transform desired, error: %v", err)
				return
			}
			unstructuredObservedObj, err := helper.ToUnstructured(tt.args.observed)
			if err != nil {
				klog.Errorf("Failed to transform observed, error: %v", err)
				return
			}
			got, err := retainJobSelectorFields(unstructuredDesiredObj, unstructuredObservedObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("retainJobSelectorFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			unstructuredWantObj, err := helper.ToUnstructured(tt.want)
			if err != nil {
				klog.Errorf("Failed to transform want, error: %v", err)
				return
			}
			assert.Equalf(t, unstructuredWantObj, got, "retainJobSelectorFields(%v, %v)", unstructuredDesiredObj, unstructuredObservedObj)
		})
	}
}

func Test_retainPodFields(t *testing.T) {
	type args struct {
		desired  interface{}
		observed interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "retain observed pod fields",
			args: args{
				desired: &corev1.Pod{Spec: corev1.PodSpec{
					NodeName:           "node2",
					ServiceAccountName: "fake-desired-sa",
					Volumes: []corev1.Volume{
						{
							Name: "fake-desired-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "fake-desired-secret",
								},
							},
						},
					},
					InitContainers: []corev1.Container{{
						Name: "test",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-desired-config",
								ReadOnly:  false,
								MountPath: "/etc/etcd/desired",
							},
						},
					}, {
						Name: "test-3",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-desired-config-2",
								ReadOnly:  false,
								MountPath: "/etc/etcd/desired2",
							},
						},
					}},
					Containers: []corev1.Container{{
						Name: "test",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-desired-cert",
								ReadOnly:  true,
								MountPath: "/etc/karmada/desired",
							},
						},
					}, {
						Name: "test-3",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-desired-cert-2",
								ReadOnly:  true,
								MountPath: "/etc/karmada/desired2",
							},
						},
					}},
				}},
				observed: &corev1.Pod{Spec: corev1.PodSpec{
					NodeName:           "node1",
					ServiceAccountName: "fake-observed-sa",
					Volumes: []corev1.Volume{
						{
							Name: "fake-observed-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "fake-observed-secret",
								},
							},
						},
					},
					InitContainers: []corev1.Container{{
						Name: "test",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-observed-config",
								ReadOnly:  false,
								MountPath: "/etc/etcd/observed",
							},
						},
					}, {
						Name: "test-2",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-observed-config-2",
								ReadOnly:  false,
								MountPath: "/etc/etcd/observed2",
							},
						},
					}},
					Containers: []corev1.Container{{
						Name: "test",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-observed-cert",
								ReadOnly:  true,
								MountPath: "/etc/karmada/observed",
							},
						},
					}, {
						Name: "test-2",
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "fake-observed-cert-2",
								ReadOnly:  true,
								MountPath: "/etc/karmada/observed2",
							},
						},
					}},
				}},
			},
			want: &corev1.Pod{Spec: corev1.PodSpec{
				NodeName:           "node1",
				ServiceAccountName: "fake-observed-sa",
				Volumes: []corev1.Volume{
					{
						Name: "fake-observed-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "fake-observed-secret",
							},
						},
					},
				},
				InitContainers: []corev1.Container{{
					Name: "test",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-observed-config",
							ReadOnly:  false,
							MountPath: "/etc/etcd/observed",
						},
					},
				}, {
					Name: "test-3",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-desired-config-2",
							ReadOnly:  false,
							MountPath: "/etc/etcd/desired2",
						},
					},
				}},
				Containers: []corev1.Container{{
					Name: "test",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-observed-cert",
							ReadOnly:  true,
							MountPath: "/etc/karmada/observed",
						},
					},
				}, {
					Name: "test-3",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "fake-desired-cert-2",
							ReadOnly:  true,
							MountPath: "/etc/karmada/desired2",
						},
					},
				}},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstructuredDesiredObj, err := helper.ToUnstructured(tt.args.desired)
			if err != nil {
				klog.Errorf("Failed to transform desired, error: %v", err)
				return
			}
			unstructuredObservedObj, err := helper.ToUnstructured(tt.args.observed)
			if err != nil {
				klog.Errorf("Failed to transform observed, error: %v", err)
				return
			}
			got, err := retainPodFields(unstructuredDesiredObj, unstructuredObservedObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("retainPodFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			unstructuredWantObj, err := helper.ToUnstructured(tt.want)
			if err != nil {
				klog.Errorf("Failed to transform want, error: %v", err)
				return
			}
			assert.Equalf(t, unstructuredWantObj, got, "retainPodFields(%v, %v)", unstructuredDesiredObj, unstructuredObservedObj)
		})
	}
}

func Test_getAllDefaultRetentionInterpreter(t *testing.T) {
	expectedKinds := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "", Version: "v1", Kind: "Pod"},
		{Group: "", Version: "v1", Kind: "Service"},
		{Group: "", Version: "v1", Kind: "ServiceAccount"},
		{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"},
		{Group: "", Version: "v1", Kind: "PersistentVolume"},
		{Group: "batch", Version: "v1", Kind: "Job"},
		{Group: "", Version: "v1", Kind: "Secret"},
	}

	got := getAllDefaultRetentionInterpreter()

	if len(got) != len(expectedKinds) {
		t.Errorf("getAllDefaultRetentionInterpreter() length = %d, want %d", len(got), len(expectedKinds))
	}

	for _, key := range expectedKinds {
		if _, exists := got[key]; !exists {
			t.Errorf("getAllDefaultRetentionInterpreter() missing key %v", key)
		}
	}
}
