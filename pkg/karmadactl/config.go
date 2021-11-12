/*
Copyright The Karmada Authors.

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

package karmadactl

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KarmadaConfig provides a rest config based on the filesystem kubeconfig (via
// pathOptions) and context in order to talk to the control plane
// and the joining kubernetes cluster.
type KarmadaConfig interface {
	// GetRestConfig used to get a cluster's rest config.
	GetRestConfig(context, kubeconfigPath string) (*rest.Config, error)

	// GetClientConfig returns a ClientConfig from kubeconfigPath.
	// If kubeconfigPath is empty, will search KUBECONFIG from default path.
	// If context is not empty, the returned ClientConfig's current-context is the input context.
	GetClientConfig(context, kubeconfigPath string) clientcmd.ClientConfig
}

// karmadaConfig implements the KarmadaConfig interface.
type karmadaConfig struct {
	pathOptions *clientcmd.PathOptions
}

// NewKarmadaConfig creates a karmadaConfig for `karmadactl` commands.
func NewKarmadaConfig(pathOptions *clientcmd.PathOptions) KarmadaConfig {
	return &karmadaConfig{
		pathOptions: pathOptions,
	}
}

func (a *karmadaConfig) GetRestConfig(context, kubeconfigPath string) (*rest.Config, error) {
	clientConfig := a.GetClientConfig(context, kubeconfigPath)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return restConfig, nil
}

// GetClientConfig is a helper method to create a client config from the
// context and kubeconfig passed as arguments.
func (a *karmadaConfig) GetClientConfig(context, kubeconfigPath string) clientcmd.ClientConfig {
	loadingRules := *a.pathOptions.LoadingRules
	loadingRules.Precedence = a.pathOptions.GetLoadingPrecedence()
	loadingRules.ExplicitPath = kubeconfigPath
	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&loadingRules, overrides)
}
