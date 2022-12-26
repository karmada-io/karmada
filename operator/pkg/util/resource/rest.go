package resource

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientConfigFromKubeConfigSecret reads a kubeconfig from the given namespace and secretName and
// returns a *rest.Config that the given user-agent is added.
func GetClientConfigFromKubeConfigSecret(client kubernetes.Interface, namespace, secretName, clientUserAgent string) (*restclient.Config, error) {
	kubeconfigSecret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	kubeconfig, ok := kubeconfigSecret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("the secret %s doesn't contain the kubeconfig field in the namespace %s", secretName, namespace)
	}

	config, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientConfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restclient.AddUserAgent(clientConfig, clientUserAgent), nil
}
