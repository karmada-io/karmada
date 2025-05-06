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

package apiserver

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/etcd"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureKarmadaAPIServer creates karmada apiserver deployment and service resource
func EnsureKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaAPIServer(client, cfg.KarmadaAPIServer, cfg.Etcd, name, namespace, featureGates); err != nil {
		return fmt.Errorf("failed to install karmada apiserver, err: %w", err)
	}

	return createKarmadaAPIServerService(client, cfg.KarmadaAPIServer, name, namespace)
}

// EnsureKarmadaAggregatedAPIServer creates karmada aggregated apiserver deployment and service resource
func EnsureKarmadaAggregatedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaAggregatedAPIServer(client, cfg.KarmadaAggregatedAPIServer, cfg.Etcd, name, namespace, featureGates); err != nil {
		return err
	}
	return createKarmadaAggregatedAPIServerService(client, name, namespace)
}

func installKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAPIServer, etcdCfg *operatorv1alpha1.Etcd, name, namespace string, _ map[string]bool) error {
	apiserverDeploymentBytes, err := util.ParseTemplate(KarmadaApiserverDeployment, struct {
		KarmadaInstanceName, DeploymentName, Namespace, Image, ImagePullPolicy string
		ServiceSubnet, KarmadaCertsSecret                                      string
		Replicas                                                               *int32
	}{
		KarmadaInstanceName: name,
		DeploymentName:      util.KarmadaAPIServerName(name),
		Namespace:           namespace,
		Image:               cfg.Image.Name(),
		ImagePullPolicy:     string(cfg.ImagePullPolicy),
		ServiceSubnet:       *cfg.ServiceSubnet,
		KarmadaCertsSecret:  util.KarmadaCertSecretName(name),
		Replicas:            cfg.Replicas,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaApiserver deployment template: %w", err)
	}

	apiserverDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverDeploymentBytes, apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver deployment: %w", err)
	}

	err = etcd.ConfigureClientCredentials(apiserverDeployment, etcdCfg, name, namespace)
	if err != nil {
		return err
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithPriorityClassName(cfg.CommonSettings.PriorityClassName).
		WithExtraArgs(cfg.ExtraArgs).WithExtraVolumeMounts(cfg.ExtraVolumeMounts).
		WithExtraVolumes(cfg.ExtraVolumes).WithSidecarContainers(cfg.SidecarContainers).WithResources(cfg.Resources).ForDeployment(apiserverDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}

func createKarmadaAPIServerService(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAPIServer, name, namespace string) error {
	karmadaApiserverServiceBytes, err := util.ParseTemplate(KarmadaApiserverService, struct {
		KarmadaInstanceName, ServiceName, Namespace, ServiceType string
	}{
		KarmadaInstanceName: name,
		ServiceName:         util.KarmadaAPIServerName(name),
		Namespace:           namespace,
		ServiceType:         string(cfg.ServiceType),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaApiserver serive template: %w", err)
	}

	karmadaApiserverService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaApiserverServiceBytes, karmadaApiserverService); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver serive: %w", err)
	}

	// merge annotations with configuration of karmada apiserver.
	karmadaApiserverService.Annotations = labels.Merge(karmadaApiserverService.Annotations, cfg.ServiceAnnotations)

	// Set LoadBalancerClass if ServiceType is LoadBalancer and LoadBalancerClass is specified
	if karmadaApiserverService.Spec.Type == corev1.ServiceTypeLoadBalancer && cfg.LoadBalancerClass != nil {
		karmadaApiserverService.Spec.LoadBalancerClass = cfg.LoadBalancerClass
	}

	if err := apiclient.CreateOrUpdateService(client, karmadaApiserverService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", karmadaApiserverService.Name, err)
	}
	return nil
}

func installKarmadaAggregatedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAggregatedAPIServer, etcdCfg *operatorv1alpha1.Etcd, name, namespace string, featureGates map[string]bool) error {
	aggregatedAPIServerDeploymentBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerDeployment, struct {
		KarmadaInstanceName, DeploymentName, Namespace, Image, ImagePullPolicy string
		KubeconfigSecret, KarmadaCertsSecret                                   string
		Replicas                                                               *int32
	}{
		KarmadaInstanceName: name,
		DeploymentName:      util.KarmadaAggregatedAPIServerName(name),
		Namespace:           namespace,
		Image:               cfg.Image.Name(),
		ImagePullPolicy:     string(cfg.ImagePullPolicy),
		KubeconfigSecret:    util.ComponentKarmadaConfigSecretName(util.KarmadaAggregatedAPIServerName(name)),
		KarmadaCertsSecret:  util.KarmadaCertSecretName(name),
		Replicas:            cfg.Replicas,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaAggregatedAPIServer deployment template: %w", err)
	}

	aggregatedAPIServerDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedAPIServerDeploymentBytes, aggregatedAPIServerDeployment); err != nil {
		return fmt.Errorf("err when decoding karmadaApiserver deployment: %w", err)
	}

	err = etcd.ConfigureClientCredentials(aggregatedAPIServerDeployment, etcdCfg, name, namespace)
	if err != nil {
		return err
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithPriorityClassName(cfg.CommonSettings.PriorityClassName).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).WithResources(cfg.Resources).ForDeployment(aggregatedAPIServerDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, aggregatedAPIServerDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", aggregatedAPIServerDeployment.Name, err)
	}
	return nil
}

func createKarmadaAggregatedAPIServerService(client clientset.Interface, name, namespace string) error {
	aggregatedAPIServerServiceBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerService, struct {
		KarmadaInstanceName, ServiceName, Namespace string
	}{
		KarmadaInstanceName: name,
		ServiceName:         util.KarmadaAggregatedAPIServerName(name),
		Namespace:           namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaAggregatedAPIServer serive template: %w", err)
	}

	aggregatedAPIServerService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedAPIServerServiceBytes, aggregatedAPIServerService); err != nil {
		return fmt.Errorf("err when decoding karmadaAggregatedAPIServer serive: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, aggregatedAPIServerService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", aggregatedAPIServerService.Name, err)
	}
	return nil
}
