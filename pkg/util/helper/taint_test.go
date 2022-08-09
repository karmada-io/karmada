package helper

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
