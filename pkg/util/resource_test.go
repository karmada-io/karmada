package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewResource(t *testing.T) {
	tests := []struct {
		name string
		rl   corev1.ResourceList
		want *Resource
	}{
		{
			name: "Resource",
			rl: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(5, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(5, resource.BinarySI),
			},
			want: &Resource{
				MilliCPU:         5,
				Memory:           5,
				EphemeralStorage: 5,
				AllowedPodNumber: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewResource(tt.rl); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResource() = %v, want %v", got, tt.want)
			}
		})
	}
}
