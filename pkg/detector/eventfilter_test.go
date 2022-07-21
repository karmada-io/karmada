package detector

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
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
					Replicas: pointer.Int32(3),
					Template: v1.PodTemplateSpec{},
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
					Replicas: pointer.Int32(5),
					Template: v1.PodTemplateSpec{},
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
					Replicas: pointer.Int32(3),
					Template: v1.PodTemplateSpec{},
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
					Replicas: pointer.Int32(3),
					Template: v1.PodTemplateSpec{},
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
					Replicas: pointer.Int32(3),
					Template: v1.PodTemplateSpec{},
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
					Replicas: pointer.Int32(5),
					Template: v1.PodTemplateSpec{},
					Paused:   false,
				},
				Status: workloadv1alpha1.WorkloadStatus{ReadyReplicas: 3},
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
