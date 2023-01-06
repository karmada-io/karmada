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
)

// EnsureKarmadaWebhook creates karmada webhook deployment and service resource.
func EnsureKarmadaWebhook(client clientset.Interface, cfg *operatorv1alpha1.KarmadaWebhook, name, namespace string) error {
	if err := installKarmadaWebhook(client, cfg, name, namespace); err != nil {
		return err
	}

	return createKarmadaWebhookService(client, name, namespace)
}

func installKarmadaWebhook(client clientset.Interface, cfg *operatorv1alpha1.KarmadaWebhook, name, namespace string) error {
	webhookDeploymentSetBytes, err := util.ParseTemplate(KarmadaWebhookDeployment, struct {
		DeploymentName, Namespace, Image     string
		KubeconfigSecret, WebhookCertsSecret string
		Replicas                             *int32
	}{
		DeploymentName:     util.KarmadaWebhookName(name),
		Namespace:          namespace,
		Image:              cfg.Image.Name(),
		Replicas:           cfg.Replicas,
		KubeconfigSecret:   util.AdminKubeconfigSercretName(name),
		WebhookCertsSecret: util.WebhookCertSecretName(name),
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaWebhook Deployment template: %w", err)
	}

	webhookDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), webhookDeploymentSetBytes, webhookDeployment); err != nil {
		return fmt.Errorf("err when decoding KarmadaWebhook Deployment: %w", err)
	}

	if err := apiclient.CreateOrUpdateDeployment(client, webhookDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", webhookDeployment.Name, err)
	}
	return nil
}

func createKarmadaWebhookService(client clientset.Interface, name, namespace string) error {
	webhookServiceSetBytes, err := util.ParseTemplate(KarmadaWebhookService, struct {
		ServiceName, Namespace string
	}{
		ServiceName: util.KarmadaWebhookName(name),
		Namespace:   namespace,
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
