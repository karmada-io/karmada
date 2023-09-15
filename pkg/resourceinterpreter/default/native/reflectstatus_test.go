package native

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_getEntireStatus(t *testing.T) {
	testMap := map[string]interface{}{"key": "value"}
	wantRawExtension, _ := helper.BuildStatusRawExtension(testMap)
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			"object doesn't have status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
			},
			nil,
			false,
		},
		{
			"object have wrong format status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": "a string",
					},
				},
			},
			nil,
			true,
		},
		{
			"object have correct format status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": testMap,
					},
				},
			},
			wantRawExtension,
			false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := reflectWholeStatus(tt.args.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("reflectWholeStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reflectWholeStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_reflectPodDisruptionBudgetStatus(t *testing.T) {
	currStatus := policyv1.PodDisruptionBudgetStatus{
		CurrentHealthy:     1,
		DesiredHealthy:     1,
		DisruptionsAllowed: 1,
		ExpectedPods:       1,
	}
	currStatusUnstructured, _ := helper.ToUnstructured(&policyv1.PodDisruptionBudget{Status: currStatus})
	wantRawExtension, _ := helper.BuildStatusRawExtension(&currStatus)
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			"object doesn't have status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
			},
			nil,
			false,
		},
		{
			"object have wrong format status",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": "a string",
					},
				},
			},
			nil,
			true,
		},
		{
			"object have correct format status",
			args{
				currStatusUnstructured,
			},
			wantRawExtension,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectPodDisruptionBudgetStatus(tt.args.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("reflectPodDisruptionBudgetStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reflectPodDisruptionBudgetStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_reflectHorizontalPodAutoscalerStatus(t *testing.T) {
	hpaStatus, _ := helper.ToUnstructured(&autoscalingv2.HorizontalPodAutoscaler{
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 2,
			DesiredReplicas: 2,
		},
	})
	grabStatus, _ := helper.BuildStatusRawExtension(autoscalingv2.HorizontalPodAutoscalerStatus{
		CurrentReplicas: 2,
		DesiredReplicas: 2,
	})
	nilHpaStatus, _ := helper.ToUnstructured(&map[string]interface{}{})
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name:    "reflect hap status",
			object:  hpaStatus,
			want:    grabStatus,
			wantErr: false,
		},
		{
			name:    "reflect nil hpa status",
			object:  nilHpaStatus,
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reflectHorizontalPodAutoscalerStatus(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("Test_reflectHorizontalPodAutoscalerStatus() err = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "reflectHorizontalPodAutoscalerStatus(%v)", tt.object)
		})
	}
}
