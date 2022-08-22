package proxy

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadainformers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/proxy/store"
)

var (
	podGVK  = corev1.SchemeGroupVersion.WithKind("Pod")
	nodeGVK = corev1.SchemeGroupVersion.WithKind("Node")

	podSelector  = searchv1alpha1.ResourceSelector{APIVersion: podGVK.GroupVersion().String(), Kind: podGVK.Kind}
	nodeSelector = searchv1alpha1.ResourceSelector{APIVersion: nodeGVK.GroupVersion().String(), Kind: nodeGVK.Kind}

	restMapper = meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
)

func init() {
	restMapper.Add(podGVK, meta.RESTScopeNamespace)
	restMapper.Add(nodeGVK, meta.RESTScopeRoot)
}

func TestController_reconcile(t *testing.T) {
	echoStrings := func(ss ...string) string {
		sort.Strings(ss)
		return strings.Join(ss, ",")
	}
	tests := []struct {
		name  string
		input []runtime.Object
		want  map[string]string
	}{
		{
			name:  "all empty",
			input: []runtime.Object{},
			want:  map[string]string{},
		},
		{
			name: "resource registered, while cluster not registered",
			input: []runtime.Object{
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							podSelector,
						},
					},
				},
			},
			want: map[string]string{},
		},
		{
			name: "pod and node are registered",
			input: []runtime.Object{
				newCluster("cluster1"),
				newCluster("cluster2"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							podSelector,
							nodeSelector,
						},
					},
				},
			},
			want: map[string]string{
				"cluster1": echoStrings("pods", "nodes"),
				"cluster2": echoStrings("pods", "nodes"),
			},
		},
		{
			name: "register pod in cluster1, register node in cluster2",
			input: []runtime.Object{
				newCluster("cluster1"),
				newCluster("cluster2"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster:     policyv1alpha1.ClusterAffinity{ClusterNames: []string{"cluster1"}},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{podSelector},
					},
				},
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr2"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster:     policyv1alpha1.ClusterAffinity{ClusterNames: []string{"cluster2"}},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{nodeSelector},
					},
				},
			},
			want: map[string]string{
				"cluster1": echoStrings("pods"),
				"cluster2": echoStrings("nodes"),
			},
		},
		{
			name: "register pod,node in cluster1, register node in cluster2",
			input: []runtime.Object{
				newCluster("cluster1"),
				newCluster("cluster2"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster:     policyv1alpha1.ClusterAffinity{ClusterNames: []string{"cluster1"}},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{podSelector, nodeSelector},
					},
				},
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr2"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster:     policyv1alpha1.ClusterAffinity{ClusterNames: []string{"cluster2"}},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{nodeSelector},
					},
				},
			},
			want: map[string]string{
				"cluster1": echoStrings("pods", "nodes"),
				"cluster2": echoStrings("nodes"),
			},
		},
		{
			name: "register pod twice in one ResourceRegistry",
			input: []runtime.Object{
				newCluster("cluster1"),
				newCluster("cluster2"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							podSelector,
							podSelector,
						},
					},
				},
			},
			want: map[string]string{
				"cluster1": echoStrings("pods"),
			},
		},
		{
			name: "register pod twice in two ResourceRegistries",
			input: []runtime.Object{
				newCluster("cluster1"),
				newCluster("cluster2"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							podSelector,
						},
					},
				},
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr2"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							podSelector,
						},
					},
				},
			},
			want: map[string]string{
				"cluster1": echoStrings("pods"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := map[string]string{}
			karmadaClientset := karmadafake.NewSimpleClientset(tt.input...)
			karmadaFactory := karmadainformers.NewSharedInformerFactory(karmadaClientset, 0)

			ctl := &Controller{
				restMapper:     restMapper,
				clusterLister:  karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
				registryLister: karmadaFactory.Search().V1alpha1().ResourceRegistries().Lister(),
				store: &cacheFuncs{
					UpdateCacheFunc: func(m map[string]map[schema.GroupVersionResource]struct{}) error {
						for clusterName, resources := range m {
							resourceNames := make([]string, 0, len(resources))
							for resource := range resources {
								resourceNames = append(resourceNames, resource.Resource)
							}
							actual[clusterName] = echoStrings(resourceNames...)
						}
						if len(actual) != len(m) {
							return fmt.Errorf("cluster duplicate: %#v", m)
						}
						return nil
					},
				},
			}
			stopCh := make(chan struct{})
			defer close(stopCh)
			karmadaFactory.Start(stopCh)
			karmadaFactory.WaitForCacheSync(stopCh)

			err := ctl.reconcile(workKey)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("diff: %v", cmp.Diff(actual, tt.want))
			}
		})
	}
}

func newCluster(name string) *clusterv1alpha1.Cluster {
	c := &clusterv1alpha1.Cluster{}
	c.Name = name
	return c
}

type cacheFuncs struct {
	UpdateCacheFunc          func(resourcesByCluster map[string]map[schema.GroupVersionResource]struct{}) error
	HasResourceFunc          func(resource schema.GroupVersionResource) bool
	GetResourceFromCacheFunc func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error)
	StopFunc                 func()
}

var _ store.Cache = &cacheFuncs{}

func (c *cacheFuncs) UpdateCache(resourcesByCluster map[string]map[schema.GroupVersionResource]struct{}) error {
	if c.UpdateCacheFunc == nil {
		panic("implement me")
	}
	return c.UpdateCacheFunc(resourcesByCluster)
}

func (c *cacheFuncs) HasResource(resource schema.GroupVersionResource) bool {
	if c.HasResourceFunc == nil {
		panic("implement me")
	}
	return c.HasResourceFunc(resource)
}

func (c *cacheFuncs) GetResourceFromCache(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (runtime.Object, string, error) {
	if c.GetResourceFromCacheFunc == nil {
		panic("implement me")
	}
	return c.GetResourceFromCacheFunc(ctx, gvr, namespace, name)
}

func (c *cacheFuncs) Stop() {
	if c.StopFunc != nil {
		c.StopFunc()
	}
}
