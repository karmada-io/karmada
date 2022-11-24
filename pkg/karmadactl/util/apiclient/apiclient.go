package apiclient

import (
	"os"
	"path/filepath"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

var (
	defaultKubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")

	// ErrEmptyConfig is the error message to be displayed if the configuration info is missing or incomplete
	ErrEmptyConfig = clientcmd.NewEmptyConfigError(
		`Missing or incomplete configuration info.  Please point to an existing, complete config file:
  1. Via the command-line flag --kubeconfig
  2. Via the KUBECONFIG environment variable
  3. In your home directory as ~/.kube/config
`)
)

// KubeConfigPath is to return kubeconfig file path in the following order:
// 1. Via the command-line flag --kubeconfig
// 2. Via the KUBECONFIG environment variable
// 3. In your home directory as ~/.kube/config
func KubeConfigPath(kubeconfigPath string) string {
	if kubeconfigPath == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			kubeconfigPath = env
		} else {
			kubeconfigPath = defaultKubeConfig
		}
	}

	return kubeconfigPath
}

// RestConfig is to create a rest config from the context and kubeconfig passed as arguments.
func RestConfig(context, kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			kubeconfigPath = env
		} else {
			kubeconfigPath = defaultKubeConfig
		}
	}
	if !Exists(kubeconfigPath) {
		return nil, ErrEmptyConfig
	}

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

// NewClientSet is to create a kubernetes ClientSet
func NewClientSet(c *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(c)
}

// NewCRDsClient is to create a clientset ClientSet
func NewCRDsClient(c *rest.Config) (*clientset.Clientset, error) {
	return clientset.NewForConfig(c)
}

// NewAPIRegistrationClient is to create an apiregistration ClientSet
func NewAPIRegistrationClient(c *rest.Config) (*aggregator.Clientset, error) {
	return aggregator.NewForConfig(c)
}

// Exists determine if path exists
func Exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err)
	}
	return true
}
