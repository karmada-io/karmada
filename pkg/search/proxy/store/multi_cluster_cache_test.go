package store

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	kubetesting "k8s.io/client-go/testing"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

var (
	podGVR    = corev1.SchemeGroupVersion.WithResource("pods")
	nodeGVR   = corev1.SchemeGroupVersion.WithResource("nodes")
	secretGVR = corev1.SchemeGroupVersion.WithResource("secrets")

	podGVK    = corev1.SchemeGroupVersion.WithKind("Pod")
	nodeGVK   = corev1.SchemeGroupVersion.WithKind("Node")
	secretGVK = corev1.SchemeGroupVersion.WithKind("Secret")

	restMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	scheme     = runtime.NewScheme()
)

func init() {
	restMapper.Add(podGVK, meta.RESTScopeNamespace)
	restMapper.Add(nodeGVK, meta.RESTScopeRoot)
	restMapper.Add(secretGVK, meta.RESTScopeNamespace)
	scheme.AddKnownTypes(corev1.SchemeGroupVersion,
		&corev1.Pod{}, &corev1.PodList{},
		&corev1.Node{}, &corev1.NodeList{},
		&corev1.Secret{}, &corev1.SecretList{})
}

func TestMultiClusterCache_UpdateCache(t *testing.T) {
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()

	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	resources := map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR, nodeGVR),
		cluster2.Name: resourceSet(podGVR),
	}

	err := cache.UpdateCache(resources)
	if err != nil {
		t.Error(err)
	}

	if len(cache.cache) != 2 {
		t.Errorf("cache len expect %v, actual %v", 2, len(cache.cache))
	}

	// Then test removing cluster2 and remove node cache for cluster1
	err = cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Error(err)
	}
	if len(cache.cache) != 1 {
		t.Errorf("cache len expect %v, actual %v", 1, len(cache.cache))
	}
}

func TestMultiClusterCache_HasResource(t *testing.T) {
	fakeClient := fakedynamic.NewSimpleDynamicClient(scheme)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		return fakeClient, nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	resources := map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR, nodeGVR),
		cluster2.Name: resourceSet(podGVR),
	}
	err := cache.UpdateCache(resources)
	if err != nil {
		t.Error(err)
		return
	}

	tests := []struct {
		name string
		args schema.GroupVersionResource
		want bool
	}{
		{
			"has gets",
			podGVR,
			true,
		},
		{
			"has nodes",
			nodeGVR,
			true,
		},
		{
			"has no secret",
			secretGVR,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := cache.HasResource(tt.args)
			if tt.want != actual {
				t.Errorf("should return %v", tt.want)
			}
		})
	}
}

func TestMultiClusterCache_GetResourceFromCache(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	resources := map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR),
		cluster2.Name: resourceSet(podGVR),
	}
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11", withDefaultNamespace()),
		newUnstructuredObject(podGVK, "pod_conflict", withDefaultNamespace()),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21", withDefaultNamespace()),
		newUnstructuredObject(podGVK, "pod_conflict", withDefaultNamespace()),
	)

	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(resources)
	if err != nil {
		t.Error(err)
		return
	}

	type args struct {
		gvr       schema.GroupVersionResource
		namespace string
		name      string
	}
	type want struct {
		objectName string
		cluster    string
		errAssert  func(error) bool
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get pod from cluster1",
			args: args{
				gvr:       podGVR,
				namespace: metav1.NamespaceDefault,
				name:      "pod11",
			},
			want: want{
				objectName: "pod11",
				cluster:    "cluster1",
				errAssert:  noError,
			},
		},
		{
			name: "get pod from cluster2",
			args: args{
				gvr:       podGVR,
				namespace: metav1.NamespaceDefault,
				name:      "pod21",
			},
			want: want{
				objectName: "pod21",
				cluster:    "cluster2",
				errAssert:  noError,
			},
		},
		{
			name: "pod not found",
			args: args{
				gvr:       podGVR,
				namespace: metav1.NamespaceDefault,
				name:      "podz",
			},
			want: want{
				errAssert: apierrors.IsNotFound,
			},
		},
		{
			name: "get resource conflict",
			args: args{
				gvr:       podGVR,
				namespace: metav1.NamespaceDefault,
				name:      "pod_conflict",
			},
			want: want{
				errAssert: apierrors.IsConflict,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, cluster, err := cache.GetResourceFromCache(context.TODO(), tt.args.gvr, tt.args.namespace, tt.args.name)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpect error: %v", err)
				return
			}
			if err != nil {
				return
			}

			accessor, err := meta.Accessor(obj)
			if err != nil {
				t.Error(err)
				return
			}
			if tt.want.objectName != accessor.GetName() {
				t.Errorf("Expect object %v. But got %v", tt.want.objectName, accessor.GetName())
			}
			if !reflect.DeepEqual(tt.want.cluster, cluster) {
				t.Errorf("Cluster diff: %v", cmp.Diff(tt.want.cluster, cluster))
			}
		})
	}
}

