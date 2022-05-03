package helper

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
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
