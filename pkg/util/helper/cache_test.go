/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestEnsureInformerHandlersReady(t *testing.T) {
	cluster := &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
	gvr := corev1.SchemeGroupVersion.WithResource("pods")
	handler := &cache.ResourceEventHandlerFuncs{}
	dynamicClient := fake.NewSimpleDynamicClient(scheme.Scheme)
	kubeClient := controllerfake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build()

	tests := []struct {
		name                 string
		manager              func(context.Context) genericmanager.MultiClusterInformerManager
		clusterClientSetFunc util.NewClusterDynamicClientSetFunc
		wantSynced           bool
		wantErr              bool
	}{
		{
			name: "registers missing handler and starts informer",
			manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
				m := genericmanager.NewMultiClusterInformerManager(ctx)
				m.ForCluster(cluster.Name, dynamicClient, 0)
				return m
			},
			clusterClientSetFunc: func(string, client.Client, *util.ClientOption) (*util.DynamicClusterClient, error) {
				return nil, errors.New("should use existing manager")
			},
			wantSynced: false,
		},
		{
			name: "returns synced when handler exists and informer synced",
			manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
				m := genericmanager.NewMultiClusterInformerManager(ctx)
				m.ForCluster(cluster.Name, dynamicClient, 0).ForResource(gvr, handler)
				m.Start(cluster.Name)
				m.WaitForCacheSync(cluster.Name)
				return m
			},
			clusterClientSetFunc: func(string, client.Client, *util.ClientOption) (*util.DynamicClusterClient, error) {
				return nil, errors.New("should use existing manager")
			},
			wantSynced: true,
		},
		{
			name: "creates manager with cluster client when missing",
			manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
				return genericmanager.NewMultiClusterInformerManager(ctx)
			},
			clusterClientSetFunc: func(clusterName string, _ client.Client, _ *util.ClientOption) (*util.DynamicClusterClient, error) {
				return &util.DynamicClusterClient{ClusterName: clusterName, DynamicClientSet: dynamicClient}, nil
			},
			wantSynced: false,
		},
		{
			name: "returns error when cluster client creation fails",
			manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
				return genericmanager.NewMultiClusterInformerManager(ctx)
			},
			clusterClientSetFunc: func(string, client.Client, *util.ClientOption) (*util.DynamicClusterClient, error) {
				return nil, errors.New("failed to build dynamic client")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := tt.manager(t.Context())

			gotSynced, err := EnsureInformerHandlersReady(cluster, []schema.GroupVersionResource{gvr}, handler, mgr, tt.clusterClientSetFunc, kubeClient, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotSynced != tt.wantSynced {
				t.Fatalf("EnsureInformerHandlersReady() synced = %v, want %v", gotSynced, tt.wantSynced)
			}

			singleClusterManager := mgr.GetSingleClusterManager(cluster.Name)
			if singleClusterManager == nil {
				t.Fatalf("expected single cluster manager")
			}
			if !singleClusterManager.IsHandlerExist(gvr, handler) {
				t.Fatalf("expected handler to be registered")
			}
		})
	}
}

func TestGetObjectFromCache(t *testing.T) {
	type args struct {
		restMapper meta.RESTMapper
		manager    func(ctx context.Context) genericmanager.MultiClusterInformerManager
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					return genericmanager.NewMultiClusterInformerManager(ctx)
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					return genericmanager.NewMultiClusterInformerManager(ctx)
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(ctx)
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(ctx)
					m.ForCluster("cluster", fake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}},
					), 0)
					return m
				},
				fedKey: keys.FederatedKey{Cluster: "cluster", ClusterWideKey: keys.ClusterWideKey{
					Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod",
				}},
			},
			want: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "pod", "namespace": "default"},
				"spec":     map[string]any{"containers": nil},
				"status":   map[string]any{},
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(ctx)
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
				manager: func(ctx context.Context) genericmanager.MultiClusterInformerManager {
					m := genericmanager.NewMultiClusterInformerManager(ctx)
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
			want: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "pod", "namespace": "default"},
				"spec":     map[string]any{"containers": nil},
				"status":   map[string]any{},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			mgr := tt.args.manager(ctx)
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
		manager    func(context.Context) genericmanager.SingleClusterInformerManager
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
				manager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, fake.NewSimpleDynamicClient(scheme.Scheme), 0)
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
				manager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(ctx, fake.NewSimpleDynamicClient(scheme.Scheme), 0)
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
				manager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := fake.NewSimpleDynamicClient(scheme.Scheme, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					return genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "pod", "namespace": "default"},
				"spec":     map[string]any{"containers": nil},
				"status":   map[string]any{},
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
				manager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					m := genericmanager.NewSingleClusterInformerManager(ctx, fake.NewSimpleDynamicClient(scheme.Scheme), 0)
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
				manager: func(ctx context.Context) genericmanager.SingleClusterInformerManager {
					c := fake.NewSimpleDynamicClient(scheme.Scheme, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					m := genericmanager.NewSingleClusterInformerManager(ctx, c, 0)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				cwk: &keys.ClusterWideKey{Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod"},
			},
			want: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "pod", "namespace": "default"},
				"spec":     map[string]any{"containers": nil},
				"status":   map[string]any{},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			mgr := tt.args.manager(ctx)
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
