package resource

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClientConfigFromKubeConfigSecret reads a kubeconfig from the given namespace and secretName and
// returns a *rest.Config that the given user-agent is added.
func GetClientConfigFromKubeConfigSecret(c client.Client, namespace, secretName string) (*restclient.Config, error) {
	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      secretName,
		},
	}

	err := c.Get(context.TODO(), client.ObjectKeyFromObject(kubeconfigSecret), kubeconfigSecret)
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
	return config.ClientConfig()
}