func TestMultiClusterCache_Get(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11", withDefaultNamespace(), withResourceVersion("1000")),
		newUnstructuredObject(nodeGVK, "node11", withResourceVersion("1000")),
		newUnstructuredObject(podGVK, "pod-conflict", withDefaultNamespace(), withResourceVersion("1000")),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21", withDefaultNamespace(), withResourceVersion("2000")),
		newUnstructuredObject(podGVK, "pod-conflict", withDefaultNamespace(), withResourceVersion("2000")),
	)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR, nodeGVR),
		cluster2.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		ctx     context.Context
		gvr     schema.GroupVersionResource
		name    string
		options *metav1.GetOptions
	}
	type want struct {
		object    runtime.Object
		errAssert func(error) bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get pod11 from cluster1",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				name:    "pod11",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    newUnstructuredObject(podGVK, "pod11", withDefaultNamespace(), withResourceVersion(buildMultiClusterRV("cluster1", "1000")), withCacheSourceAnnotation("cluster1")),
				errAssert: noError,
			},
		},
		{
			name: "get pod21 from cluster2",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				name:    "pod21",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    newUnstructuredObject(podGVK, "pod21", withDefaultNamespace(), withResourceVersion(buildMultiClusterRV("cluster2", "2000")), withCacheSourceAnnotation("cluster2")),
				errAssert: noError,
			},
		},
		{
			name: "get pod not found",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				name:    "podz",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    nil,
				errAssert: apierrors.IsNotFound,
			},
		},
		{
			name: "get pod with large resource version",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				name:    "pod11",
				options: &metav1.GetOptions{ResourceVersion: "9999"},
			},
			want: want{
				object:    nil,
				errAssert: apierrors.IsTimeout,
			},
		},
		{
			name: "get node from cluster1",
			args: args{
				ctx:     context.TODO(),
				gvr:     nodeGVR,
				name:    "node11",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    newUnstructuredObject(nodeGVK, "node11", withResourceVersion(buildMultiClusterRV("cluster1", "1000")), withCacheSourceAnnotation("cluster1")),
				errAssert: noError,
			},
		},
		{
			name: "get pod conflict",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				name:    "pod-conflict",
				options: &metav1.GetOptions{},
			},
			want: want{
				errAssert: apierrors.IsConflict,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := cache.Get(tt.args.ctx, tt.args.gvr, tt.args.name, tt.args.options)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.want.object, obj) {
				t.Errorf("Objects diff: %v", cmp.Diff(tt.want.object, obj))
			}
		})
	}
}

