/*
Copyright 2022 The Karmada Authors.

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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
)

const proxyURL = "/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/"

// The Factory interface provides 2 major features compared to the cmdutil.Factory:
// 1. provides a method to get a karmada clientset
// 2. provides a method to get a cmdutil.Factory for the member cluster
type Factory interface {
	cmdutil.Factory

	// KarmadaClientSet returns a karmada clientset
	KarmadaClientSet() (karmadaclientset.Interface, error)
	// FactoryForMemberCluster returns a cmdutil.Factory for the member cluster
	FactoryForMemberCluster(clusterName string) (cmdutil.Factory, error)
}

var _ Factory = &factoryImpl{}

// factoryImpl is the implementation of Factory
type factoryImpl struct {
	cmdutil.Factory

	// kubeConfigFlags holds all the flags specified by user.
	// These flags will be inherited by the member cluster's client.
	kubeConfigFlags *genericclioptions.ConfigFlags
}

// NewFactory returns a new factory
func NewFactory(kubeConfigFlags *genericclioptions.ConfigFlags) Factory {
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := &factoryImpl{
		kubeConfigFlags: kubeConfigFlags,
		Factory:         cmdutil.NewFactory(matchVersionKubeConfigFlags),
	}
	return f
}

// KarmadaClientSet returns a karmada clientset
func (f *factoryImpl) KarmadaClientSet() (karmadaclientset.Interface, error) {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return karmadaclientset.NewForConfig(clientConfig)
}

// FactoryForMemberCluster returns a cmdutil.Factory for the member cluster
func (f *factoryImpl) FactoryForMemberCluster(clusterName string) (cmdutil.Factory, error) {
	// Get client config of the karmada, and use it to create a cmdutil.Factory for the member cluster later.
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	karmadaAPIServer := clientConfig.Host

	// Check if the given cluster is joined to karmada
	client, err := karmadaclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	_, err = client.ClusterV1alpha1().Clusters().Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Inherit all properties from the original flags specified by user except for the kube-apiserver address.
	kubeConfigFlags := &genericclioptions.ConfigFlags{
		CacheDir:         f.kubeConfigFlags.CacheDir,
		KubeConfig:       f.kubeConfigFlags.KubeConfig,
		ClusterName:      f.kubeConfigFlags.ClusterName,
		AuthInfoName:     f.kubeConfigFlags.AuthInfoName,
		Context:          f.kubeConfigFlags.Context,
		Namespace:        f.kubeConfigFlags.Namespace,
		Insecure:         f.kubeConfigFlags.Insecure,
		CertFile:         f.kubeConfigFlags.CertFile,
		KeyFile:          f.kubeConfigFlags.KeyFile,
		CAFile:           f.kubeConfigFlags.CAFile,
		BearerToken:      f.kubeConfigFlags.BearerToken,
		Impersonate:      f.kubeConfigFlags.Impersonate,
		ImpersonateUID:   f.kubeConfigFlags.ImpersonateUID,
		ImpersonateGroup: f.kubeConfigFlags.ImpersonateGroup,
		Username:         f.kubeConfigFlags.Username,
		Password:         f.kubeConfigFlags.Password,
		Timeout:          f.kubeConfigFlags.Timeout,
		WrapConfigFn:     f.kubeConfigFlags.WrapConfigFn,
	}
	// Override the kube-apiserver address.
	memberAPIServer := karmadaAPIServer + fmt.Sprintf(proxyURL, clusterName)
	kubeConfigFlags.APIServer = &memberAPIServer
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionKubeConfigFlags), nil
}
