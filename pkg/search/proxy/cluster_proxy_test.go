package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

func TestModifyRequest(t *testing.T) {
	newObjectFunc := func(annotations map[string]string, resourceVersion string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("Pod")
		obj.SetAnnotations(annotations)
		obj.SetResourceVersion(resourceVersion)
		return obj
	}

	type args struct {
		body    interface{}
		cluster string
	}
	type want struct {
		body interface{}
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Empty body",
			args: args{
				body: nil,
			},
			want: want{
				body: nil,
			},
		},
		{
			name: "Body with nil annotations",
			args: args{
				body: newObjectFunc(nil, ""),
			},
			want: want{
				body: newObjectFunc(nil, ""),
			},
		},
		{
			name: "Body with empty annotations",
			args: args{
				body: newObjectFunc(map[string]string{}, ""),
			},
			want: want{
				body: newObjectFunc(map[string]string{}, ""),
			},
		},
		{
			name: "Body with cache source annotation",
			args: args{
				body: newObjectFunc(map[string]string{clusterv1alpha1.CacheSourceAnnotationKey: "bar"}, ""),
			},
			want: want{
				body: newObjectFunc(map[string]string{}, ""),
			},
		},
		{
			name: "Body with single cluster resource version",
			args: args{
				body:    newObjectFunc(nil, "1234"),
				cluster: "cluster1",
			},
			want: want{
				body: newObjectFunc(nil, "1234"),
			},
		},
		{
			name: "Body with multi cluster resource version",
			args: args{
				body:    newObjectFunc(nil, store.BuildMultiClusterResourceVersion(map[string]string{"cluster1": "1234", "cluster2": "5678"})),
				cluster: "cluster1",
			},
			want: want{
				body: newObjectFunc(nil, "1234"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.args.body != nil {
				buf := bytes.NewBuffer(nil)
				err := json.NewEncoder(buf).Encode(tt.args.body)
				if err != nil {
					t.Error(err)
					return
				}
				body = buf
			}
			req, _ := http.NewRequest("PUT", "/api/v1/namespaces/default/pods/foo", body)
			err := modifyRequest(req, tt.args.cluster)
			if err != nil {
				t.Error(err)
				return
			}

			var get runtime.Object
			if req.ContentLength != 0 {
				data, err := ioutil.ReadAll(req.Body)
				if err != nil {
					t.Error(err)
					return
				}

				if int64(len(data)) != req.ContentLength {
					t.Errorf("expect contentLength %v, but got %v", len(data), req.ContentLength)
					return
				}

				get, _, err = unstructured.UnstructuredJSONScheme.Decode(data, nil, nil)
				if err != nil {
					t.Error(err)
					return
				}
			}

			if !reflect.DeepEqual(tt.want.body, get) {
				t.Errorf("get body diff: %v", cmp.Diff(tt.want.body, get))
			}
		})
	}
}
