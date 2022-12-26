package resource

import (
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewBuilder is a helper function that accepts a rest.Config as an input parameter and
// returns a resource.Builder instance.
func NewBuilder(restConfig *rest.Config) *resource.Builder {
	return resource.NewBuilder(clientGetter{restConfig: restConfig})
}

// NewBuilder is a helper function that accepts a kubeconfig secret as an input parameter and
// returns a resource.Builder instance.
func NewBuilderFromKubeConfigSecret(client kubernetes.Interface, namespace, secretName, userAgent string) (*resource.Builder, error) {
	clientConfig, err := GetClientConfigFromKubeConfigSecret(client, namespace, secretName, userAgent)
	if err != nil {
		return nil, err
	}
	return NewBuilder(clientConfig), nil
}
