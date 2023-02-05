package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCommandInitOption_SecretFromSpec(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	type args struct {
		name       string
		secretType corev1.SecretType
		data       map[string]string
	}
	tests := []struct {
		name string
		args args
		want *corev1.Secret
	}{
		{
			name: "get secret from spec",
			args: args{
				name:       "foo-secret",
				secretType: "Opaque",
				data:       map[string]string{"foo": "bar"},
			},
			want: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-secret",
					Namespace: "karmada",
					Labels:    map[string]string{"karmada.io/bootstrapping": "secret-defaults"},
				},
				Type:       "Opaque",
				StringData: map[string]string{"foo": "bar"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdOpt.SecretFromSpec(tt.args.name, tt.args.secretType, tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CommandInitOption.SecretFromSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}
