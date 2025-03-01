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

package search

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/etcd"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureKarmadaSearch creates karmada search deployment and service resource.
func EnsureKarmadaSearch(client clientset.Interface, cfg *operatorv1alpha1.KarmadaSearch, etcdCfg *operatorv1alpha1.Etcd, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaSearch(client, cfg, etcdCfg, name, namespace, featureGates); err != nil {
		return err
	}

	return createKarmadaSearchService(client, name, namespace)
}

func installKarmadaSearch(client clientset.Interface, cfg *operatorv1alpha1.KarmadaSearch, etcdCfg *operatorv1alpha1.Etcd, name, namespace string, _ map[string]bool) error {
	searchDeploymentSetBytes, err := util.ParseTemplate(KarmadaSearchDeployment, struct {
		KarmadaInstanceName, DeploymentName, Namespace, Image, ImagePullPolicy, KarmadaCertsSecret string
		KubeconfigSecret                                                                           string
		Replicas                                                                                   *int32
	}{
		KarmadaInstanceName: name,
		DeploymentName:      util.KarmadaSearchName(name),
		Namespace:           namespace,
		Image:               cfg.Image.Name(),
		ImagePullPolicy:     string(cfg.ImagePullPolicy),
		KarmadaCertsSecret:  util.KarmadaCertSecretName(name),
		Replicas:            cfg.Replicas,
		KubeconfigSecret:    util.ComponentKarmadaConfigSecretName(util.KarmadaSearchName(name)),
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaSearch Deployment template: %w", err)
	}

	searchDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), searchDeploymentSetBytes, searchDeployment); err != nil {
		return fmt.Errorf("err when decoding KarmadaSearch Deployment: %w", err)
	}

	err = etcd.ConfigureClientCredentials(searchDeployment, etcdCfg, name, namespace)
	if err != nil {
		return err
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithPriorityClassName(cfg.CommonSettings.PriorityClassName).
		WithExtraArgs(cfg.ExtraArgs).WithResources(cfg.Resources).ForDeployment(searchDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, searchDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", searchDeployment.Name, err)
	}
	return nil
}

func createKarmadaSearchService(client clientset.Interface, name, namespace string) error {
	searchServiceSetBytes, err := util.ParseTemplate(KarmadaSearchService, struct {
		KarmadaInstanceName, ServiceName, Namespace string
	}{
		KarmadaInstanceName: name,
		ServiceName:         util.KarmadaSearchName(name),
		Namespace:           namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaSearch Service template: %w", err)
	}

	searchService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), searchServiceSetBytes, searchService); err != nil {
		return fmt.Errorf("err when decoding KarmadaSearch Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, searchService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", searchService.Name, err)
	}
	return nil
}
