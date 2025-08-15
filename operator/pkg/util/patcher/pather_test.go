/*
Copyright 2024 The Karmada Authors.

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

package patcher

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/pkg/features"
)

func init() {
	// Enable LabelPropagation feature for testing
	_ = features.FeatureGate.Set("LabelPropagation=true")
}

func TestPatchForDeployment(t *testing.T) {
	tests := []struct {
		name       string
		patcher    *Patcher
		deployment *appsv1.Deployment
		want       *appsv1.Deployment
	}{
		{
			name: "PatchForDeployment_WithLabelsAndAnnotations_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"label1": "value1-patched",
				},
				annotations: map[string]string{
					"annotation1": "annot1-patched",
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test",
					Labels: map[string]string{
						"label1": "value1",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"annotation1": "annot1",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test",
					Labels: map[string]string{
						"label1": "value1-patched",
					},
					Annotations: map[string]string{
						"annotation1": "annot1-patched",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"label1": "value1-patched",
							},
							Annotations: map[string]string{
								"annotation1": "annot1-patched",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "PatchForDeployment_WithResourcesExtraArgsAndFeatureGates_Patched",
			patcher: &Patcher{
				extraArgs: map[string]string{
					"some-arg": "some-value",
				},
				featureGates: map[string]bool{
					"SomeGate": true,
				},
				resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
									Command: []string{
										"/bin/bash",
										"--feature-gates=OldGate=false",
									},
								},
							},
						},
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
									Command: []string{
										"/bin/bash",
										"--feature-gates=OldGate=false,SomeGate=true",
										"--some-arg=some-value",
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "PatchForDeployment_WithExtraVolumesAndVolumeMounts_Patched",
			patcher: &Patcher{
				extraVolumes: []corev1.Volume{
					{
						Name: "extra-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				extraVolumeMounts: []corev1.VolumeMount{
					{
						Name:      "extra-volume",
						MountPath: "/extra/path",
					},
				},
			},
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "extra-volume",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "extra-volume",
											MountPath: "/extra/path",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForDeployment(test.deployment)
			if !reflect.DeepEqual(test.deployment, test.want) {
				t.Errorf("unexpected err, expected deployment %v but got %v", test.deployment, test.want)
			}
		})
	}
}

func TestPatchForStatefulSet(t *testing.T) {
	tests := []struct {
		name        string
		patcher     *Patcher
		statefulSet *appsv1.StatefulSet
		want        *appsv1.StatefulSet
	}{
		{
			name: "PatchForStatefulSet_WithLabelsAndAnnotations_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"label1": "value1-patched",
				},
				annotations: map[string]string{
					"annotation1": "annot1-patched",
				},
			},
			statefulSet: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "test",
					Labels: map[string]string{
						"label1": "value1",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "test",
					Labels: map[string]string{
						"label1": "value1-patched",
					},
					Annotations: map[string]string{
						"annotation1": "annot1-patched",
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"label1": "value1-patched",
							},
							Annotations: map[string]string{
								"annotation1": "annot1-patched",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "PatchForStatefulSet_WithVolumes_Patched",
			patcher: &Patcher{
				volume: &v1alpha1.VolumeData{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium:    corev1.StorageMediumMemory,
						SizeLimit: &resource.Quantity{},
					},
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp",
						Type: ptr.To(corev1.HostPathDirectory),
					},
					VolumeClaim: &corev1.PersistentVolumeClaimTemplate{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1024m"),
								},
							},
						},
					},
				},
			},
			statefulSet: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
							Volumes: []corev1.Volume{},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-statefulset",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: constants.EtcdDataVolumeName,
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
								{
									Name: constants.EtcdDataVolumeName,
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/tmp",
											Type: ptr.To(corev1.HostPathDirectory),
										},
									},
								},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: constants.EtcdDataVolumeName,
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("1024m"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "PatchForStatefulSet_WithResourcesAndExtraArgs_Patched",
			patcher: &Patcher{
				extraArgs: map[string]string{
					"some-arg": "some-value",
				},
				resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			statefulSet: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-statefulset",
					Namespace: "test",
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-statefulset",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
									Command: []string{
										"--some-arg=some-value",
									},
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForStatefulSet(test.statefulSet)
			if !reflect.DeepEqual(test.statefulSet, test.want) {
				t.Errorf("unexpected err, expected statefulset %v but got %v", test.statefulSet, test.want)
			}
		})
	}
}

func TestPatcherForService(t *testing.T) {
	tests := []struct {
		name    string
		patcher *Patcher
		service *corev1.Service
		want    *corev1.Service
	}{
		{
			name: "PatchForService_WithLabelsAndAnnotations_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"test": "value",
				},
				annotations: map[string]string{
					"test-annotation": "test-value",
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
						"test":     "value",
					},
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
				},
			},
		},
		{
			name: "PatchForService_WithNilLabels_NoChange",
			patcher: &Patcher{
				labels: nil,
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForService(test.service)
			if !reflect.DeepEqual(test.service, test.want) {
				t.Errorf("unexpected result, expected service %v but got %v", test.want, test.service)
			}
		})
	}
}

func TestPatcherForSecret(t *testing.T) {
	tests := []struct {
		name    string
		patcher *Patcher
		secret  *corev1.Secret
		want    *corev1.Secret
	}{
		{
			name: "PatchForSecret_WithLabels_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"test": "value",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
						"test":     "value",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForSecret(test.secret)
			if !reflect.DeepEqual(test.secret, test.want) {
				t.Errorf("unexpected result, expected secret %v but got %v", test.want, test.secret)
			}
		})
	}
}

func TestPatcherForClusterRole(t *testing.T) {
	tests := []struct {
		name    string
		patcher *Patcher
		role    *rbacv1.ClusterRole
		want    *rbacv1.ClusterRole
	}{
		{
			name: "PatchForClusterRole_WithLabels_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"test": "value",
				},
			},
			role: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-role",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-role",
					Labels: map[string]string{
						"existing": "label",
						"test":     "value",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForClusterRole(test.role)
			if !reflect.DeepEqual(test.role, test.want) {
				t.Errorf("unexpected result, expected role %v but got %v", test.want, test.role)
			}
		})
	}
}

func TestPatcherForClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name        string
		patcher     *Patcher
		roleBinding *rbacv1.ClusterRoleBinding
		want        *rbacv1.ClusterRoleBinding
	}{
		{
			name: "PatchForClusterRoleBinding_WithLabels_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"test": "value",
				},
			},
			roleBinding: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rolebinding",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rolebinding",
					Labels: map[string]string{
						"existing": "label",
						"test":     "value",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForClusterRoleBinding(test.roleBinding)
			if !reflect.DeepEqual(test.roleBinding, test.want) {
				t.Errorf("unexpected result, expected roleBinding %v but got %v", test.want, test.roleBinding)
			}
		})
	}
}

func TestPatcherForGenericResource(t *testing.T) {
	tests := []struct {
		name    string
		patcher *Patcher
		obj     metav1.Object
		want    metav1.Object
	}{
		{
			name: "PatchForGenericResource_WithLabels_Patched",
			patcher: &Patcher{
				labels: map[string]string{
					"test": "value",
				},
			},
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
						"test":     "value",
					},
				},
			},
		},
		{
			name: "PatchForGenericResource_WithAnnotations_Patched",
			patcher: &Patcher{
				annotations: map[string]string{
					"test-annotation": "test-value",
				},
			},
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "test",
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "test",
					Annotations: map[string]string{
						"existing":        "annotation",
						"test-annotation": "test-value",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForGenericResource(test.obj)
			if !reflect.DeepEqual(test.obj, test.want) {
				t.Errorf("unexpected result, expected object %v but got %v", test.want, test.obj)
			}
		})
	}
}

func TestPatcherWithNilValues(t *testing.T) {
	tests := []struct {
		name    string
		patcher *Patcher
		obj     metav1.Object
		want    metav1.Object
	}{
		{
			name: "PatchWithNilLabels_NoChange",
			patcher: &Patcher{
				labels: nil,
			},
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Labels: map[string]string{
						"existing": "label",
					},
				},
			},
		},
		{
			name: "PatchWithNilAnnotations_NoChange",
			patcher: &Patcher{
				annotations: nil,
			},
			obj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test",
					Annotations: map[string]string{
						"existing": "annotation",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.patcher.ForGenericResource(test.obj)
			if !reflect.DeepEqual(test.obj, test.want) {
				t.Errorf("unexpected result, expected object %v but got %v", test.want, test.obj)
			}
		})
	}
}

// Simple test to verify basic functionality
func TestBasicPatcherFunctionality(t *testing.T) {
	patcher := NewPatcher()

	// Test WithLabels
	labels := map[string]string{"test": "value"}
	patcher = patcher.WithLabels(labels)
	if patcher.labels["test"] != "value" {
		t.Errorf("expected label 'test' to be 'value', got %s", patcher.labels["test"])
	}

	// Test WithAnnotations
	annotations := map[string]string{"test-annotation": "test-value"}
	patcher = patcher.WithAnnotations(annotations)
	if patcher.annotations["test-annotation"] != "test-value" {
		t.Errorf("expected annotation 'test-annotation' to be 'test-value', got %s", patcher.annotations["test-annotation"])
	}
}
