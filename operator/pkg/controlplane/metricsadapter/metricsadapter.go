package metricsadapter

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

// EnsureKarmadaMetricAdapter creates karmada-metric-adapter deployment and service resource.
func EnsureKarmadaMetricAdapter(client clientset.Interface, cfg *operatorv1alpha1.KarmadaMetricsAdapter, name, namespace string) error {
	if err := installKarmadaMetricAdapter(client, cfg, name, namespace); err != nil {
		return err
	}

	return createKarmadaMetricAdapterService(client, name, namespace)
}

func installKarmadaMetricAdapter(client clientset.Interface, cfg *operatorv1alpha1.KarmadaMetricsAdapter, name, namespace string) error {
	metricAdapterBytes, err := util.ParseTemplate(KarmadaMetricsAdapterDeployment, struct {
		DeploymentName, Namespace, Image     string
		KubeconfigSecret, KarmadaCertsSecret string
		Replicas                             *int32
	}{
		DeploymentName:     util.KarmadaMetricsAdapterName(name),
		Namespace:          namespace,
		Image:              cfg.Image.Name(),
		Replicas:           cfg.Replicas,
		KubeconfigSecret:   util.AdminKubeconfigSecretName(name),
		KarmadaCertsSecret: util.KarmadaCertSecretName(name),
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaMetricAdapter Deployment template: %w", err)
	}

	metricAdapter := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), metricAdapterBytes, metricAdapter); err != nil {
		return fmt.Errorf("err when decoding KarmadaMetricAdapter Deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).WithResources(cfg.Resources).ForDeployment(metricAdapter)

	if err := apiclient.CreateOrUpdateDeployment(client, metricAdapter); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", metricAdapter.Name, err)
	}
	return nil
}

func createKarmadaMetricAdapterService(client clientset.Interface, name, namespace string) error {
	metricAdapterServiceBytes, err := util.ParseTemplate(KarmadaMetricsAdapterService, struct {
		ServiceName, Namespace string
	}{
		ServiceName: util.KarmadaMetricsAdapterName(name),
		Namespace:   namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaMetricAdapter Service template: %w", err)
	}

	metricAdapterService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), metricAdapterServiceBytes, metricAdapterService); err != nil {
		return fmt.Errorf("err when decoding KarmadaMetricAdapter Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, metricAdapterService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", metricAdapterService.Name, err)
	}
	return nil
}
