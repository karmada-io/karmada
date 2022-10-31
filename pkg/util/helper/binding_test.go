package helper

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestHasScheduledReplica(t *testing.T) {
	tests := []struct {
		name           string
		scheduleResult []workv1alpha2.TargetCluster
		want           bool
	}{
		{
			name: "all targetCluster have replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name:     "bar",
					Replicas: 2,
				},
			},
			want: true,
		},
		{
			name: "a targetCluster has replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name:     "foo",
					Replicas: 1,
				},
				{
					Name: "bar",
				},
			},
			want: true,
		},
		{
			name: "another targetCluster has replicas",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name: "foo",
				},
				{
					Name:     "bar",
					Replicas: 1,
				},
			},
			want: true,
		},
		{
			name: "not assigned replicas for a cluster",
			scheduleResult: []workv1alpha2.TargetCluster{
				{
					Name: "foo",
				},
				{
					Name: "bar",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasScheduledReplica(tt.scheduleResult); got != tt.want {
				t.Errorf("HasScheduledReplica() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObtainBindingSpecExistingClusters(t *testing.T) {
	tests := []struct {
		name        string
		bindingSpec workv1alpha2.ResourceBindingSpec
		want        sets.String
	}{
		{
			name: "unique cluster name without GracefulEvictionTasks field",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3"),
		},
		{
			name: "all spec fields do not contain duplicate cluster names",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member4",
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3", "member4"),
		},
		{
			name: "duplicate cluster name",
			bindingSpec: workv1alpha2.ResourceBindingSpec{
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     "member1",
						Replicas: 2,
					},
					{
						Name:     "member2",
						Replicas: 3,
					},
				},
				RequiredBy: []workv1alpha2.BindingSnapshot{
					{
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member3",
								Replicas: 2,
							},
						},
					},
				},
				GracefulEvictionTasks: []workv1alpha2.GracefulEvictionTask{
					{
						FromCluster: "member3",
					},
				},
			},
			want: sets.NewString("member1", "member2", "member3"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ObtainBindingSpecExistingClusters(tt.bindingSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ObtainBindingSpecExistingClusters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortClusterByWeight(t *testing.T) {
	type args struct {
		m map[string]int64
	}
	tests := []struct {
		name string
		args args
		want ClusterWeightInfoList
	}{
		{
			name: "nil",
			args: args{
				m: nil,
			},
			want: []ClusterWeightInfo{},
		},
		{
			name: "empty",
			args: args{
				m: map[string]int64{},
			},
			want: []ClusterWeightInfo{},
		},
		{
			name: "sort",
			args: args{
				m: map[string]int64{
					"cluster11": 1,
					"cluster12": 2,
					"cluster13": 3,
					"cluster21": 1,
					"cluster22": 2,
					"cluster23": 3,
				},
			},
			want: []ClusterWeightInfo{
				{ClusterName: "cluster13", Weight: 3},
				{ClusterName: "cluster23", Weight: 3},
				{ClusterName: "cluster12", Weight: 2},
				{ClusterName: "cluster22", Weight: 2},
				{ClusterName: "cluster11", Weight: 1},
				{ClusterName: "cluster21", Weight: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortClusterByWeight(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortClusterByWeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindOrphanWorks(t *testing.T) {
	type args struct {
		c                client.Client
		bindingNamespace string
		bindingName      string
		expectClusters   sets.String
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "get cluster name error",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       "wrong format",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "binding"),
							},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
								workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
							},
						},
					},
				).Build(),
				bindingNamespace: "default",
				bindingName:      "binding",
				expectClusters:   sets.NewString("clusterx"),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "namespace scope",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "binding"),
							},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
								workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-labels",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels:          map[string]string{},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
								workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-annotation",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "binding"),
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-cluster",
							Namespace:       names.ExecutionSpacePrefix + "clusterx",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "binding"),
							},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
								workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
							},
						},
					},
				).Build(),
				bindingNamespace: "default",
				bindingName:      "binding",
				expectClusters:   sets.NewString("clusterx"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "w1",
						Namespace:       names.ExecutionSpacePrefix + "cluster1",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "binding"),
						},
						Annotations: map[string]string{
							workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
							workv1alpha2.ResourceBindingNameAnnotationKey:      "binding",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cluster scope",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "w1",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", "binding"),
							},
							Annotations: map[string]string{
								workv1alpha2.ClusterResourceBindingAnnotationKey: "binding",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-labels",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels:          map[string]string{},
							Annotations: map[string]string{
								workv1alpha2.ClusterResourceBindingAnnotationKey: "binding",
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-annotation",
							Namespace:       names.ExecutionSpacePrefix + "cluster1",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", "binding"),
							},
						},
					},
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "not-selected-because-of-cluster",
							Namespace:       names.ExecutionSpacePrefix + "clusterx",
							ResourceVersion: "999",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", "binding"),
							},
							Annotations: map[string]string{
								workv1alpha2.ClusterResourceBindingAnnotationKey: "binding",
							},
						},
					},
				).Build(),
				bindingNamespace: "",
				bindingName:      "binding",
				expectClusters:   sets.NewString("clusterx"),
			},
			want: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "w1",
						Namespace:       names.ExecutionSpacePrefix + "cluster1",
						ResourceVersion: "999",
						Labels: map[string]string{
							workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", "binding"),
						},
						Annotations: map[string]string{
							workv1alpha2.ClusterResourceBindingAnnotationKey: "binding",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindOrphanWorks(tt.args.c, tt.args.bindingNamespace, tt.args.bindingName, tt.args.expectClusters)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindOrphanWorks() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveOrphanWorks(t *testing.T) {
	makeWork := func(name string) *workv1alpha1.Work {
		return &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: names.ExecutionSpacePrefix + "c",
				ResourceVersion: "9999",
			},
		}
	}

	type args struct {
		c     client.Client
		works []workv1alpha1.Work
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "works is empty",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					makeWork("w1"), makeWork("w2"), makeWork("w3")).Build(),
				works: nil,
			},
			wantErr: false,
			want:    []workv1alpha1.Work{*makeWork("w1"), *makeWork("w2"), *makeWork("w3")},
		},
		{
			name: "remove existing and non existing",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					makeWork("w1"), makeWork("w2"), makeWork("w3")).Build(),
				works: []workv1alpha1.Work{*makeWork("w3"), *makeWork("w4")},
			},
			wantErr: true,
			want:    []workv1alpha1.Work{*makeWork("w1"), *makeWork("w2")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemoveOrphanWorks(tt.args.c, tt.args.works); (err != nil) != tt.wantErr {
				t.Errorf("RemoveOrphanWorks() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := &workv1alpha1.WorkList{}
			if err := tt.args.c.List(context.TODO(), got); err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(got.Items, tt.want) {
				t.Errorf("RemoveOrphanWorks() got = %v, want %v", got.Items, tt.want)
			}
		})
	}
}