func TestMultiClusterCache_Get_Namespaced(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11", withNamespace("ns1"), withResourceVersion("1000")),
		newUnstructuredObject(podGVK, "pod12", withNamespace("ns2"), withResourceVersion("1000")),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21", withNamespace("ns1"), withResourceVersion("2000")),
		newUnstructuredObject(podGVK, "pod22", withNamespace("ns2"), withResourceVersion("2000")),
		newUnstructuredObject(podGVK, "pod23", withNamespace("ns3"), withResourceVersion("2000")),
	)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: {
			podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1")},
		},
		cluster2.Name: {
			podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1", "ns2")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		ns      string
		gvr     schema.GroupVersionResource
		name    string
		options *metav1.GetOptions
	}
	type want struct {
		object    runtime.Object
		errAssert func(error) bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "pod is cached in single ns cache",
			args: args{
				ns:      "ns1",
				gvr:     podGVR,
				name:    "pod11",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    newUnstructuredObject(podGVK, "pod11", withNamespace("ns1"), withResourceVersion(buildMultiClusterRV("cluster1", "1000")), withCacheSourceAnnotation("cluster1")),
				errAssert: noError,
			},
		},
		{
			name: "pod is not cached in single ns cache",
			args: args{
				ns:      "ns1",
				gvr:     podGVR,
				name:    "pod12",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    nil,
				errAssert: apierrors.IsNotFound,
			},
		},
		{
			name: "pod is cached in multi ns cache",
			args: args{
				ns:      "ns1",
				gvr:     podGVR,
				name:    "pod21",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    newUnstructuredObject(podGVK, "pod21", withNamespace("ns1"), withResourceVersion(buildMultiClusterRV("cluster2", "2000")), withCacheSourceAnnotation("cluster2")),
				errAssert: noError,
			},
		},
		{
			name: "pod not found in cache.",
			args: args{
				ns:      "ns1",
				gvr:     podGVR,
				name:    "non-exist",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    nil,
				errAssert: apierrors.IsNotFound,
			},
		},
		{
			name: "ns is not cached",
			args: args{
				ns:      "ns3",
				gvr:     podGVR,
				name:    "pod23",
				options: &metav1.GetOptions{},
			},
			want: want{
				object:    nil,
				errAssert: apierrors.IsNotFound,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := cache.Get(request.WithNamespace(context.TODO(), tt.args.ns), tt.args.gvr, tt.args.name, tt.args.options)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.want.object, obj) {
				t.Errorf("Objects diff: %v", cmp.Diff(tt.want.object, obj))
			}
		})
	}
}

func TestMultiClusterCache_List(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11", withDefaultNamespace(), withResourceVersion("1001"), withLabel("app", "foo")),
		newUnstructuredObject(podGVK, "pod12", withDefaultNamespace(), withResourceVersion("1002")),
		newUnstructuredObject(podGVK, "pod13", withDefaultNamespace(), withResourceVersion("1003"), withLabel("app", "foo")),
		newUnstructuredObject(podGVK, "pod14", withDefaultNamespace(), withResourceVersion("1004")),
		newUnstructuredObject(podGVK, "pod15", withDefaultNamespace(), withResourceVersion("1005")),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21", withDefaultNamespace(), withResourceVersion("2001"), withLabel("app", "foo")),
		newUnstructuredObject(podGVK, "pod22", withDefaultNamespace(), withResourceVersion("2002")),
		newUnstructuredObject(podGVK, "pod23", withDefaultNamespace(), withResourceVersion("2003"), withLabel("app", "foo")),
		newUnstructuredObject(podGVK, "pod24", withDefaultNamespace(), withResourceVersion("2004")),
		newUnstructuredObject(podGVK, "pod25", withDefaultNamespace(), withResourceVersion("2005")),
	)

	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}

	type args struct {
		ctx     context.Context
		gvr     schema.GroupVersionResource
		options *metainternalversion.ListOptions
	}
	type want struct {
		resourceVersion string
		names           sets.Set[string]
		errAssert       func(error) bool
	}
	tests := []struct {
		name      string
		resources map[string]map[schema.GroupVersionResource]*MultiNamespace
		args      args
		want      want
	}{
		{
			name:      "list gets with labelSelector",
			resources: map[string]map[schema.GroupVersionResource]*MultiNamespace{},
			args: args{
				ctx: request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr: podGVR,
				options: &metainternalversion.ListOptions{
					LabelSelector: asLabelSelector("app=foo"),
				},
			},
			want: want{
				// fakeDynamic returns list with resourceVersion=""
				resourceVersion: "",
				names:           sets.New[string](),
				errAssert:       noError,
			},
		},
		{
			name: "list gets",
			resources: map[string]map[schema.GroupVersionResource]*MultiNamespace{
				cluster1.Name: resourceSet(podGVR),
				cluster2.Name: resourceSet(podGVR),
			},
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				// fakeDynamic returns list with resourceVersion=""
				resourceVersion: buildMultiClusterRV("cluster1", "", "cluster2", ""),
				names:           sets.New[string]("pod11", "pod12", "pod13", "pod14", "pod15", "pod21", "pod22", "pod23", "pod24", "pod25"),
				errAssert:       noError,
			},
		},
		{
			name: "list gets with labelSelector",
			resources: map[string]map[schema.GroupVersionResource]*MultiNamespace{
				cluster1.Name: resourceSet(podGVR),
				cluster2.Name: resourceSet(podGVR),
			},
			args: args{
				ctx: request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr: podGVR,
				options: &metainternalversion.ListOptions{
					LabelSelector: asLabelSelector("app=foo"),
				},
			},
			want: want{
				// fakeDynamic returns list with resourceVersion=""
				resourceVersion: buildMultiClusterRV("cluster1", "", "cluster2", ""),
				names:           sets.New[string]("pod11", "pod13", "pod21", "pod23"),
				errAssert:       noError,
			},
		},
		// TODO: add case for limit option. But fakeClient doesn't support it.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewMultiClusterCache(newClientFunc, restMapper)
			defer cache.Stop()
			err := cache.UpdateCache(tt.resources)
			if err != nil {
				t.Error(err)
				return
			}

			obj, err := cache.List(tt.args.ctx, tt.args.gvr, tt.args.options)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if err != nil {
				return
			}

			object, err := meta.ListAccessor(obj)
			if err != nil {
				t.Error(err)
				return
			}
			if tt.want.resourceVersion != object.GetResourceVersion() {
				t.Errorf("ResourceVersion want=%v, actual=%v", tt.want.resourceVersion, object.GetResourceVersion())
			}
			names := sets.New[string]()

			err = meta.EachListItem(obj, func(o runtime.Object) error {
				a, err := meta.Accessor(o)
				if err != nil {
					return err
				}
				names.Insert(a.GetName())
				return nil
			})
			if err != nil {
				t.Error(err)
				return
			}

			if !tt.want.names.Equal(names) {
				t.Errorf("List items want=%v, actual=%v", strings.Join(sets.List(tt.want.names), ","), strings.Join(sets.List(names), ","))
			}
		})
	}
}

