package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_retainDeploymentFields(t *testing.T) {
	hpa := autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-hpa",
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "nginx",
				APIVersion: "apps/v1",
			},
		},
	}
	hpaList := &autoscalingv2.HorizontalPodAutoscalerList{Items: []autoscalingv2.HorizontalPodAutoscaler{hpa}}

	cli := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithLists(hpaList).WithIndex(&autoscalingv2.HorizontalPodAutoscaler{}, IndexKeyHPAScaleTargetRef,
		func(object client.Object) []string {
			hpa := object.(*autoscalingv2.HorizontalPodAutoscaler)
			return []string{hpa.Spec.ScaleTargetRef.String()}
		}).Build()
	desiredNum, observedNum := int32(2), int32(4)

	observedDeploy := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &observedNum,
		},
	}
	desiredDeploy := observedDeploy
	desiredDeploy.Spec.Replicas = &desiredNum

	observed, _ := helper.ToUnstructured(&observedDeploy)
	desired, _ := helper.ToUnstructured(&desiredDeploy)

	observedDeploy2 := observedDeploy
	observedDeploy2.Name = "nginx-2"
	desiredDeploy2 := observedDeploy2
	desiredDeploy2.Spec.Replicas = &desiredNum

	observed2, _ := helper.ToUnstructured(&observedDeploy2)
	desired2, _ := helper.ToUnstructured(&desiredDeploy2)

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
			want:    observed,
			wantErr: false,
		},
		{
			name: "deployment is not control by hpa",
			args: args{
				desired:  desired2,
				observed: observed2,
			},
			want:    desired2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retainDeploymentFields(cli)(tt.args.desired, tt.args.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("retainDeploymentFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "retainDeploymentFields(%v, %v)", tt.args.desired, tt.args.observed)
		})
	}
}
