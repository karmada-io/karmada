package proxy

import (
	"context"
	"fmt"
	"net/http"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/registry/rest"
	listcorev1 "k8s.io/client-go/listers/core/v1"

	"github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/proxy"
)

// clusterProxy proxy to member clusters
type clusterProxy struct {
	secretLister listcorev1.SecretLister
}

func newClusterProxy(secretLister listcorev1.SecretLister) *clusterProxy {
	return &clusterProxy{
		secretLister: secretLister,
	}
}

func (c *clusterProxy) connect(ctx context.Context, cluster *v1alpha1.Cluster, proxyPath string, responder rest.Responder) (http.Handler, error) {
	location, transport, err := proxy.Location(cluster.Name, cluster.Spec.APIEndpoint, cluster.Spec.ProxyURL)
	if err != nil {
		return nil, err
	}
	location.Path = path.Join(location.Path, proxyPath)

	secretGetter := func(context.Context, string) (*corev1.Secret, error) {
		if cluster.Spec.ImpersonatorSecretRef == nil {
			return nil, fmt.Errorf("the impersonatorSecretRef of cluster %s is nil", cluster.Name)
		}
		return c.secretLister.Secrets(cluster.Spec.ImpersonatorSecretRef.Namespace).Get(cluster.Spec.ImpersonatorSecretRef.Name)
	}
	return proxy.ConnectCluster(ctx, cluster.Name, location, transport, responder, secretGetter)
}
