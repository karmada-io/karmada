package proxy

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	karmadainformers "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	proxytest "github.com/karmada-io/karmada/pkg/search/proxy/testing"
)

func TestClientFactory_dynamicClientForCluster(t *testing.T) {
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

			factory := &clientFactory{
				ClusterLister: karmadaFactory.Cluster().V1alpha1().Clusters().Lister(),
				SecretLister:  kubeFactory.Core().V1().Secrets().Lister(),
			}

			stopCh := make(chan struct{})
			defer close(stopCh)
			karmadaFactory.Start(stopCh)
			karmadaFactory.WaitForCacheSync(stopCh)
			kubeFactory.Start(stopCh)
			kubeFactory.WaitForCacheSync(stopCh)

			client, err := factory.DynamicClientForCluster("test")

			if !proxytest.ErrorMessageEquals(err, tt.want.err) {
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
