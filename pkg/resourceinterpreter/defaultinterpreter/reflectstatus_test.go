package defaultinterpreter

import (
	"reflect"
	"testing"

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