func TestMultiClusterCache_List_CacheSourceAnnotation(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11"),
		newUnstructuredObject(podGVK, "pod12"),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21"),
		newUnstructuredObject(podGVK, "pod22"),
	)

	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR),
		cluster2.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Error(err)
		return
	}

	list, err := cache.List(context.TODO(), podGVR, &metainternalversion.ListOptions{})
	if err != nil {
		t.Errorf("List error: %v", err)
		return
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("ExtractList error: %v", err)
		return
	}

	expect := []runtime.Object{
		newUnstructuredObject(podGVK, "pod11", withCacheSourceAnnotation("cluster1")),
		newUnstructuredObject(podGVK, "pod12", withCacheSourceAnnotation("cluster1")),
		newUnstructuredObject(podGVK, "pod21", withCacheSourceAnnotation("cluster2")),
		newUnstructuredObject(podGVK, "pod22", withCacheSourceAnnotation("cluster2")),
	}
	if !reflect.DeepEqual(items, expect) {
		t.Errorf("list items diff: %v", cmp.Diff(expect, items))
	}
}

func TestMultiClusterCache_List_Namespaced(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod11", withNamespace("ns1"), withResourceVersion("1000")),
		newUnstructuredObject(podGVK, "pod12", withNamespace("ns2"), withResourceVersion("1000")),
	)
	cluster2Client := fakedynamic.NewSimpleDynamicClient(scheme,
		newUnstructuredObject(podGVK, "pod21", withNamespace("ns1"), withResourceVersion("2000")),
		newUnstructuredObject(podGVK, "pod22", withNamespace("ns2"), withResourceVersion("2000")),
	)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: {podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1")}},
		cluster2.Name: {podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1", "ns2", "ns3")}},
	})
	if err != nil {
		t.Error(err)
		return
	}

	type args struct {
		ns      string
		gvr     schema.GroupVersionResource
		options *metainternalversion.ListOptions
	}
	type want struct {
		names     sets.Set[string]
		errAssert func(error) bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "request all namespaces",
			args: args{
				ns:      metav1.NamespaceAll,
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				names:     sets.New[string]("pod11", "pod21", "pod22"),
				errAssert: noError,
			},
		},
		{
			name: "request ns1 namespaces",
			args: args{
				ns:      "ns1",
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				names:     sets.New[string]("pod11", "pod21"),
				errAssert: noError,
			},
		},
		{
			name: "request ns2 namespaces",
			args: args{
				ns:      "ns2",
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				names:     sets.New[string]("pod22"),
				errAssert: noError,
			},
		},
		{
			name: "request ns3 namespaces",
			args: args{
				ns:      "ns3",
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				names:     sets.New[string](),
				errAssert: noError,
			},
		},
		{
			name: "request ns4 namespaces",
			args: args{
				ns:      "ns4",
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				names:     sets.New[string](),
				errAssert: noError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := cache.List(request.WithNamespace(context.TODO(), tt.args.ns), tt.args.gvr, tt.args.options)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if err != nil {
				return
			}

			names := sets.New[string]()

			err = meta.EachListItem(obj, func(o runtime.Object) error {
				a, err := meta.Accessor(o)
				if err != nil {
					return err
				}
				names.Insert(a.GetName())
				return nil
			})
			if err != nil {
				t.Error(err)
				return
			}

			if !tt.want.names.Equal(names) {
				t.Errorf("List items want=%v, actual=%v", strings.Join(sets.List(tt.want.names), ","), strings.Join(sets.List(names), ","))
			}
		})
	}
}

