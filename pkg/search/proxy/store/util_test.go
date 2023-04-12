package store

import (
	"encoding/base64"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func Test_newMultiClusterResourceVersionFromString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want *multiClusterResourceVersion
	}{
		{
			name: "empty",
			args: args{
				s: "",
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{},
			},
		},
		{
			name: "zero",
			args: args{
				s: "0",
			},
			want: &multiClusterResourceVersion{
				rvs:    map[string]string{},
				isZero: true,
			},
		},
		{
			name: "decode error",
			args: args{
				s: "`not encoded`",
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{},
			},
		},
		{
			name: "not a json",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte(`not a json`)),
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{},
			},
		},
		{
			name: "success - normal",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"1","cluster2":"2"}`)),
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{
					"cluster1": "1",
					"cluster2": "2",
				},
			},
		},
		{
			name: "success - empty cluster name",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte(`{"":"1","cluster2":"2"}`)),
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{
					"":         "1",
					"cluster2": "2",
				},
			},
		},
		{
			name: "success - empty ResourceVersion",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"","cluster2":""}`)),
			},
			want: &multiClusterResourceVersion{
				rvs: map[string]string{
					"cluster1": "",
					"cluster2": "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newMultiClusterResourceVersionFromString(tt.args.s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newMultiClusterResourceVersionFromString() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func Test_multiClusterResourceVersion_get(t *testing.T) {
	type fields struct {
		rvs    map[string]string
		isZero bool
	}
	type args struct {
		cluster string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "zero",
			fields: fields{
				isZero: true,
				rvs:    map[string]string{},
			},
			args: args{
				cluster: "cluster1",
			},
			want: "0",
		},
		{
			name: "not exist",
			fields: fields{
				rvs: map[string]string{},
			},
			args: args{
				cluster: "cluster1",
			},
			want: "",
		},
		{
			name: "get success",
			fields: fields{
				rvs: map[string]string{
					"cluster1": "1",
				},
			},
			args: args{
				cluster: "cluster1",
			},
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &multiClusterResourceVersion{
				rvs:    tt.fields.rvs,
				isZero: tt.fields.isZero,
			}
			if got := m.get(tt.args.cluster); got != tt.want {
				t.Errorf("get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_multiClusterResourceVersion_String(t *testing.T) {
	type fields struct {
		rvs    map[string]string
		isZero bool
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "zero",
			fields: fields{
				isZero: true,
				rvs:    map[string]string{},
			},
			want: "0",
		},
		{
			name: "empty",
			fields: fields{
				rvs: map[string]string{},
			},
			want: "",
		},
		{
			name: "get success - normal",
			fields: fields{
				rvs: map[string]string{
					"cluster1": "1",
					"cluster2": "2",
				},
			},
			want: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"1","cluster2":"2"}`)),
		},
		{
			name: "get success - empty cluster name",
			fields: fields{
				rvs: map[string]string{
					"":         "1",
					"cluster2": "2",
				},
			},
			want: base64.RawURLEncoding.EncodeToString([]byte(`{"":"1","cluster2":"2"}`)),
		},
		{
			name: "get success - empty ResourceVersion",
			fields: fields{
				rvs: map[string]string{
					"cluster1": "",
					"cluster2": "",
				},
			},
			want: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"","cluster2":""}`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &multiClusterResourceVersion{
				rvs:    tt.fields.rvs,
				isZero: tt.fields.isZero,
			}
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newMultiClusterContinueFromString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want multiClusterContinue
	}{
		{
			name: "empty",
			args: args{
				s: "",
			},
			want: multiClusterContinue{},
		},
		{
			name: "not encoded",
			args: args{
				s: "not encoded",
			},
			want: multiClusterContinue{},
		},
		{
			name: "not json",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte("not json")),
			},
			want: multiClusterContinue{},
		},
		{
			name: "success",
			args: args{
				s: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster":"cluster1","continue":"1"}`)),
			},
			want: multiClusterContinue{
				Cluster:  "cluster1",
				Continue: "1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newMultiClusterContinueFromString(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newMultiClusterContinueFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_multiClusterContinue_String(t *testing.T) {
	type fields struct {
		RV       string
		Cluster  string
		Continue string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "empty",
			fields: fields{
				RV:       "",
				Cluster:  "",
				Continue: "",
			},
			want: "",
		},
		{
			name: "success",
			fields: fields{
				RV:       "123",
				Cluster:  "cluster1",
				Continue: "1",
			},
			want: base64.RawURLEncoding.EncodeToString([]byte(`{"rv":"123","cluster":"cluster1","continue":"1"}`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &multiClusterContinue{
				RV:       tt.fields.RV,
				Cluster:  tt.fields.Cluster,
				Continue: tt.fields.Continue,
			}
			if got := c.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveCacheSourceAnnotation(t *testing.T) {
	type args struct {
		obj runtime.Object
	}
	type want struct {
		obj     runtime.Object
		changed bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "not a meta",
			args: args{
				obj: &metav1.Status{},
			},
			want: want{
				changed: false,
				obj:     &metav1.Status{},
			},
		},
		{
			name: "annotation not exist",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{},
				},
			},
			want: want{
				changed: false,
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{},
				},
			},
		},
		{
			name: "remove annotation",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							clusterv1alpha1.CacheSourceAnnotationKey: "cluster1",
						},
					},
				},
			},
			want: want{
				changed: true,
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveCacheSourceAnnotation(tt.args.obj)
			if got != tt.want.changed {
				t.Errorf("RemoveCacheSourceAnnotation() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.args.obj, tt.want.obj) {
				t.Errorf("RemoveCacheSourceAnnotation() got obj = %#v, want %#v", tt.args.obj, tt.want.obj)
			}
		})
	}
}

func TestRecoverClusterResourceVersion(t *testing.T) {
	type args struct {
		obj     runtime.Object
		cluster string
	}
	type want struct {
		obj     runtime.Object
		changed bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "not a meta",
			args: args{
				obj:     &metav1.Status{},
				cluster: "cluster1",
			},
			want: want{
				changed: false,
				obj:     &metav1.Status{},
			},
		},
		{
			name: "rv is empty",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "",
					},
				},
				cluster: "cluster1",
			},
			want: want{
				changed: false,
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{},
				},
			},
		},
		{
			name: "rv is 0",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "0",
					},
				},
				cluster: "cluster1",
			},
			want: want{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "0",
					},
				},
				changed: false,
			},
		},
		{
			name: "single cluster rv",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "1000",
					},
				},
				cluster: "cluster1",
			},
			want: want{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "1000",
					},
				},
				changed: false,
			},
		},
		{
			name: "cluster rv not set",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: base64.RawURLEncoding.EncodeToString([]byte(`{}`)),
					},
				},
				cluster: "cluster1",
			},
			want: want{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{},
				},
				changed: true,
			},
		},
		{
			name: "recover cluster rv",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"1","cluster2":"2"}`)),
					},
				},
				cluster: "cluster1",
			},
			want: want{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "1",
					},
				},
				changed: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecoverClusterResourceVersion(tt.args.obj, tt.args.cluster)
			if got != tt.want.changed {
				t.Errorf("RecoverClusterResourceVersion() changed = %v, want %v", got, tt.want.changed)
			}

			if !reflect.DeepEqual(tt.args.obj, tt.want.obj) {
				t.Errorf("RecoverClusterResourceVersion() got obj = %#v, want %#v", tt.args.obj, tt.want.obj)
			}
		})
	}
}

func TestBuildMultiClusterResourceVersion(t *testing.T) {
	type args struct {
		clusterResourceMap map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				clusterResourceMap: nil,
			},
			want: "",
		},
		{
			name: "success",
			args: args{
				clusterResourceMap: map[string]string{
					"cluster1": "1",
				},
			},
			want: base64.RawURLEncoding.EncodeToString([]byte(`{"cluster1":"1"}`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildMultiClusterResourceVersion(tt.args.clusterResourceMap); got != tt.want {
				t.Errorf("BuildMultiClusterResourceVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
