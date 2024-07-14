/*
Copyright 2020 The Karmada Authors.

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
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

// ClusterClient stands for a cluster Clientset for the given member cluster
type ClusterClient struct {
	KubeClient  *kubeclientset.Clientset
	ClusterName string
}

// DynamicClusterClient stands for a dynamic client for the given member cluster
type DynamicClusterClient struct {
	DynamicClientSet dynamic.Interface
	ClusterName      string
}

// ClusterScaleClient stands for a cluster ClientSet with scale client for the given member cluster
type ClusterScaleClient struct {
	KubeClient  *kubeclientset.Clientset
	ScaleClient scale.ScalesGetter
	ClusterName string
}

// Config holds the common attributes that can be passed to a Kubernetes client on
// initialization.

// ClientOption holds the attributes that should be injected to a Kubernetes client.
type ClientOption struct {
	// QPS indicates the maximum QPS to the master from this client.
	// If it's zero, the created RESTClient will use DefaultQPS: 5
	QPS float32

	// Burst indicates the maximum burst for throttle.
	// If it's zero, the created RESTClient will use DefaultBurst: 10.
	Burst int
}

// NewClusterScaleClientSet returns a ClusterScaleClient for the given member cluster.
func NewClusterScaleClientSet(clusterName string, client client.Client) (*ClusterScaleClient, error) {
	clusterConfig, err := BuildClusterConfig(clusterName, clusterGetter(client), secretGetter(client))
	if err != nil {
		return nil, err
	}

	var clusterScaleClientSet = ClusterScaleClient{ClusterName: clusterName}

	if clusterConfig != nil {
		hpaClient := kubeclientset.NewForConfigOrDie(clusterConfig)
		scaleKindResolver := scale.NewDiscoveryScaleKindResolver(hpaClient.Discovery())
		httpClient, err := rest.HTTPClientFor(clusterConfig)
		if err != nil {
			return nil, err
		}
		mapper, err := apiutil.NewDynamicRESTMapper(clusterConfig, httpClient)
		if err != nil {
			return nil, err
		}

		scaleClient, err := scale.NewForConfig(clusterConfig, mapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
		if err != nil {
			return nil, err
		}

		clusterScaleClientSet.KubeClient = hpaClient
		clusterScaleClientSet.ScaleClient = scaleClient
	}

	return &clusterScaleClientSet, nil
}

// NewClusterClientSet returns a ClusterClient for the given member cluster.
func NewClusterClientSet(clusterName string, client client.Client, clientOption *ClientOption) (*ClusterClient, error) {
	clusterConfig, err := BuildClusterConfig(clusterName, clusterGetter(client), secretGetter(client))
	if err != nil {
		return nil, err
	}

	var clusterClientSet = ClusterClient{ClusterName: clusterName}

	if clusterConfig != nil {
		if clientOption != nil {
			clusterConfig.QPS = clientOption.QPS
			clusterConfig.Burst = clientOption.Burst
		}
		clusterClientSet.KubeClient = kubeclientset.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterClientSetForAgent returns a ClusterClient for the given member cluster which will be used in karmada agent.
func NewClusterClientSetForAgent(clusterName string, _ client.Client, clientOption *ClientOption) (*ClusterClient, error) {
	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig of member cluster: %s", err.Error())
	}

	var clusterClientSet = ClusterClient{ClusterName: clusterName}

	if clusterConfig != nil {
		if clientOption != nil {
			clusterConfig.QPS = clientOption.QPS
			clusterConfig.Burst = clientOption.Burst
		}
		clusterClientSet.KubeClient = kubeclientset.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterDynamicClientSet returns a dynamic client for the given member cluster.
func NewClusterDynamicClientSet(clusterName string, client client.Client) (*DynamicClusterClient, error) {
	clusterConfig, err := BuildClusterConfig(clusterName, clusterGetter(client), secretGetter(client))
	if err != nil {
		return nil, err
	}
	var clusterClientSet = DynamicClusterClient{ClusterName: clusterName}

	if clusterConfig != nil {
		clusterClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// NewClusterDynamicClientSetForAgent returns a dynamic client for the given member cluster which will be used in karmada agent.
func NewClusterDynamicClientSetForAgent(clusterName string, _ client.Client) (*DynamicClusterClient, error) {
	clusterConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig of member cluster: %s", err.Error())
	}
	var clusterClientSet = DynamicClusterClient{ClusterName: clusterName}

	if clusterConfig != nil {
		clusterClientSet.DynamicClientSet = dynamic.NewForConfigOrDie(clusterConfig)
	}
	return &clusterClientSet, nil
}

// BuildClusterConfig return rest config for member cluster.
func BuildClusterConfig(clusterName string,
	clusterGetter func(string) (*clusterv1alpha1.Cluster, error),
	secretGetter func(string, string) (*corev1.Secret, error)) (*rest.Config, error) {
	cluster, err := clusterGetter(clusterName)
	if err != nil {
		return nil, err
	}

	apiEndpoint := cluster.Spec.APIEndpoint
	if apiEndpoint == "" {
		return nil, fmt.Errorf("the api endpoint of cluster %s is empty", clusterName)
	}

	if cluster.Spec.SecretRef == nil {
		return nil, fmt.Errorf("cluster %s does not have a secret", clusterName)
	}

	secret, err := secretGetter(cluster.Spec.SecretRef.Namespace, cluster.Spec.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	token, ok := secret.Data[clusterv1alpha1.SecretTokenKey]
	if !ok || len(token) == 0 {
		return nil, fmt.Errorf("the secret for cluster %s is missing a non-empty value for %q", clusterName, clusterv1alpha1.SecretTokenKey)
	}

	// Initialize cluster configuration.
	clusterConfig := &rest.Config{
		BearerToken: string(token),
		Host:        apiEndpoint,
	}

	// Handle TLS configuration.
	if cluster.Spec.InsecureSkipTLSVerification {
		clusterConfig.TLSClientConfig.Insecure = true
	} else {
		ca, ok := secret.Data[clusterv1alpha1.SecretCADataKey]
		if !ok {
			return nil, fmt.Errorf("the secret for cluster %s is missing the CA data key %q", clusterName, clusterv1alpha1.SecretCADataKey)
		}
		clusterConfig.TLSClientConfig = rest.TLSClientConfig{CAData: ca}
	}

	// Handle proxy configuration.
	if cluster.Spec.ProxyURL != "" {
		proxy, err := url.Parse(cluster.Spec.ProxyURL)
		if err != nil {
			klog.Errorf("parse proxy error. %v", err)
			return nil, err
		}
		clusterConfig.Proxy = http.ProxyURL(proxy)

		if len(cluster.Spec.ProxyHeader) != 0 {
			clusterConfig.Wrap(NewProxyHeaderRoundTripperWrapperConstructor(clusterConfig.WrapTransport, cluster.Spec.ProxyHeader))
		}
	}

	return clusterConfig, nil
}

func clusterGetter(client client.Client) func(string) (*clusterv1alpha1.Cluster, error) {
	return func(cluster string) (*clusterv1alpha1.Cluster, error) {
		return GetCluster(client, cluster)
	}
}

func secretGetter(client client.Client) func(string, string) (*corev1.Secret, error) {
	return func(namespace string, name string) (*corev1.Secret, error) {
		secret := &corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, secret)
		return secret, err
	}
}