func TestMultiClusterCache_Watch(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := NewEnhancedFakeDynamicClientWithResourceVersion(scheme, "1002",
		newUnstructuredObject(podGVK, "pod11", withDefaultNamespace(), withResourceVersion("1001")),
		newUnstructuredObject(podGVK, "pod12", withDefaultNamespace(), withResourceVersion("1002")),
	)
	cluster2Client := NewEnhancedFakeDynamicClientWithResourceVersion(scheme, "2002",
		newUnstructuredObject(podGVK, "pod21", withDefaultNamespace(), withResourceVersion("2001")),
		newUnstructuredObject(podGVK, "pod22", withDefaultNamespace(), withResourceVersion("2002")),
	)

	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR),
		cluster2.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Error(err)
		return
	}

	// wait cache synced
	time.Sleep(time.Second)

	// put gets into Cacher.incoming chan
	_ = cluster1Client.Tracker().Add(newUnstructuredObject(podGVK, "pod13", withDefaultNamespace(), withResourceVersion("1003")))
	_ = cluster2Client.Tracker().Add(newUnstructuredObject(podGVK, "pod23", withDefaultNamespace(), withResourceVersion("2003")))
	cluster1Client.versionTracker.Set("1003")
	cluster2Client.versionTracker.Set("2003")

	type args struct {
		options *metainternalversion.ListOptions
	}
	type want struct {
		gets sets.Set[string]
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "resource version is zero",
			args: args{
				options: &metainternalversion.ListOptions{
					ResourceVersion: "0",
				},
			},
			want: want{
				gets: sets.New[string]("pod11", "pod12", "pod13", "pod21", "pod22", "pod23"),
			},
		},
		{
			name: "resource version of cluster2 is zero",
			args: args{
				options: &metainternalversion.ListOptions{
					ResourceVersion: buildMultiClusterRV(cluster1.Name, "1002", cluster2.Name, "0"),
				},
			},
			want: want{
				gets: sets.New[string]("pod13", "pod21", "pod22", "pod23"),
			},
		},
		{
			name: "resource versions are not zero",
			args: args{
				options: &metainternalversion.ListOptions{
					ResourceVersion: buildMultiClusterRV(cluster1.Name, "1002", cluster2.Name, "2002"),
				},
			},
			want: want{
				gets: sets.New[string]("pod13", "pod23"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(request.WithNamespace(context.TODO(), metav1.NamespaceDefault))
			defer cancel()
			watcher, err := cache.Watch(ctx, podGVR, tt.args.options)
			if err != nil {
				t.Error(err)
				return
			}
			defer watcher.Stop()
			waitCh := time.After(time.Second)

			gets := sets.New[string]()
		LOOP:
			for {
				select {
				case event, ok := <-watcher.ResultChan():
					if !ok {
						break LOOP
					}
					accessor, err := meta.Accessor(event.Object)
					if err == nil {
						gets.Insert(accessor.GetName())
					}
				case <-waitCh:
					break LOOP
				}
			}

			if !tt.want.gets.Equal(gets) {
				t.Errorf("Watch() got = %v, but want = %v", gets, tt.want.gets)
			}
		})
	}
}