func TestFetchWorkload(t *testing.T) {
	type args struct {
		dynamicClient   dynamic.Interface
		informerManager func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager
		restMapper      meta.RESTMapper
		resource        workv1alpha2.ObjectReference
	}
	tests := []struct {
		name    string
		args    args
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "kind is not registered",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				restMapper: meta.NewDefaultRESTMapper(nil),
				resource:   workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "not found",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "namespace scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}}),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":              "pod",
					"namespace":         "default",
					"creationTimestamp": nil,
				},
			}},
			wantErr: false,
		},
		{
			name: "namespace scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"}})
					m := genericmanager.NewSingleClusterInformerManager(c, 0, stopCh)
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":              "pod",
					"namespace":         "default",
					"creationTimestamp": nil,
				},
			}},
			wantErr: false,
		},
		{
			name: "cluster scope: get from client",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}}),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					return genericmanager.NewSingleClusterInformerManager(dynamicfake.NewSimpleDynamicClient(scheme.Scheme), 0, stopCh)
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]interface{}{
					"name":              "node",
					"creationTimestamp": nil,
				},
			}},
			wantErr: false,
		},
		{
			name: "cluster scope: get from cache",
			args: args{
				dynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				informerManager: func(stopCh <-chan struct{}) genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}})
					m := genericmanager.NewSingleClusterInformerManager(c, 0, stopCh)
					m.Lister(corev1.SchemeGroupVersion.WithResource("nodes"))
					m.Start()
					m.WaitForCacheSync()
					return m
				},
				restMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Node"), meta.RESTScopeRoot)
					return m
				}(),
				resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       "node",
				},
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Node",
				"metadata": map[string]interface{}{
					"name":              "node",
					"creationTimestamp": nil,
				},
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopCh := make(chan struct{})
			mgr := tt.args.informerManager(stopCh)
			got, err := FetchWorkload(tt.args.dynamicClient, mgr, tt.args.restMapper, tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchWorkload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				// client will add some default fields in spec & status. We don't compare these fields.
				delete(got.Object, "spec")
				delete(got.Object, "status")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FetchWorkload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteWorkByRBNamespaceAndName(t *testing.T) {
	type args struct {
		c         client.Client
		namespace string
		name      string
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "work is not found",
			args: args{
				c:         fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				namespace: "default",
				name:      "foo",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "delete",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name: "w1", Namespace: names.ExecutionSpacePrefix + "cluster",
							Labels: map[string]string{
								workv1alpha2.ResourceBindingReferenceKey: names.GenerateBindingReferenceKey("default", "foo"),
							},
							Annotations: map[string]string{
								workv1alpha2.ResourceBindingNameAnnotationKey:      "foo",
								workv1alpha2.ResourceBindingNamespaceAnnotationKey: "default",
							},
						},
					},
				).Build(),
				namespace: "default",
				name:      "foo",
			},
			want:    []workv1alpha1.Work{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteWorkByRBNamespaceAndName(tt.args.c, tt.args.namespace, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteWorkByRBNamespaceAndName() error = %v, wantErr %v", err, tt.wantErr)
			}
			list := &workv1alpha1.WorkList{}
			if err := tt.args.c.List(context.TODO(), list); err != nil {
				t.Error(err)
				return
			}
			if got := list.Items; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteWorkByRBNamespaceAndName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteWorkByCRBName(t *testing.T) {
	type args struct {
		c    client.Client
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    []workv1alpha1.Work
		wantErr bool
	}{
		{
			name: "delete",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&workv1alpha1.Work{
						ObjectMeta: metav1.ObjectMeta{
							Name: "w1", Namespace: names.ExecutionSpacePrefix + "cluster",
							Labels: map[string]string{
								workv1alpha2.ClusterResourceBindingReferenceKey: names.GenerateBindingReferenceKey("", "foo"),
							},
							Annotations: map[string]string{
								workv1alpha2.ClusterResourceBindingAnnotationKey: "foo",
							},
						},
					},
				).Build(),
				name: "foo",
			},
			want:    []workv1alpha1.Work{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteWorkByCRBName(tt.args.c, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteWorkByRBNamespaceAndName() error = %v, wantErr %v", err, tt.wantErr)
			}
			list := &workv1alpha1.WorkList{}
			if err := tt.args.c.List(context.TODO(), list); err != nil {
				t.Error(err)
				return
			}
			if got := list.Items; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteWorkByRBNamespaceAndName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateReplicaRequirements(t *testing.T) {
	type args struct {
		podTemplate *corev1.PodTemplateSpec
	}
	tests := []struct {
		name string
		args args
		want *workv1alpha2.ReplicaRequirements
	}{
		{
			name: "nodeClaim and resource request are nil",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{},
			},
			want: nil,
		},
		{
			name: "has nodeClaim",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{"foo": "foo1"},
						Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
								}}}},
						},
					},
				},
			},
			want: &workv1alpha2.ReplicaRequirements{
				NodeClaim: &workv1alpha2.NodeClaim{
					NodeSelector: map[string]string{"foo": "foo1"},
					Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
					HardNodeAffinity: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
						}}},
				},
			},
		},
		{
			name: "has resourceRequest",
			args: args{
				podTemplate: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
								},
							},
						}},
					},
				},
			},
			want: &workv1alpha2.ReplicaRequirements{
				ResourceRequest: map[corev1.ResourceName]resource.Quantity{
					"cpu": *resource.NewMilliQuantity(200, resource.DecimalSI),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateReplicaRequirements(tt.args.podTemplate); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateReplicaRequirements() = %v, want %v", got, tt.want)
			}
		})
	}
}
