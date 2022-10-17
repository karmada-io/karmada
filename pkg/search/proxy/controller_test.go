package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"

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

func TestController(t *testing.T) {
	restConfig := &restclient.Config{
		Host: "https：//localhost:6443",
	}

	cluster1 := newCluster("cluster1")
	rr := &searchv1alpha1.ResourceRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "rr"},
		Spec: searchv1alpha1.ResourceRegistrySpec{
			ResourceSelectors: []searchv1alpha1.ResourceSelector{
				podSelector,
			},
		},
	}

	kubeFactory := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0)
	karmadaFactory := karmadainformers.NewSharedInformerFactory(karmadafake.NewSimpleClientset(cluster1, rr), 0)

	ctrl, err := NewController(
		restConfig,
		restMapper,
		kubeFactory,
		karmadaFactory,
		0,
	)
	if err != nil {
		t.Error(err)
		return
	}
	if ctrl == nil {
		t.Error("ctrl is nil")
		return
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	kubeFactory.Start(stopCh)
	karmadaFactory.Start(stopCh)
	ctrl.Start(stopCh)
	defer ctrl.Stop()

	kubeFactory.WaitForCacheSync(stopCh)
	karmadaFactory.WaitForCacheSync(stopCh)
	// wait for controller synced
	time.Sleep(time.Second)

	hasPod := ctrl.store.HasResource(podGVR)
	if !hasPod {
		t.Error("has no pod resource")
		return
	}
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
		{
			name: "GetGroupVersionResource error shall be ignored",
			input: []runtime.Object{
				newCluster("cluster1"),
				&searchv1alpha1.ResourceRegistry{
					ObjectMeta: metav1.ObjectMeta{Name: "rr1"},
					Spec: searchv1alpha1.ResourceRegistrySpec{
						TargetCluster: policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
						ResourceSelectors: []searchv1alpha1.ResourceSelector{
							{APIVersion: "test.nonexist.group", Kind: "nonexist"},
						},
					},
				},
			},
			want: map[string]string{},
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

func TestController_Connect(t *testing.T) {
	var karmadaProxying, clusterProxying, cacheProxying bool
	ctl := &Controller{
		karmadaProxy: connectFunc(func(context.Context, schema.GroupVersionResource, string, rest.Responder) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				karmadaProxying = true
			}), nil
		}),
		cacheProxy: connectFunc(func(context.Context, schema.GroupVersionResource, string, rest.Responder) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				cacheProxying = true
			}), nil
		}),
		clusterProxy: connectFunc(func(context.Context, schema.GroupVersionResource, string, rest.Responder) (http.Handler, error) {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				clusterProxying = true
			}), nil
		}),
		store: &cacheFuncs{
			HasResourceFunc: func(gvr schema.GroupVersionResource) bool { return gvr == podGVR },
		},
	}

	type args struct {
		path string
	}
	type want struct {
		karmadaProxying, clusterProxying, cacheProxying bool
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get api from karmada",
			args: args{
				path: "/api",
			},
			want: want{
				karmadaProxying: true,
			},
		},
		{
			name: "get event api karmada",
			args: args{
				path: "/apis/events.k8s.io/v1",
			},
			want: want{
				karmadaProxying: true,
			},
		},
		{
			name: "list nodes from karmada",
			args: args{
				path: "/api/v1/nodes",
			},
			want: want{
				karmadaProxying: true,
			},
		},
		{
			name: "get node from karmada",
			args: args{
				path: "/api/v1/nodes",
			},
			want: want{
				karmadaProxying: true,
			},
		},
		{
			name: "list pod from cache",
			args: args{
				path: "/api/v1/pods",
			},
			want: want{
				cacheProxying: true,
			},
		},
		{
			name: "list pod from cache with namespace",
			args: args{
				path: "/api/v1/namespaces/default/pods",
			},
			want: want{
				cacheProxying: true,
			},
		},
		{
			name: "get pod from cache",
			args: args{
				path: "/api/v1/namespaces/default/pods/foo",
			},
			want: want{
				cacheProxying: true,
			},
		},
		{
			name: "get pod log from cluster",
			args: args{
				path: "/api/v1/namespaces/default/pods/foo/log",
			},
			want: want{
				clusterProxying: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			karmadaProxying, clusterProxying, cacheProxying = false, false, false
			conn, err := ctl.Connect(context.TODO(), tt.args.path, nil)
			if err != nil {
				t.Error(err)
				return
			}
			req, err := http.NewRequest("GET", "/prefix"+tt.args.path, nil)
			if err != nil {
				t.Error(err)
				return
			}
			conn.ServeHTTP(httptest.NewRecorder(), req)

			if karmadaProxying != tt.want.karmadaProxying {
				t.Errorf("karmadaProxying get = %v, want = %v", karmadaProxying, tt.want.karmadaProxying)
			}
			if cacheProxying != tt.want.cacheProxying {
				t.Errorf("cacheProxying get = %v, want = %v", cacheProxying, tt.want.cacheProxying)
			}
			if clusterProxying != tt.want.clusterProxying {
				t.Errorf("clusterProxying get = %v, want = %v", clusterProxying, tt.want.clusterProxying)
			}
		})
	}
}

