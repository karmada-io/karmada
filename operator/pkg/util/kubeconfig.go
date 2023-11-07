package util

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

// CreateWithCerts creates a KubeConfig object with access to the API server with client certificates
func CreateWithCerts(serverURL, clusterName, userName string, caCert []byte, clientKey []byte, clientCert []byte) *clientcmdapi.Config {
	config := CreateBasic(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		ClientKeyData:         clientKey,
		ClientCertificateData: clientCert,
	}
	return config
}

// CreateBasic creates a basic, general KubeConfig object that then can be extended
func CreateBasic(serverURL, clusterName, userName string, caCert []byte) *clientcmdapi.Config {
	// Use the cluster and the username as the context name
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
}

// IsInCluster returns a bool represents whether the remote cluster is the local or not.
func IsInCluster(hostCluster *operatorv1alpha1.HostCluster) bool {
	return hostCluster == nil || hostCluster.SecretRef == nil || len(hostCluster.SecretRef.Name) == 0
}

func BuildClientFromSecretRef(client *clientset.Clientset, ref *operatorv1alpha1.LocalSecretReference) (*clientset.Clientset, error) {
	secret, err := client.CoreV1().Secrets(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	kubeconfigBytes, ok := secret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("the kubeconfig or data key 'kubeconfig' is not found, please check the secret %s/%s", secret.Namespace, secret.Name)
	}

	return newClientSetForConfig(kubeconfigBytes)
}

func newClientSetForConfig(kubeconfig []byte) (*clientset.Clientset, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}
