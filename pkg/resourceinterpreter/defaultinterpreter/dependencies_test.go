package defaultinterpreter

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/test/helper"
)

func TestGetSecretNames(t *testing.T) {
	fakePod := helper.NewPod("foo", "bar")
	fakePod.Spec.Volumes = []corev1.Volume{
		{
			Name: "foo-name",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "fake-foo",
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
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected sets.String
	}{
		{
			name:     "get secret names from pod",
			pod:      fakePod,
			expected: sets.NewString("fake-foo", "fake-bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := getSecretNames(tt.pod)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("getSecretNames() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGetConfigMapNames(t *testing.T) {
	fakePod := helper.NewPod("foo", "bar")
	fakePod.Spec.Volumes = []corev1.Volume{
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
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "fake-bar",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected sets.String
	}{
		{
			name:     "get configMap names from pod",
			pod:      fakePod,
			expected: sets.NewString("fake-foo", "fake-bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := getConfigMapNames(tt.pod)
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("getConfigMapNames() = %v, want %v", res, tt.expected)
			}
		})
	}
}

func TestGetDependenciesFromPodTemplate(t *testing.T) {
	fakePod := helper.NewPod("foo", "bar")
	fakePod.Spec.Volumes = []corev1.Volume{
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
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected []configv1alpha1.DependentObjectReference
	}{
		{
			name: "get dependencies from PodTemplate",
			pod:  fakePod,
			expected: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  "foo",
					Name:       "fake-foo",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "foo",
					Name:       "fake-bar",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := getDependenciesFromPodTemplate(tt.pod)
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
		want sets.String
	}{
		{
			name: "get ServiceAccountName from pod ",
			args: args{pod: &corev1.Pod{Spec: corev1.PodSpec{ServiceAccountName: "test"}}},
			want: sets.NewString("test"),
		},
		{
			name: "get default ServiceAccountName from pod ",
			args: args{pod: &corev1.Pod{Spec: corev1.PodSpec{ServiceAccountName: "default"}}},
			want: sets.NewString(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getServiceAccountNames(tt.args.pod); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getServiceAccountNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
