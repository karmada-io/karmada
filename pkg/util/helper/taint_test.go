package helper

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

var (
	unreachableTaintTemplate = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterUnreachable,
		Effect: corev1.TaintEffectNoExecute,
	}

	notReadyTaintTemplate = &corev1.Taint{
		Key:    clusterv1alpha1.TaintClusterNotReady,
		Effect: corev1.TaintEffectNoExecute,
	}
)

func TestUpdateClusterControllerTaint(t *testing.T) {
	type args struct {
		taints         []corev1.Taint
		taintsToAdd    []*corev1.Taint
		taintsToRemove []*corev1.Taint
	}
	tests := []struct {
		name       string
		args       args
		wantTaints []corev1.Taint
		wantErr    bool
	}{
		{
			name: "ready condition from true to false",
			args: args{
				taints:         nil,
				taintsToAdd:    []*corev1.Taint{notReadyTaintTemplate.DeepCopy()},
				taintsToRemove: []*corev1.Taint{unreachableTaintTemplate.DeepCopy()},
			},
			wantTaints: []corev1.Taint{*notReadyTaintTemplate},
			wantErr:    false,
		},
		{
			name: "ready condition from true to unknown",
			args: args{
				taints:         nil,
				taintsToAdd:    []*corev1.Taint{unreachableTaintTemplate.DeepCopy()},
				taintsToRemove: []*corev1.Taint{notReadyTaintTemplate.DeepCopy()},
			},
			wantTaints: []corev1.Taint{*unreachableTaintTemplate},
			wantErr:    false,
		},
		{
			name: "ready condition from false to unknown",
			args: args{
				taints:         []corev1.Taint{*notReadyTaintTemplate},
				taintsToAdd:    []*corev1.Taint{unreachableTaintTemplate.DeepCopy()},
				taintsToRemove: []*corev1.Taint{notReadyTaintTemplate.DeepCopy()},
			},
			wantTaints: []corev1.Taint{*unreachableTaintTemplate},
			wantErr:    false,
		},
		{
			name: "ready condition from false to true",
			args: args{
				taints:         []corev1.Taint{*notReadyTaintTemplate},
				taintsToAdd:    []*corev1.Taint{},
				taintsToRemove: []*corev1.Taint{notReadyTaintTemplate.DeepCopy(), unreachableTaintTemplate.DeepCopy()},
			},
			wantTaints: nil,
			wantErr:    false,
		},
		{
			name: "ready condition from unknown to true",
			args: args{
				taints:         []corev1.Taint{*unreachableTaintTemplate},
				taintsToAdd:    []*corev1.Taint{},
				taintsToRemove: []*corev1.Taint{notReadyTaintTemplate.DeepCopy(), unreachableTaintTemplate.DeepCopy()},
			},
			wantTaints: nil,
			wantErr:    false,
		},
		{
			name: "ready condition from unknown to false",
			args: args{
				taints:         []corev1.Taint{*unreachableTaintTemplate},
				taintsToAdd:    []*corev1.Taint{notReadyTaintTemplate.DeepCopy()},
				taintsToRemove: []*corev1.Taint{unreachableTaintTemplate.DeepCopy()},
			},
			wantTaints: []corev1.Taint{*notReadyTaintTemplate},
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cluster := &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "member"},
				Spec: clusterv1alpha1.ClusterSpec{
					Taints: tt.args.taints,
				},
			}
			c := fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster).Build()

			if err := UpdateClusterControllerTaint(ctx, c, tt.args.taintsToAdd, tt.args.taintsToRemove, cluster); (err != nil) != tt.wantErr {
				t.Errorf("UpdateClusterControllerTaint() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := c.Get(ctx, client.ObjectKey{Name: cluster.Name}, cluster); err != nil {
				t.Fatalf("Failed to get cluster %s: %v", cluster.Name, err)
			}

			if len(cluster.Spec.Taints) != len(tt.wantTaints) {
				t.Errorf("Cluster gotTaints = %v, want %v", cluster.Spec.Taints, tt.wantTaints)
			}
			for i := range cluster.Spec.Taints {
				if cluster.Spec.Taints[i].Key != tt.wantTaints[i].Key ||
					cluster.Spec.Taints[i].Value != tt.wantTaints[i].Value ||
					cluster.Spec.Taints[i].Effect != tt.wantTaints[i].Effect {
					t.Errorf("Cluster gotTaints = %v, want %v", cluster.Spec.Taints, tt.wantTaints)
				}
			}
		})
	}
}

