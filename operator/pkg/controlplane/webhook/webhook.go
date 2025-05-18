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

package webhook

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureKarmadaWebhook creates karmada webhook deployment and service resource.
func EnsureKarmadaWebhook(client clientset.Interface, cfg *operatorv1alpha1.KarmadaWebhook, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaWebhook(client, cfg, name, namespace, featureGates); err != nil {
		return err
	}

	return createKarmadaWebhookService(client, name, namespace)
}

func installKarmadaWebhook(client clientset.Interface, cfg *operatorv1alpha1.KarmadaWebhook, name, namespace string, featureGates map[string]bool) error {
	webhookDeploymentSetBytes, err := util.ParseTemplate(KarmadaWebhookDeployment, struct {
		KarmadaInstanceName, DeploymentName, Namespace, Image, ImagePullPolicy string
		KubeconfigSecret, WebhookCertsSecret                                   string
		Replicas                                                               *int32
	}{
		KarmadaInstanceName: name,
		DeploymentName:      util.KarmadaWebhookName(name),
		Namespace:           namespace,
		Image:               cfg.Image.Name(),
		ImagePullPolicy:     string(cfg.ImagePullPolicy),
		Replicas:            cfg.Replicas,
		KubeconfigSecret:    util.ComponentKarmadaConfigSecretName(util.KarmadaWebhookName(name)),
		WebhookCertsSecret:  util.WebhookCertSecretName(name),
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaWebhook Deployment template: %w", err)
	}

	webhookDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), webhookDeploymentSetBytes, webhookDeployment); err != nil {
		return fmt.Errorf("err when decoding KarmadaWebhook Deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithPriorityClassName(cfg.CommonSettings.PriorityClassName).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).WithResources(cfg.Resources).ForDeployment(webhookDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, webhookDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", webhookDeployment.Name, err)
	}
	return nil
}

func createKarmadaWebhookService(client clientset.Interface, name, namespace string) error {
	webhookServiceSetBytes, err := util.ParseTemplate(KarmadaWebhookService, struct {
		KarmadaInstanceName, ServiceName, Namespace string
	}{
		KarmadaInstanceName: name,
		ServiceName:         util.KarmadaWebhookName(name),
		Namespace:           namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaWebhook Service template: %w", err)
	}

	webhookService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), webhookServiceSetBytes, webhookService); err != nil {
		return fmt.Errorf("err when decoding KarmadaWebhook Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, webhookService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", webhookService.Name, err)
	}
	return nil
}
