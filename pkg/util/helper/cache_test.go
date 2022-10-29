package helper

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func TestGetObjectFromCache(t *testing.T) {
	type args struct {
		restMapper meta.RESTMapper
		manager    func(<-chan struct{}) genericmanager.MultiClusterInformerManager
		fedKey     keys.FederatedKey
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "kind not registered",
			args: args{
				restMapper: meta.NewDefaultRESTMapper(nil),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					return genericmanager.NewMultiClusterInformerManager(stopCh)
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "cluster manager not found",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					return genericmanager.NewMultiClusterInformerManager(stopCh)
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from client, and not found",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(stopCh)
					m.ForCluster("cluster", fake.NewSimpleDynamicClient(scheme.Scheme), 0)
					return m
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from client, and get success",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(stopCh)
					m.ForCluster("cluster", fake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}},
					), 0)
					return m
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]interface{}{"name": "pod", "namespace": "default", "creationTimestamp": nil},
				"spec":     map[string]interface{}{"containers": nil},
				"status":   map[string]interface{}{},
			}},
			wantErr: false,
		},
		{
			name: "get from cache, and not found",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(stopCh)
					m.ForCluster("cluster", fake.NewSimpleDynamicClient(scheme.Scheme), 0).
						Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
					m.Start("cluster")
					m.WaitForCacheSync("cluster")
					return m
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from cache, and get success",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(stopCh)
					m.ForCluster("cluster", fake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}},
					), 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
					m.Start("cluster")
					m.WaitForCacheSync("cluster")
					return m
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]interface{}{"name": "pod", "namespace": "default", "creationTimestamp": nil},
				"spec":     map[string]interface{}{"containers": nil},
				"status":   map[string]interface{}{},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			defer close(stopCh)
			mgr := tt.args.manager(stopCh)
			got, err := GetObjectFromCache(tt.args.restMapper, mgr, tt.args.fedKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectFromCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetObjectFromCache() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetObjectFromSingleClusterCache(t *testing.T) {
	type args struct {
		restMapper meta.RESTMapper
		manager    func(<-chan struct{}) genericmanager.SingleClusterInformerManager
		cwk        *keys.ClusterWideKey
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "kind not registered",
			args: args{
				restMapper: meta.NewDefaultRESTMapper(nil),
				manager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(fake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from client, and not found",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(fake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from client, and get success",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					c := fake.NewSimpleDynamicClient(scheme.Scheme, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					return genericmanager.NewSingleClusterInformerManager(c, 0, stopCh)
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]interface{}{"name": "pod", "namespace": "default", "creationTimestamp": nil},
				"spec":     map[string]interface{}{"containers": nil},
				"status":   map[string]interface{}{},
			}},
			wantErr: false,
		},
		{
			name: "get from cache, and not found",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					m := genericmanager.NewSingleClusterInformerManager(fake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get from cache, and get success",
			args: args{
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				manager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					c := fake.NewSimpleDynamicClient(scheme.Scheme, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					m := genericmanager.NewSingleClusterInformerManager(c, 0, stopCh)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]interface{}{"name": "pod", "namespace": "default", "creationTimestamp": nil},
				"spec":     map[string]interface{}{"containers": nil},
				"status":   map[string]interface{}{},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			defer close(stopCh)
			mgr := tt.args.manager(stopCh)
			got, err := GetObjectFromSingleClusterCache(tt.args.restMapper, mgr, tt.args.cwk)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectFromCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetObjectFromCache() got = %v, want %v", got, tt.want)
			}
		})
	}
}
