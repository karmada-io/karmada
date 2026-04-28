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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

var (
	namespace = "karmada-test"
	podName   = "fake-pod"
)

func TestGetPodCondition(t *testing.T) {
	type args struct {
		status        *corev1.PodStatus
		conditionType corev1.PodConditionType
	}
	tests := []struct {
		name          string
		args          args
		wantedOrder   int
		wantCondition *corev1.PodCondition
	}{
		{
			name: "empty status",
			args: args{
				status:        nil,
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
		{
			name: "empty condition",
			args: args{
				status:        &corev1.PodStatus{Conditions: nil},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
		{
			name: "doesn't have objective condition",
			args: args{
				status: &corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type: corev1.PodInitialized,
						},
					},
				},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
		{
			name: "have objective condition",
			args: args{
				status: &corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type: corev1.PodInitialized,
						},
						{
							Type: corev1.ContainersReady,
						},
					},
				},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder: 1,
			wantCondition: &corev1.PodCondition{
				Type: corev1.ContainersReady,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetPodCondition(tt.args.status, tt.args.conditionType)
			if got != tt.wantedOrder {
				t.Errorf("GetPodCondition() got = %v, wantedOrder %v", got, tt.wantedOrder)
			}
			if !reflect.DeepEqual(got1, tt.wantCondition) {
				t.Errorf("GetPodCondition() got1 = %v, wantedOrder %v", got1, tt.wantCondition)
			}
		})
	}
}

func TestGetPodConditionFromList(t *testing.T) {
	type args struct {
		conditions    []corev1.PodCondition
		conditionType corev1.PodConditionType
	}
	var tests = []struct {
		name          string
		args          args
		wantedOrder   int
		wantCondition *corev1.PodCondition
	}{
		{
			name: "empty condition",
			args: args{
				conditions:    nil,
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
		{
			name: "doesn't have objective condition",
			args: args{
				conditions: []corev1.PodCondition{
					{
						Type: corev1.PodInitialized,
					},
				},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
		{
			name: "have objective condition",
			args: args{
				conditions: []corev1.PodCondition{
					{
						Type: corev1.PodInitialized,
					},
					{
						Type: corev1.ContainersReady,
					},
				},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder: 1,
			wantCondition: &corev1.PodCondition{
				Type: corev1.ContainersReady,
			},
		},
		{
			name: "PodReady condition",
			args: args{
				conditions: []corev1.PodCondition{
					{
						Type: corev1.PodInitialized,
					},
					{
						Type: corev1.PodReady,
					},
				},
				conditionType: corev1.ContainersReady,
			},
			wantedOrder:   -1,
			wantCondition: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetPodConditionFromList(tt.args.conditions, tt.args.conditionType)
			if got != tt.wantedOrder {
				t.Errorf("GetPodConditionFromList() got = %v, wantedOrder %v", got, tt.wantedOrder)
			}
			if !reflect.DeepEqual(got1, tt.wantCondition) {
				t.Errorf("GetPodConditionFromList() got1 = %v, wantedOrder %v", got1, tt.wantCondition)
			}
		})
	}
}

func TestGeneratePodFromTemplateAndNamespace(t *testing.T) {
	pod1 := helper.NewPod(namespace, podName)
	pod2 := helper.NewPod(namespace, podName)
	pod2.Spec.ServiceAccountName = "fake-sa"
	type args struct {
		template  *corev1.PodTemplateSpec
		namespace string
	}
	tests := []struct {
		name     string
		args     args
		expected corev1.Pod
	}{
		{
			name: "generate a simple pod from template and namespace without dependency",
			args: args{
				template:  &corev1.PodTemplateSpec{Spec: pod1.Spec},
				namespace: "test1",
			},
			expected: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test1"},
				Spec:       *pod1.Spec.DeepCopy(),
			},
		},
		{
			name: "generate a simple pod from template and namespace",
			args: args{
				template:  &corev1.PodTemplateSpec{Spec: pod2.Spec},
				namespace: "test2",
			},
			expected: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test2"},
				Spec:       *pod2.Spec.DeepCopy(),
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GeneratePodFromTemplateAndNamespace(tt.args.template, tt.args.namespace); !reflect.DeepEqual(*got, tt.expected) {
				t.Errorf("GeneratePodFromTemplateAndNamespace() = %v,\n want %v", got, tt.expected)
			}
		})
	}
}

func TestGetSecretNames(t *testing.T) {
	emptySecretsPod := helper.NewPod(namespace, podName)

	secretsPod := helper.NewPod(namespace, podName)
	secretsPod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: "image-secret",
		},
	}
	secretsPod.Spec.InitContainers = []corev1.Container{
		{
			EnvFrom: []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "initcontainer-envfrom-secret",
						},
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "initcontainer-env-secret",
							},
						},
					},
				},
			},
		},
	}
	secretsPod.Spec.Containers = []corev1.Container{
		{
			EnvFrom: []corev1.EnvFromSource{
				{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "container-envfrom-secret",
						},
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "container-env-secret",
							},
						},
					},
				},
			},
		},
	}
	secretsPod.Spec.EphemeralContainers = []corev1.EphemeralContainer{
		{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				EnvFrom: []corev1.EnvFromSource{
					{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ephemeralcontainer-envfrom-secret",
							},
						},
					},
				},
				Env: []corev1.EnvVar{
					{
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ephemeralcontainer-env-secret",
								},
							},
						},
					},
				},
			},
		},
	}
	secretsPod.Spec.Volumes = []corev1.Volume{
		{
			VolumeSource: corev1.VolumeSource{
				AzureFile: &corev1.AzureFileVolumeSource{
					SecretName: "azure-secret",
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				CephFS: &corev1.CephFSVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "cephfs-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				Cinder: &corev1.CinderVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "cinder-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				FlexVolume: &corev1.FlexVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "flex-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "projected-secret",
								},
							},
						},
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				RBD: &corev1.RBDVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "rbd-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "secret-config",
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				ScaleIO: &corev1.ScaleIOVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "scaleio-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				ISCSI: &corev1.ISCSIVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "iscsi-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				StorageOS: &corev1.StorageOSVolumeSource{
					SecretRef: &corev1.LocalObjectReference{
						Name: "storageos-secret",
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					NodePublishSecretRef: &corev1.LocalObjectReference{
						Name: "csi-secret",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected sets.Set[string]
	}{
		{
			name:     "get secret names from pod without secrets",
			pod:      emptySecretsPod,
			expected: make(sets.Set[string], 0),
		},
		{
			name: "get secret names from pod with secrets",
			pod:  secretsPod,
			expected: sets.New("image-secret", "initcontainer-envfrom-secret", "initcontainer-env-secret", "container-envfrom-secret", "container-env-secret",
				"ephemeralcontainer-envfrom-secret", "ephemeralcontainer-env-secret", "azure-secret", "cephfs-secret", "cinder-secret", "flex-secret", "projected-secret",
				"rbd-secret", "secret-config", "scaleio-secret", "iscsi-secret", "storageos-secret", "csi-secret"),
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := getSecretNames(tt.pod)
			if !res.Equal(tt.expected) {
				t.Errorf("getSecretNames() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGetConfigMapNames(t *testing.T) {
	emptyConfigMapsPod := helper.NewPod(namespace, podName)

	configMapsPod := helper.NewPod(namespace, podName)
	configMapsPod.Spec.InitContainers = []corev1.Container{
		{
			EnvFrom: []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "initcontainer-envfrom-configmap",
						},
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "initcontainer-env-configmap",
							},
						},
					},
				},
			},
		},
	}
	configMapsPod.Spec.Containers = []corev1.Container{
		{
			EnvFrom: []corev1.EnvFromSource{
				{
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "container-envfrom-configmap",
						},
					},
				},
			},
			Env: []corev1.EnvVar{
				{
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "container-env-configmap",
							},
						},
					},
				},
			},
		},
	}
	configMapsPod.Spec.EphemeralContainers = []corev1.EphemeralContainer{
		{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				EnvFrom: []corev1.EnvFromSource{
					{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ephemeralcontainer-envfrom-configmap",
							},
						},
					},
				},
				Env: []corev1.EnvVar{
					{
						ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "ephemeralcontainer-env-configmap",
								},
							},
						},
					},
				},
			},
		},
	}
	configMapsPod.Spec.Volumes = []corev1.Volume{
		{
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "projected-configmap",
								},
							},
						},
					},
				},
			},
		},
		{
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "configmap-config",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected sets.Set[string]
	}{
		{
			name:     "get configMap names from pod without configMaps",
			pod:      emptyConfigMapsPod,
			expected: make(sets.Set[string], 0),
		},
		{
			name: "get configMap names from pod with configMaps",
			pod:  configMapsPod,
			expected: sets.New("initcontainer-envfrom-configmap", "initcontainer-env-configmap", "container-envfrom-configmap", "container-env-configmap",
				"ephemeralcontainer-envfrom-configmap", "ephemeralcontainer-env-configmap", "projected-configmap", "configmap-config"),
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := getConfigMapNames(tt.pod)
			if !res.Equal(tt.expected) {
				t.Errorf("getConfigMapNames() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGetPVCNames(t *testing.T) {
	emptyPvcsPod := helper.NewPod(namespace, podName)
	pvcsPod := helper.NewPod(namespace, podName)
	pvcsPod.Spec.Volumes = []corev1.Volume{
		{
			Name: "foo-name",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "fake-foo",
					ReadOnly:  true,
				},
			},
		},
		{
			Name: "bar-name",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "fake-bar",
					ReadOnly:  true,
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected sets.Set[string]
	}{
		{
			name:     "get pvc names from pod without pvcs",
			pod:      emptyPvcsPod,
			expected: make(sets.Set[string], 0),
		},
		{
			name:     "get pvc names from pod with pvcs",
			pod:      pvcsPod,
			expected: sets.New("fake-foo", "fake-bar"),
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := getPVCNames(tt.pod)
			if !res.Equal(tt.expected) {
				t.Errorf("getPVCNames() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGetDependenciesFromPodTemplate(t *testing.T) {
	emptyDependenciesPod := helper.NewPod(namespace, podName)

	dependenciesPod := helper.NewPod(namespace, podName)
	dependenciesPod.Spec.Volumes = []corev1.Volume{
		{
			Name: "foo-name",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "fake-foo",
					},
				},
			},
		},
		{
			Name: "bar-name",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "fake-bar",
				},
			},
		},
		{
			Name: "pvc-name",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "fake-pvc",
				},
			},
		},
	}
	dependenciesPod.Spec.ServiceAccountName = "fake-sa"

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected []configv1alpha1.DependentObjectReference
	}{
		{
			name:     "get dependencies from PodTemplate without dependencies",
			pod:      emptyDependenciesPod,
			expected: nil,
		},
		{
			name: "get dependencies from PodTemplate with dependencies",
			pod:  dependenciesPod,
			expected: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  namespace,
					Name:       "fake-foo",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "fake-bar",
				},
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Namespace:  namespace,
					Name:       "fake-sa",
				},
				{
					APIVersion: "v1",
					Kind:       "PersistentVolumeClaim",
					Namespace:  namespace,
					Name:       "fake-pvc",
				},
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, _ := GetDependenciesFromPodTemplate(tt.pod)
			if len(res) == 0 {
				res = nil
			}
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("getDependenciesFromPodTemplate() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func Test_getServiceAccountNames(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want sets.Set[string]
	}{
		{
			name: "get ServiceAccountName from pod ",
			args: args{pod: &corev1.Pod{Spec: corev1.PodSpec{ServiceAccountName: "test"}}},
			want: sets.New("test"),
		},
		{
			name: "get default ServiceAccountName from pod ",
			args: args{pod: &corev1.Pod{Spec: corev1.PodSpec{ServiceAccountName: "default"}}},
			want: sets.New[string](),
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getServiceAccountNames(tt.args.pod); !got.Equal(tt.want) {
				t.Errorf("getServiceAccountNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