func TestMultiClusterCache_Watch_Namespaced(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	cluster1Client := NewEnhancedFakeDynamicClientWithResourceVersion(scheme, "1002",
		newUnstructuredObject(podGVK, "pod11", withNamespace("ns1"), withResourceVersion("1001")),
		newUnstructuredObject(podGVK, "pod12", withNamespace("ns2"), withResourceVersion("1002")),
	)
	cluster2Client := NewEnhancedFakeDynamicClientWithResourceVersion(scheme, "2002",
		newUnstructuredObject(podGVK, "pod21", withNamespace("ns1"), withResourceVersion("2001")),
		newUnstructuredObject(podGVK, "pod22", withNamespace("ns2"), withResourceVersion("2002")),
	)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		case cluster2.Name:
			return cluster2Client, nil
		}
		return fakedynamic.NewSimpleDynamicClient(scheme), nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: {podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1")}},
		cluster2.Name: {podGVR: &MultiNamespace{namespaces: sets.New[string]("ns1", "ns2", "ns3")}},
	})
	if err != nil {
		t.Error(err)
		return
	}
	// wait cache synced
	time.Sleep(time.Second)

	type args struct {
		ns string
	}
	type want struct {
		names     sets.Set[string]
		errAssert func(error) bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "request all namespaces",
			args: args{
				ns: metav1.NamespaceAll,
			},
			want: want{
				names:     sets.New[string]("pod11", "pod21", "pod22"),
				errAssert: noError,
			},
		},
		{
			name: "request ns1 namespace",
			args: args{
				ns: "ns1",
			},
			want: want{
				names:     sets.New[string]("pod11", "pod21"),
				errAssert: noError,
			},
		},
		{
			name: "request ns2 namespace",
			args: args{
				ns: "ns2",
			},
			want: want{
				names:     sets.New[string]("pod22"),
				errAssert: noError,
			},
		},
		{
			name: "request ns3 namespace",
			args: args{
				ns: "ns3",
			},
			want: want{
				names:     sets.New[string](),
				errAssert: noError,
			},
		},
		{
			name: "request ns4 namespace",
			args: args{
				ns: "ns4",
			},
			want: want{
				names:     sets.New[string](),
				errAssert: noError,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(request.WithNamespace(context.TODO(), tt.args.ns))
			defer cancel()
			watcher, err := cache.Watch(ctx, podGVR, &metainternalversion.ListOptions{ResourceVersion: "0"})
			if err != nil {
				t.Error(err)
				return
			}
			defer watcher.Stop()
			waitCh := time.After(time.Second)

			gots := sets.New[string]()
		LOOP:
			for {
				select {
				case event, ok := <-watcher.ResultChan():
					if !ok {
						break LOOP
					}
					accessor, err := meta.Accessor(event.Object)
					if err == nil {
						gots.Insert(accessor.GetName())
					}
				case <-waitCh:
					break LOOP
				}
			}

			if !tt.want.names.Equal(gots) {
				t.Errorf("Watch() got = %v, but want = %v", gots, tt.want.names)
			}
		})
	}
}

func newCluster(name string) *clusterv1alpha1.Cluster {
	o := &clusterv1alpha1.Cluster{}
	o.Name = name
	return o
}

func newUnstructuredObject(gvk schema.GroupVersionKind, name string, options ...func(object *unstructured.Unstructured)) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	for _, opt := range options {
		opt(obj)
	}
	return obj
}

