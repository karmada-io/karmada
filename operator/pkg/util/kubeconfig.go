/*
Copyright 2023 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// IsInCluster checks if the specified host cluster is the local cluster.
// It returns true if:
// - the hostCluster is nil;
// - or its SecretRef is nil;
// - or the SecretRef's Name is an empty string.
// This indicates that the remote cluster is either not configured or not identifiable as the local cluster.
func IsInCluster(hostCluster *operatorv1alpha1.HostCluster) bool {
	return hostCluster == nil || hostCluster.SecretRef == nil || len(hostCluster.SecretRef.Name) == 0
}

// BuildClientFromSecretRef builds a clientset from the secret reference.
func BuildClientFromSecretRef(client clientset.Interface, ref *operatorv1alpha1.LocalSecretReference) (clientset.Interface, error) {
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

func newClientSetForConfig(kubeconfig []byte) (clientset.Interface, error) {
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

// GetKubeConfigProxyURLFromSecretRef fetches secret by given LocalSecretReference, extracts kubeconfig data,
// then parses and returns proxy-url from the kubeconfig's current context cluster.
// client: Kubernetes clientset for accessing Secret resources
// ref: Reference to the secret storing kubeconfig data
// return: proxy-url string if exists, empty string when no proxy configured; error occurs on secret access or kubeconfig parse failure
func GetKubeConfigProxyURLFromSecretRef(client clientset.Interface, ref *operatorv1alpha1.LocalSecretReference) (string, error) {
	secret, err := client.CoreV1().Secrets(ref.Namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	kubeconfigBytes, ok := secret.Data["kubeconfig"]
	if !ok {
		return "", fmt.Errorf("the kubeconfig or data key 'kubeconfig' is not found, please check the secret %s/%s", secret.Namespace, secret.Name)
	}

	return GetKubeConfigProxyURL(kubeconfigBytes)
}

// GetKubeConfigProxyURL loads kubeconfig and returns the proxy-url from the cluster config of the current context
// clusterProxy: Returns proxy address if proxy exists, returns empty string if no proxy configured
// err: Only returns error when failing to parse kubeconfig or build client config; missing proxy is not treated as an error
func GetKubeConfigProxyURL(kubeConfig []byte) (clusterProxy string, err error) {
	rawCfg, err := clientcmd.Load(kubeConfig)
	if err != nil {
		return "", fmt.Errorf("parse kubeconfig yaml failed: %w", err)
	}

	clientCfg := clientcmd.NewDefaultClientConfig(*rawCfg, &clientcmd.ConfigOverrides{})
	restCfg, err := clientCfg.ClientConfig()
	if err != nil {
		return "", fmt.Errorf("generate rest config from kubeconfig failed: %w", err)
	}

	if restCfg.Proxy == nil {
		return "", nil
	}

	proxy, err := restCfg.Proxy(nil)
	if err != nil {
		return "", fmt.Errorf("resolve proxy url from rest config failed: %w", err)
	}

	if proxy == nil {
		return "", nil
	}

	clusterProxy = proxy.String()
	return clusterProxy, nil
}

// SetKubeConfigProxyURL takes raw kubeconfig yaml bytes and proxy URL, sets the proxy-url field, and returns the modified kubeconfig as []byte
// rawKubeConfigYaml: Raw yaml content of the original kubeconfig
// proxyURL: Proxy address, e.g. http://127.0.0.1:8080
func SetKubeConfigProxyURL(rawKubeConfigYaml []byte, proxyURL string) ([]byte, error) {
	config, err := clientcmd.Load(rawKubeConfigYaml)
	if err != nil {
		return nil, fmt.Errorf("load raw kubeconfig failed: %w", err)
	}

	for clusterName, cluster := range config.Clusters {
		cluster.ProxyURL = proxyURL
		config.Clusters[clusterName] = cluster
	}

	buf, err := clientcmd.Write(*config)
	if err != nil {
		return nil, fmt.Errorf("marshal kubeconfig to yaml failed: %w", err)
	}

	return buf, nil
}
