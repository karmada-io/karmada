package store

import (
	"context"
	"encoding/base64"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func Test_watchMux_StopBySelf(t *testing.T) {
	m := newWatchMux()

	for i := 0; i < 10; i++ {
		w := newFakeWatcher(wait.NeverStop)
		m.AddSource(w, func(event watch.Event) {})
		go func() {
			ticker := time.NewTimer(time.Second)
			for {
				ticker.Reset(time.Millisecond)
				<-ticker.C
				if _, stopped := w.TryAdd(nil); stopped {
					return
				}
			}
		}()
	}

	m.Start()

	var recvCount int32
	time.AfterFunc(time.Second, m.Stop)
	timeout := time.After(time.Second * 5)
	for {
		select {
		case _, ok := <-m.ResultChan():
			if !ok {
				if recvCount == 0 {
					t.Error("receive no events")
				}
				return
			}
			recvCount++
		case <-timeout:
			t.Error("timeout")
			m.Stop()
			return
		}
	}
}

func Test_watchMux_StopBySource(t *testing.T) {
	m := newWatchMux()
	defer m.Stop()

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		w := newFakeWatcher(ctx.Done())
		m.AddSource(w, func(event watch.Event) {})
		go func() {
			ticker := time.NewTimer(time.Second)
			for {
				ticker.Reset(time.Millisecond)
				<-ticker.C
				if _, stopped := w.TryAdd(nil); stopped {
					return
				}
			}
		}()
	}

	m.Start()

	var recvCount int32
	timeout := time.After(time.Second * 5)
	for {
		select {
		case _, ok := <-m.ResultChan():
			if !ok {
				if recvCount == 0 {
					t.Error("receive no events")
				}
				return
			}
			recvCount++
		case <-timeout:
			t.Error("timeout")
			return
		}
	}
}

type fakeWatcher struct {
	result  chan watch.Event
	done    chan struct{}
	stopped bool
	sync.Mutex
}

func newFakeWatcher(stopCh <-chan struct{}) *fakeWatcher {
	f := &fakeWatcher{
		result: make(chan watch.Event),
		done:   make(chan struct{}),
	}
	go func() {
		defer f.Stop()
		select {
		case <-stopCh:
			return
		case <-f.done:
			return
		}
	}()
	return f
}

func (f *fakeWatcher) Stop() {
	f.Lock()
	defer f.Unlock()
	if !f.stopped {
		close(f.result)
		close(f.done)
		f.stopped = true
	}
}

func (f *fakeWatcher) ResultChan() <-chan watch.Event {
	return f.result
}

func (f *fakeWatcher) TryAdd(obj runtime.Object) (added bool, stopped bool) {
	f.Lock()
	defer f.Unlock()
	if f.stopped {
		return false, true
	}

	select {
	case f.result <- watch.Event{Type: watch.Added, Object: obj}:
		return true, false
	default:
		return false, false
	}
}

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