func withDefaultNamespace() func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		obj.SetNamespace(metav1.NamespaceDefault)
	}
}

func withNamespace(ns string) func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		obj.SetNamespace(ns)
	}
}

func withResourceVersion(rv string) func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		obj.SetResourceVersion(rv)
	}
}

func withCacheSourceAnnotation(cluster string) func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		addCacheSourceAnnotation(obj, cluster)
	}
}

func withLabel(label, value string) func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		err := unstructured.SetNestedField(obj.Object, value, "metadata", "labels", label)
		if err != nil {
			panic(err)
		}
	}
}

func noError(err error) bool {
	return err == nil
}

func buildMultiClusterRV(clusterAndRV ...string) string {
	m := newMultiClusterResourceVersionWithCapacity(len(clusterAndRV) / 2)
	for i := 0; i < len(clusterAndRV); {
		m.set(clusterAndRV[i], clusterAndRV[i+1])
		i += 2
	}
	return m.String()
}

func asLabelSelector(s string) labels.Selector {
	selector, err := labels.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("Fail to parse %s to labels: %v", s, err))
	}
	return selector
}

func resourceSet(rs ...schema.GroupVersionResource) map[schema.GroupVersionResource]*MultiNamespace {
	m := make(map[schema.GroupVersionResource]*MultiNamespace, len(rs))
	for _, r := range rs {
		m[r] = &MultiNamespace{allNamespaces: true}
	}
	return m
}

type VersionTracker interface {
	Set(string)
	Get() string
}

type versionTracker struct {
	lock sync.RWMutex
	rv   string
}

func NewVersionTracker(rv string) VersionTracker {
	return &versionTracker{
		rv: rv,
	}
}

func (t *versionTracker) Set(rv string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.rv = rv
}

func (t *versionTracker) Get() string {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.rv
}

// EnhancedFakeDynamicClient enhances FakeDynamicClient. It will return resourceVersion for list request.
type EnhancedFakeDynamicClient struct {
	*fakedynamic.FakeDynamicClient
	ObjectReaction kubetesting.ReactionFunc
	versionTracker VersionTracker
}

// NewEnhancedFakeDynamicClientWithResourceVersion returns instance of EnhancedFakeDynamicClient.
func NewEnhancedFakeDynamicClientWithResourceVersion(scheme *runtime.Scheme, rv string, objects ...runtime.Object) *EnhancedFakeDynamicClient {
	v := NewVersionTracker(rv)

	c := fakedynamic.NewSimpleDynamicClient(scheme, objects...)
	c.PrependReactor("list", "*", enhancedListReaction(c.Tracker(), v))

	return &EnhancedFakeDynamicClient{
		FakeDynamicClient: c,
		ObjectReaction:    kubetesting.ObjectReaction(c.Tracker()),
		versionTracker:    v,
	}
}

func enhancedListReaction(o kubetesting.ObjectTracker, v VersionTracker) kubetesting.ReactionFunc {
	return func(act kubetesting.Action) (bool, runtime.Object, error) {
		action, ok := act.(kubetesting.ListActionImpl)
		if !ok {
			return false, nil, nil
		}

		ret, err := o.List(action.GetResource(), action.GetKind(), action.GetNamespace())
		if err != nil {
			return true, ret, err
		}

		accessor, err := meta.ListAccessor(ret)
		if err != nil {
			// object is not a list object, don't change it. Don't return this error.
			return true, ret, nil
		}
		accessor.SetResourceVersion(v.Get())
		return true, ret, nil
	}
}

