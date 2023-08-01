package dependenciesdistributor

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func Test_dependentObjectReferenceMatches(t *testing.T) {
	type args struct {
		objectKey        keys.ClusterWideKey
		referenceBinding *workv1alpha2.ResourceBinding
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test custom resource",
			args: args{
				objectKey: keys.ClusterWideKey{
					Group:     "example-stgzr.karmada.io",
					Version:   "v1alpha1",
					Kind:      "Foot5zmh",
					Namespace: "karmadatest-vpvll",
					Name:      "cr-fxzq6",
				},
				referenceBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
						bindingDependenciesAnnotationKey: "[{\"apiVersion\":\"example-stgzr.karmada.io/v1alpha1\",\"kind\":\"Foot5zmh\",\"namespace\":\"karmadatest-vpvll\",\"name\":\"cr-fxzq6\"}]",
					}},
				},
			},
			want: true,
		},
		{
			name: "test configmap",
			args: args{
				objectKey: keys.ClusterWideKey{
					Group:     "",
					Version:   "v1",
					Kind:      "ConfigMap",
					Namespace: "karmadatest-h46wh",
					Name:      "configmap-8w426",
				},
				referenceBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
						bindingDependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"namespace\":\"karmadatest-h46wh\",\"name\":\"configmap-8w426\"}]",
					}},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dependentObjectReferenceMatches(tt.args.objectKey, tt.args.referenceBinding)
			if got != tt.want {
				t.Errorf("dependentObjectReferenceMatches() got = %v, want %v", got, tt.want)
			}
		})
	}
}
