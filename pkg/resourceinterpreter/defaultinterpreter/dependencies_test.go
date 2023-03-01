package defaultinterpreter

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

var (
	namespace = "karmada-test"
	podName   = "fake-pod"
)

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
			res, _ := getDependenciesFromPodTemplate(tt.pod)
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