func TestController_Connect_Error(t *testing.T) {
	ctl := &Controller{
		karmadaProxy: connectFunc(func(context.Context, schema.GroupVersionResource, string, rest.Responder) (http.Handler, error) {
			return nil, fmt.Errorf("test")
		}),
		negotiatedSerializer: scheme.Codecs.WithoutConversion(),
	}

	h, err := ctl.Connect(context.TODO(), "/api", nil)
	if err != nil {
		t.Error(err)
		return
	}

	response := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api", nil)
	if err != nil {
		t.Error(err)
		return
	}
	req.Header = make(http.Header)
	req.Header.Add("Accept", "application/json")
	h.ServeHTTP(response, req)
	wantBody := `{"kind":"Status","apiVersion":"get","metadata":{},"status":"Failure","message":"test","code":500}` + "\n"
	gotBody := response.Body.String()
	if wantBody != gotBody {
		t.Errorf("got body: %v", diff.StringDiff(gotBody, wantBody))
	}
}

func TestController_dynamicClientForCluster(t *testing.T) {
	// copy from go/src/net/http/internal/testcert/testcert.go
	testCA := []byte(`-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIQSRJrEpBGFc7tNb1fb5pKFzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA6Gba5tHV1dAKouAaXO3/ebDUU4rvwCUg/CNaJ2PT5xLD4N1Vcb8r
bFSW2HXKq+MPfVdwIKR/1DczEoAGf/JWQTW7EgzlXrCd3rlajEX2D73faWJekD0U
aUgz5vtrTXZ90BQL7WvRICd7FlEZ6FPOcPlumiyNmzUqtwGhO+9ad1W5BqJaRI6P
YfouNkwR6Na4TzSj5BrqUfP0FwDizKSJ0XXmh8g8G9mtwxOSN3Ru1QFc61Xyeluk
POGKBV/q6RBNklTNe0gI8usUMlYyoC7ytppNMW7X2vodAelSu25jgx2anj9fDVZu
h7AXF5+4nJS4AAt0n1lNY7nGSsdZas8PbQIDAQABo4GIMIGFMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBStsdjh3/JCXXYlQryOrL4Sh7BW5TAuBgNVHREEJzAlggtleGFtcGxlLmNv
bYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAxWGI
5NhpF3nwwy/4yB4i/CwwSpLrWUa70NyhvprUBC50PxiXav1TeDzwzLx/o5HyNwsv
cxv3HdkLW59i/0SlJSrNnWdfZ19oTcS+6PtLoVyISgtyN6DpkKpdG1cOkW3Cy2P2
+tK/tKHRP1Y/Ra0RiDpOAmqn0gCOFGz8+lqDIor/T7MTpibL3IxqWfPrvfVRHL3B
grw/ZQTTIVjjh4JBSW3WyWgNo/ikC1lrVxzl4iPUGptxT36Cr7Zk2Bsg0XqwbOvK
5d+NTDREkSnUbie4GeutujmX3Dsx88UiV6UY/4lHJa6I5leHUNOHahRbpbWeOfs/
WkBKOclmOV2xlTVuPw==
-----END CERTIFICATE-----`)

	type args struct {
		clusters []runtime.Object
		secrets  []runtime.Object
	}

	type want struct {
		err error
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "cluster not found",
			args: args{
				clusters: nil,
				secrets:  nil,
			},
			want: want{
				err: apierrors.NewNotFound(schema.GroupResource{Resource: "cluster", Group: "cluster.karmada.io"}, "test"),
			},
		},
		{
			name: "api endpoint is empty",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{},
				}},
				secrets: nil,
			},
			want: want{
				err: errors.New("the api endpoint of cluster test is empty"),
			},
		},
		{
			name: "secret is empty",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: "https://localhost",
					},
				}},
				secrets: nil,
			},
			want: want{
				err: errors.New("cluster test does not have a secret"),
			},
		},
		{
			name: "secret not found",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: "https://localhost",
						SecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "test_secret",
						},
					},
				}},
				secrets: nil,
			},
			want: want{
				err: errors.New(`secret "test_secret" not found`),
			},
		},
		{
			name: "token not found",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: "https://localhost",
						SecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "test_secret",
						},
					},
				}},
				secrets: []runtime.Object{&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test_secret",
					},
					Data: map[string][]byte{},
				}},
			},
			want: want{
				err: errors.New(`the secret for cluster test is missing a non-empty value for "token"`),
			},
		},
		{
			name: "success",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: "https://localhost",
						SecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "test_secret",
						},
					},
				}},
				secrets: []runtime.Object{&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test_secret",
					},
					Data: map[string][]byte{
						clusterv1alpha1.SecretTokenKey:  []byte("test_token"),
						clusterv1alpha1.SecretCADataKey: testCA,
					},
				}},
			},
			want: want{
				err: nil,
			},
		},
		{
			name: "has proxy",
			args: args{
				clusters: []runtime.Object{&clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: clusterv1alpha1.ClusterSpec{
						APIEndpoint: "https://localhost",
						SecretRef: &clusterv1alpha1.LocalSecretReference{
							Namespace: "default",
							Name:      "test_secret",
						},
						ProxyURL: "https://localhost",
					},
				}},
				secrets: []runtime.Object{&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "test_secret",
					},
					Data: map[string][]byte{
						clusterv1alpha1.SecretTokenKey:  []byte("test_token"),
						clusterv1alpha1.SecretCADataKey: testCA,
					},
				}},
			},
			want: want{
				err: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(tt.args.secrets...)
			kubeFactory := informers.NewSharedInformerFactory(kubeClient, 0)

			karmadaClient := karmadafake.NewSimpleClientset(tt.args.clusters...)
			karmadaFactory := karmadainformers.NewSharedInformerFactory(karmadaClient, 0)

			ctrl := &Controller{
				clusterLister: karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
				secretLister:  kubeFactory.Core().V1().Secrets().Lister(),
			}

			stopCh := make(chan struct{})
			defer close(stopCh)
			karmadaFactory.Start(stopCh)
			karmadaFactory.WaitForCacheSync(stopCh)
			kubeFactory.Start(stopCh)
			kubeFactory.WaitForCacheSync(stopCh)

			client, err := ctrl.dynamicClientForCluster("test")

			if !errorEquals(err, tt.want.err) {
				t.Errorf("got error %v, want %v", err, tt.want.err)
				return
			}

			if err != nil {
				return
			}
			if client == nil {
				t.Error("got client nil")
			}
		})
	}
}

func errorEquals(a, b error) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Error() == b.Error()
}

type connectFunc func(ctx context.Context, gvr schema.GroupVersionResource, proxyPath string, responder rest.Responder) (http.Handler, error)

func (c connectFunc) connect(ctx context.Context, gvr schema.GroupVersionResource, proxyPath string, responder rest.Responder) (http.Handler, error) {
	return c(ctx, gvr, proxyPath, responder)
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
