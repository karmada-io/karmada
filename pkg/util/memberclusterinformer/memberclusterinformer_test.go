package memberclusterinformer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func Test_getSingleClusterManager(t *testing.T) {
	clusterName := "cluster"
	cluster := testhelper.NewClusterWithTypeAndStatus(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	// Generate InformerManager
	stopCh := make(chan struct{})
	defer close(stopCh)

	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)

	tests := []struct {
		name                             string
		rightClusterName                 bool
		expectInformer                   bool
		expectError                      bool
		wrongClusterDynamicClientSetFunc bool
	}{
		{
			name:             "normal case",
			rightClusterName: true,
			expectInformer:   true,
			expectError:      false,
		},
		{
			name:                             "failed to build dynamic cluster client",
			rightClusterName:                 false,
			expectInformer:                   false,
			expectError:                      true,
			wrongClusterDynamicClientSetFunc: true,
		},
		{
			name:             "failed to get single cluster",
			rightClusterName: false,
			expectInformer:   true,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newMemberClusterInformer(cluster)
			m := genericmanager.NewMultiClusterInformerManager(stopCh)
			if tt.rightClusterName {
				m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
			} else {
				m.ForCluster("test", dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
			}
			m.Start(clusterName)
			m.WaitForCacheSync(clusterName)
			c.informerManager = m

			if tt.wrongClusterDynamicClientSetFunc {
				c.clusterDynamicClientSetFunc = NewClusterDynamicClientSetForAgentWithError
			} else {
				c.clusterDynamicClientSetFunc = util.NewClusterDynamicClientSet
				c.Client = fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint:                 "https://127.0.0.1",
							SecretRef:                   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							InsecureSkipTLSVerification: true,
						},
						Status: clusterv1alpha1.ClusterStatus{
							Conditions: []metav1.Condition{
								{
									Type:   clusterv1alpha1.ClusterConditionReady,
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token")},
					}).Build()
			}

			informerManager, err := c.getSingleClusterManager(cluster)

			if tt.expectInformer {
				assert.NotEmpty(t, informerManager)
			} else {
				assert.Empty(t, informerManager)
			}

			if tt.expectError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func Test_registerInformersAndStart(t *testing.T) {
	clusterName := "cluster"
	cluster := testhelper.NewClusterWithTypeAndStatus(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	// Generate InformerManager
	stopCh := make(chan struct{})
	defer close(stopCh)
	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	c := newMemberClusterInformer(cluster)

	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`)
	work := testhelper.NewWork("work", "default", raw)

	eventHandler := fedinformer.NewHandlerOnEvents(nil, nil, nil)

	t.Run("normal case", func(t *testing.T) {
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.informerManager = m

		err := c.registerInformersAndStart(cluster, work, eventHandler)
		assert.Empty(t, err)
	})

	t.Run("failed to getSingleClusterManager", func(t *testing.T) {
		c := newMemberClusterInformer(cluster)
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster("test", dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.informerManager = m
		c.clusterDynamicClientSetFunc = NewClusterDynamicClientSetForAgentWithError

		err := c.registerInformersAndStart(cluster, work, eventHandler)
		assert.NotEmpty(t, err)
	})

	t.Run("failed to getGVRsFromWork", func(t *testing.T) {
		work.Spec.Workload.Manifests[0].RawExtension.Raw = []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}},`)

		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.informerManager = m

		err := c.registerInformersAndStart(cluster, work, eventHandler)
		assert.NotEmpty(t, err)
	})
}

func NewClusterDynamicClientSetForAgentWithError(clusterName string, client client.Client) (*util.DynamicClusterClient, error) {
	return nil, fmt.Errorf("err")
}

func newMemberClusterInformer(cluster *clusterv1alpha1.Cluster) *memberClusterInformerImpl {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)

	return &memberClusterInformerImpl{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster).Build(),
		restMapper:                  mapper,
		clusterCacheSyncTimeout:     metav1.Duration{},
		clusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
	}
}
