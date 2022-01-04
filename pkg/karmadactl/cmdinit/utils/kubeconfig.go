package utils

import (
	"errors"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// CreateBasic creates a basic, general KubeConfig object that then can be extended
func CreateBasic(serverURL, userName, clusterName string, caCert []byte) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: clusterName,
	}
}

// CreateWithCerts creates a KubeConfig object with access to the API server with client certificates
func CreateWithCerts(serverURL, userName, clusterName string, caCert []byte, clientKey []byte, clientCert []byte) *clientcmdapi.Config {
	config := CreateBasic(serverURL, userName, clusterName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		ClientKeyData:         clientKey,
		ClientCertificateData: clientCert,
	}
	return config
}

// WriteKubeConfigFromSpec creates a kubeconfig object from a kubeConfigSpec and writes it to the given writer.
func WriteKubeConfigFromSpec(serverURL, userName, clusterName, kubeconfigPath, kubeconfigName string, caCert []byte, clientKey []byte, clientCert []byte) error {
	// builds the KubeConfig object
	config := CreateWithCerts(serverURL, userName, clusterName, caCert, clientKey, clientCert)

	// writes the kubeconfig to disk if it not exists
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return errors.New("failure while serializing admin kubeconfig")
	}

	return BytesToFile(kubeconfigPath, kubeconfigName, configBytes)
}
