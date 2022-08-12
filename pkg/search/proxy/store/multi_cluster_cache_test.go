package store

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

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
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	resources := map[string]map[schema.GroupVersionResource]struct{}{
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
}

func TestMultiClusterCache_HasResource(t *testing.T) {
	fakeClient := fakedynamic.NewSimpleDynamicClient(scheme)
	newClientFunc := func(cluster string) (dynamic.Interface, error) {
		return fakeClient, nil
	}
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	cluster1 := newCluster("cluster1")
	cluster2 := newCluster("cluster2")
	resources := map[string]map[schema.GroupVersionResource]struct{}{
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
			"has pods",
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
	resources := map[string]map[schema.GroupVersionResource]struct{}{
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
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]struct{}{
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
				object:    newUnstructuredObject(podGVK, "pod11", withDefaultNamespace(), withResourceVersion(buildMultiClusterRV("cluster1", "1000"))),
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
				object:    newUnstructuredObject(podGVK, "pod21", withDefaultNamespace(), withResourceVersion(buildMultiClusterRV("cluster2", "2000"))),
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
				object:    newUnstructuredObject(nodeGVK, "node11", withResourceVersion(buildMultiClusterRV("cluster1", "1000"))),
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
	cache := NewMultiClusterCache(newClientFunc, restMapper)
	err := cache.UpdateCache(map[string]map[schema.GroupVersionResource]struct{}{
		cluster1.Name: resourceSet(podGVR),
		cluster2.Name: resourceSet(podGVR),
	})
	if err != nil {
		t.Error(err)
		return
	}

	type args struct {
		ctx     context.Context
		gvr     schema.GroupVersionResource
		options *metainternalversion.ListOptions
	}
	type want struct {
		resourceVersion string
		names           sets.String
		errAssert       func(error) bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list pods",
			args: args{
				ctx:     request.WithNamespace(context.TODO(), metav1.NamespaceDefault),
				gvr:     podGVR,
				options: &metainternalversion.ListOptions{},
			},
			want: want{
				// fakeDynamic returns list with resourceVersion=""
				resourceVersion: buildMultiClusterRV("cluster1", "", "cluster2", ""),
				names:           sets.NewString("pod11", "pod12", "pod13", "pod14", "pod15", "pod21", "pod22", "pod23", "pod24", "pod25"),
				errAssert:       noError,
			},
		},
		{
			name: "list pods with labelSelector",
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
				names:           sets.NewString("pod11", "pod13", "pod21", "pod23"),
				errAssert:       noError,
			},
		},
		// TODO: add case for limit option. But fakeClient doesn't support it.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := cache.List(tt.args.ctx, tt.args.gvr, tt.args.options)
			if !tt.want.errAssert(err) {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if err != nil {
				return
			}

			object := obj.(*unstructured.UnstructuredList)
			if tt.want.resourceVersion != object.GetResourceVersion() {
				t.Errorf("ResourceVersion want=%v, actual=%v", tt.want.resourceVersion, object.GetResourceVersion())
			}
			names := sets.NewString()
			for _, item := range object.Items {
				names.Insert(item.GetName())
			}
			if !tt.want.names.Equal(names) {
				t.Errorf("List items want=%v, actual=%v", strings.Join(tt.want.names.List(), ","), strings.Join(names.List(), ","))
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

func withResourceVersion(rv string) func(*unstructured.Unstructured) {
	return func(obj *unstructured.Unstructured) {
		obj.SetResourceVersion(rv)
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

func resourceSet(rs ...schema.GroupVersionResource) map[schema.GroupVersionResource]struct{} {
	m := make(map[schema.GroupVersionResource]struct{}, len(rs))
	for _, r := range rs {
		m[r] = struct{}{}
	}
	return m
}