func TestTaintExists(t *testing.T) {
	type args struct {
		taints      []corev1.Taint
		taintToFind *corev1.Taint
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exist",
			args: args{
				taints: []corev1.Taint{
					{
						Key:    clusterv1alpha1.TaintClusterUnreachable,
						Effect: corev1.TaintEffectNoExecute,
					},
					{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoExecute,
					},
				},
				taintToFind: &corev1.Taint{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectNoExecute,
				},
			},
			want: true,
		},
		{
			name: "not exist",
			args: args{
				taints: []corev1.Taint{
					{
						Key:    clusterv1alpha1.TaintClusterUnreachable,
						Effect: corev1.TaintEffectNoExecute,
					},
					{
						Key:    clusterv1alpha1.TaintClusterNotReady,
						Effect: corev1.TaintEffectNoExecute,
					},
				},
				taintToFind: &corev1.Taint{
					Key:    clusterv1alpha1.TaintClusterNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TaintExists(tt.args.taints, tt.args.taintToFind); got != tt.want {
				t.Errorf("TaintExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTolerationExists(t *testing.T) {
	type args struct {
		tolerations      []corev1.Toleration
		tolerationToFind *corev1.Toleration
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "not exist",
			args: args{
				tolerations: []corev1.Toleration{
					{
						Key:      clusterv1alpha1.TaintClusterUnreachable,
						Effect:   corev1.TaintEffectNoExecute,
						Operator: corev1.TolerationOpEqual,
						Value:    "foo",
					},
					{
						Key:      clusterv1alpha1.TaintClusterNotReady,
						Effect:   corev1.TaintEffectNoExecute,
						Operator: corev1.TolerationOpEqual,
						Value:    "foo",
					},
				},
				tolerationToFind: &corev1.Toleration{
					Key:      clusterv1alpha1.TaintClusterNotReady,
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpEqual,
					Value:    "foo",
				},
			},
			want: false,
		},
		{
			name: "exist",
			args: args{
				tolerations: []corev1.Toleration{
					{
						Key:      clusterv1alpha1.TaintClusterUnreachable,
						Effect:   corev1.TaintEffectNoExecute,
						Operator: corev1.TolerationOpEqual,
						Value:    "foo",
					},
					{
						Key:      clusterv1alpha1.TaintClusterNotReady,
						Effect:   corev1.TaintEffectNoExecute,
						Operator: corev1.TolerationOpEqual,
						Value:    "foo",
					},
				},
				tolerationToFind: &corev1.Toleration{
					Key:      clusterv1alpha1.TaintClusterNotReady,
					Effect:   corev1.TaintEffectNoExecute,
					Operator: corev1.TolerationOpEqual,
					Value:    "foo",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TolerationExists(tt.args.tolerations, tt.args.tolerationToFind); got != tt.want {
				t.Errorf("TolerationExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasNoExecuteTaints(t *testing.T) {
	tests := []struct {
		name   string
		taints []corev1.Taint
		want   bool
	}{
		{
			name: "has NoExecute taints",
			taints: []corev1.Taint{
				{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectNoExecute,
				},
				{
					Key:    clusterv1alpha1.TaintClusterNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			want: true,
		},
		{
			name: "no NoExecute taints",
			taints: []corev1.Taint{
				{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
				{
					Key:    clusterv1alpha1.TaintClusterNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasNoExecuteTaints(tt.taints); got != tt.want {
				t.Errorf("HasNoExecuteTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNoExecuteTaints(t *testing.T) {
	tests := []struct {
		name   string
		taints []corev1.Taint
		want   []corev1.Taint
	}{
		{
			name: "has NoExecute taints",
			taints: []corev1.Taint{
				{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectNoExecute,
				},
				{
					Key:    clusterv1alpha1.TaintClusterNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			want: []corev1.Taint{
				{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectNoExecute,
				},
			},
		},
		{
			name: "no NoExecute taints",
			taints: []corev1.Taint{
				{
					Key:    clusterv1alpha1.TaintClusterUnreachable,
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
				{
					Key:    clusterv1alpha1.TaintClusterNotReady,
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNoExecuteTaints(tt.taints); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNoExecuteTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMinTolerationTime(t *testing.T) {
	timeAdded1 := metav1.NewTime(time.Unix(0, 0))
	timeAdded2 := metav1.NewTime(timeAdded1.Add(5000))
	type args struct {
		noExecuteTaints []corev1.Taint
		usedTolerations []corev1.Toleration
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "the length of taints is 0",
			args: args{
				noExecuteTaints: nil,
				usedTolerations: []corev1.Toleration{
					{
						Key:               "key",
						Operator:          "Equal",
						Value:             "value",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
				},
			},
			want: -1,
		}, {
			name: "the length of tolerations is 0",
			args: args{
				noExecuteTaints: []corev1.Taint{
					{
						Key:       "key",
						Value:     "value",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
				},
				usedTolerations: nil,
			},
			want: 0,
		}, {
			name: "test get minimal toleration time",
			args: args{
				noExecuteTaints: []corev1.Taint{
					{
						Key:       "key1",
						Value:     "value1",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
					{
						Key:       "key2",
						Value:     "value2",
						Effect:    "NoExecute",
						TimeAdded: &timeAdded1,
					},
					{
						Key:       "key3",
						Value:     "value3",
						Effect:    "NoExecute",
						TimeAdded: &timeAdded2,
					},
				},
				usedTolerations: []corev1.Toleration{
					{
						Key:               "key1",
						Operator:          "Equal",
						Value:             "value1",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
					{
						Key:               "key2",
						Operator:          "Equal",
						Value:             "value2",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(10000),
					},
					{
						Key:               "key3",
						Operator:          "Equal",
						Value:             "value3",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(20000),
					},
				},
			},
			want: 0,
		}, {
			name: "exist mismatch taint",
			args: args{
				noExecuteTaints: []corev1.Taint{
					{
						Key:       "key1",
						Value:     "value1",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
					{
						Key:       "key2",
						Value:     "value2",
						Effect:    "NoExecute",
						TimeAdded: &timeAdded1,
					},
					{
						Key:       "key3",
						Value:     "value3",
						Effect:    "NoExecute",
						TimeAdded: &timeAdded2,
					},
				},
				usedTolerations: []corev1.Toleration{
					{
						Key:               "key3",
						Operator:          "Equal",
						Value:             "value3",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
					{
						Key:               "key4",
						Operator:          "Equal",
						Value:             "value4",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(10000),
					},
					{
						Key:               "key5",
						Operator:          "Equal",
						Value:             "value5",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(20000),
					},
				},
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetMinTolerationTime(tt.args.noExecuteTaints, tt.args.usedTolerations); got != tt.want {
				t.Errorf("GetMinTolerationTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMatchingTolerations(t *testing.T) {
	type args struct {
		taints      []corev1.Taint
		tolerations []corev1.Toleration
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		want1 []corev1.Toleration
	}{
		{
			name: "test no taints",
			args: args{
				taints: nil,
				tolerations: []corev1.Toleration{
					{
						Key:               "key",
						Operator:          "Equal",
						Value:             "value",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
					{
						Key:               "key",
						Operator:          "Equal",
						Value:             "value",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(10000),
					},
				},
			},
			want:  true,
			want1: []corev1.Toleration{},
		}, {
			name: "test no tolerations",
			args: args{
				taints: []corev1.Taint{
					{
						Key:       "key",
						Value:     "value",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
				},
				tolerations: nil,
			},
			want:  false,
			want1: []corev1.Toleration{},
		}, {
			name: "exist mismatch taint",
			args: args{
				taints: []corev1.Taint{
					{
						Key:       "key",
						Value:     "value",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
				},
				tolerations: []corev1.Toleration{
					{
						Key:               "key",
						Operator:          "Equal",
						Value:             "value",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
					{
						Key:               "key",
						Operator:          "Equal",
						Value:             "value",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(10000),
					},
				},
			},
			want: true,
			want1: []corev1.Toleration{
				{
					Key:               "key",
					Operator:          "Equal",
					Value:             "value",
					Effect:            "NoExecute",
					TolerationSeconds: nil,
				},
			},
		}, {
			name: "don't exist mismatch taint",
			args: args{
				taints: []corev1.Taint{
					{
						Key:       "key",
						Value:     "value",
						Effect:    "NoExecute",
						TimeAdded: nil,
					},
				},
				tolerations: []corev1.Toleration{
					{
						Key:               "key1",
						Operator:          "Equal",
						Value:             "value1",
						Effect:            "NoExecute",
						TolerationSeconds: nil,
					},
					{
						Key:               "key2",
						Operator:          "Equal",
						Value:             "value2",
						Effect:            "NoExecute",
						TolerationSeconds: pointer.Int64(10000),
					},
				},
			},
			want:  false,
			want1: []corev1.Toleration{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetMatchingTolerations(tt.args.taints, tt.args.tolerations)
			if got != tt.want {
				t.Errorf("GetMatchingTolerations() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetMatchingTolerations() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
