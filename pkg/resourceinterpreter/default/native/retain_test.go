package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_retainK8sWorkloadReplicas(t *testing.T) {
	desiredNum, observedNum := int32(2), int32(4)
	observed, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &observedNum,
		},
	})
	desired, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				util.RetainReplicasLabel: util.RetainReplicasValue,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredNum,
		},
	})
	want, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				util.RetainReplicasLabel: util.RetainReplicasValue,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &observedNum,
		},
	})
	desired2, _ := helper.ToUnstructured(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredNum,
		},
	})
	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "deployment is control by hpa",
			args: args{
				desired:  desired,
				observed: observed,
			},
			want:    want,
			wantErr: false,
		},
		{
			name: "deployment is not control by hpa",
			args: args{
				desired:  desired2,
				observed: observed,
			},
			want:    desired2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainWorkloadReplicas(tt.args.desired, tt.args.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("reflectPodDisruptionBudgetStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "retainDeploymentFields(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}
