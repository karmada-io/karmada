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

package eventfilter

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestSpecificationChanged(t *testing.T) {
	tests := []struct {
		name       string
		oldObj     interface{}
		newObj     interface{}
		wantChange bool
	}{
		{
			name: "Only user defined fields changed(Deployment)",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx_v1"},
					},
				},
				Status: appsv1.DeploymentStatus{},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx_v2"},
					},
				},
				Status: appsv1.DeploymentStatus{},
			},
			wantChange: true,
		},
		{
			name: "Only status fields changed(Deployment)",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec:   appsv1.DeploymentSpec{},
				Status: appsv1.DeploymentStatus{Replicas: 3, UpdatedReplicas: 3, ReadyReplicas: 0},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123498",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec:   appsv1.DeploymentSpec{},
				Status: appsv1.DeploymentStatus{Replicas: 3, UpdatedReplicas: 1, ReadyReplicas: 2},
			},
			wantChange: false,
		},
		{
			name: "Both user defined fields and status fields changed(Deployment)",
			oldObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx_v1"},
					},
				},
				Status: appsv1.DeploymentStatus{Replicas: 3, UpdatedReplicas: 3, ReadyReplicas: 0},
			},
			newObj: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123498",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "nginx_v2"},
					},
				},
				Status: appsv1.DeploymentStatus{Replicas: 3, UpdatedReplicas: 1, ReadyReplicas: 2},
			},
			wantChange: true,
		},
		{
			name: "Only user defined fields changed(CRD_WorkLoad)",
			oldObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 3},
			},
			newObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](5),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 3},
			},
			wantChange: true,
		},
		{
			name: "Only status fields changed(CRD_WorkLoad)",
			oldObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 1},
			},
			newObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123498",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 3},
			},
			wantChange: false,
		},
		{
			name: "Both user defined fields and status fields changed(CRD_WorkLoad)",
			oldObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](3),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 1},
			},
			newObj: &workloadv1alpha1.Workload{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123498",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec: workloadv1alpha1.WorkloadSpec{
					Replicas: ptr.To[int32](5),
					Template: corev1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 3},
			},
			wantChange: true,
		},
		{
			name: "No change in Service",
			oldObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 80}},
				},
			},
			newObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 80}},
				},
			},
			wantChange: false,
		},
		{
			name: "Change in Service ports",
			oldObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 80}},
				},
			},
			newObj: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					ManagedFields:   []metav1.ManagedFieldsEntry{{}, {}},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{Port: 8080}},
				},
			},
			wantChange: true,
		},
		{
			name: "Change in labels",
			oldObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Labels:          map[string]string{"app": "v1"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Labels:          map[string]string{"app": "v2"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChange: true,
		},
		{
			name: "Change in annotations",
			oldObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Annotations:     map[string]string{"note": "v1"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Annotations:     map[string]string{"note": "v2"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChange: true,
		},
		{
			name: "Change with user Karmada labels",
			oldObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Labels:          map[string]string{policyv1alpha1.NamespaceSkipAutoPropagationLabel: "true"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Labels:          map[string]string{policyv1alpha1.NamespaceSkipAutoPropagationLabel: "false"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstructuredOldObj, err := helper.ToUnstructured(tt.oldObj)
			if err != nil {
				klog.Errorf("Failed to transform oldObj, error: %v", err)
				return
			}

			unstructuredNewObj, err := helper.ToUnstructured(tt.newObj)
			if err != nil {
				klog.Errorf("Failed to transform newObj, error: %v", err)
				return
			}

			got := SpecificationChanged(unstructuredOldObj, unstructuredNewObj)
			if tt.wantChange != got {
				t.Fatalf("SpecificationChanged() got %v, want %v", got, tt.wantChange)
			}
		})
	}
}

func TestResourceChangeByKarmada(t *testing.T) {
	tests := []struct {
		name                string
		oldObj              interface{}
		newObj              interface{}
		wantChangeByKarmada bool
	}{
		{
			name: "Change by Karmada",
			oldObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Labels:          map[string]string{"app.karmada.io/managed": "true"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Labels:          map[string]string{"app.karmada.io/managed": "false"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChangeByKarmada: true,
		},
		{
			name: "Change not by Karmada",
			oldObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Labels:          map[string]string{"app": "v1"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Labels:          map[string]string{"app": "v2"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChangeByKarmada: false,
		},
		{
			name: "Change in Karmada annotations",
			oldObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123456",
					Annotations:     map[string]string{"note.karmada.io/managed": "true"},
				},
				Data: map[string]string{"key": "value"},
			},
			newObj: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "123457",
					Annotations:     map[string]string{"note.karmada.io/managed": "false"},
				},
				Data: map[string]string{"key": "value"},
			},
			wantChangeByKarmada: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldUnstructured, err := helper.ToUnstructured(tt.oldObj)
			if err != nil {
				t.Fatalf("Failed to convert oldObj to unstructured: %v", err)
			}
			newUnstructured, err := helper.ToUnstructured(tt.newObj)
			if err != nil {
				t.Fatalf("Failed to convert newObj to unstructured: %v", err)
			}

			got := ResourceChangeByKarmada(oldUnstructured, newUnstructured)
			if got != tt.wantChangeByKarmada {
				t.Errorf("ResourceChangeByKarmada() = %v, want %v", got, tt.wantChangeByKarmada)
			}
		})
	}
}