func Test_prepareBeforeList(t *testing.T) {
	testMRV := &multiClusterResourceVersion{rvs: map[string]string{"c1": "1234", "c2": "5678"}}
	testMRVString := testMRV.String()

	type args struct {
		o *metainternalversion.ListOptions
	}
	tests := []struct {
		name        string
		args        args
		wantCluster string
		wantOptions *metainternalversion.ListOptions
		wantMrv     *multiClusterResourceVersion
	}{
		{
			name: "Continue is empty",
			args: args{
				o: &metainternalversion.ListOptions{
					ResourceVersion: "0",
				},
			},
			wantCluster: "",
			wantOptions: &metainternalversion.ListOptions{
				ResourceVersion: "0",
				Continue:        "",
			},
			wantMrv: newMultiClusterResourceVersionFromString("0"),
		},
		{
			name: "Continue is not empty, and has no continue",
			args: args{
				o: &metainternalversion.ListOptions{
					Continue: func() string {
						c := multiClusterContinue{RV: testMRVString, Cluster: "c2", Continue: ""}
						return c.String()
					}(),
				},
			},
			wantCluster: "c2",
			wantOptions: &metainternalversion.ListOptions{
				ResourceVersion: "5678",
				Continue:        "",
			},
			wantMrv: testMRV,
		},
		{
			name: "Continue is not empty, and has continue",
			args: args{
				o: &metainternalversion.ListOptions{
					Continue: func() string {
						c := multiClusterContinue{RV: testMRVString, Cluster: "c2", Continue: "xxx"}
						return c.String()
					}(),
				},
			},
			wantCluster: "c2",
			wantOptions: &metainternalversion.ListOptions{
				Continue: "xxx",
			},
			wantMrv: testMRV,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCluster, gotOptions, gotMrv := prepareBeforeList(tt.args.o)
			if gotCluster != tt.wantCluster {
				t.Errorf("prepareBeforeList() gotCluster = %v, want %v", gotCluster, tt.wantCluster)
			}
			if !reflect.DeepEqual(gotOptions, tt.wantOptions) {
				t.Errorf("prepareBeforeList() gotOptions = %v, want %v", gotOptions, tt.wantOptions)
			}
			if !reflect.DeepEqual(gotMrv, tt.wantMrv) {
				t.Errorf("prepareBeforeList() gotMrv = %v, want %v", gotMrv, tt.wantMrv)
			}
		})
	}
}

func TestMultiClusterCache_fillMissingClusterResourceVersion(t *testing.T) {
	cluster1 := newCluster("cluster1")
	cluster1Client := NewEnhancedFakeDynamicClientWithResourceVersion(scheme, "1000")
	cluster1Client.AddReactor("*", "nodes", func(kubetesting.Action) (bool, runtime.Object, error) {
		return false, nil, fmt.Errorf("error")
	})

	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		switch cluster {
		case cluster1.Name:
			return cluster1Client, nil
		default:
			return nil, fmt.Errorf("unknown cluster %v", cluster)
		}
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	defer cache.Stop()
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]*MultiNamespace{
		cluster1.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Fatal(err)
	}
	// wait cache synced
	time.Sleep(time.Second)

	type args struct {
		mcv      *multiClusterResourceVersion
		clusters []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantMCV *multiClusterResourceVersion
	}{
		{
			name: "no missing cluster rv",
			args: args{
				mcv: &multiClusterResourceVersion{
					rvs: map[string]string{
						"cluster1": "1234",
					},
				},
				clusters: []string{"cluster1"},
			},
			wantMCV: &multiClusterResourceVersion{
				rvs: map[string]string{
					"cluster1": "1234",
				},
			},
		},
		{
			name: "has missing cluster rv",
			args: args{
				mcv: &multiClusterResourceVersion{
					rvs: map[string]string{},
				},
				clusters: []string{"cluster1"},
			},
			wantMCV: &multiClusterResourceVersion{
				rvs: map[string]string{
					"cluster1": "1000",
				},
			},
		},
		{
			name: "cluster not exist",
			args: args{
				mcv: &multiClusterResourceVersion{
					rvs: map[string]string{},
				},
				clusters: []string{"non-exist"},
			},
			wantMCV: &multiClusterResourceVersion{
				rvs: map[string]string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			if err = cache.fillMissingClusterResourceVersion(ctx, tt.args.mcv, tt.args.clusters, podGVR); (err != nil) != tt.wantErr {
				t.Fatalf("fillMissingClusterResourceVersion() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got := tt.args.mcv; !reflect.DeepEqual(got, tt.wantMCV) {
				t.Fatalf("fillMissingClusterResourceVersion() mcv = %v, want %v", got.String(), tt.wantMCV.String())
			}
		})
	}
}
