package utils

import (
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// RestConfig  Kubernetes kubeconfig
func RestConfig(context, kubeconfigPath string) (*rest.Config, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()

	loadingRules := *pathOptions.LoadingRules
	loadingRules.ExplicitPath = kubeconfigPath
	loadingRules.Precedence = pathOptions.GetLoadingPrecedence()
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&loadingRules, overrides)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restConfig, err
}

// NewClientSet Kubernetes ClientSet
func NewClientSet(c *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(c)
}

// NewCRDsClient clientset ClientSet
func NewCRDsClient(c *rest.Config) (*clientset.Clientset, error) {
	return clientset.NewForConfig(c)
}

// NewAPIRegistrationClient apiregistration ClientSet
func NewAPIRegistrationClient(c *rest.Config) (*aggregator.Clientset, error) {
	return aggregator.NewForConfig(c)
}
