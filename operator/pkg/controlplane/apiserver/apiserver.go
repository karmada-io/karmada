package apiserver

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureKarmadaAPIServer creates karmada apiserver deployment and service resource
func EnsureKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaAPIServer(client, cfg.KarmadaAPIServer, name, namespace, featureGates); err != nil {
		return fmt.Errorf("failed to install karmada apiserver, err: %w", err)
	}

	return createKarmadaAPIServerService(client, cfg.KarmadaAPIServer, name, namespace)
}

// EnsureKarmadaAggregatedAPIServer creates karmada aggregated apiserver deployment and service resource
func EnsureKarmadaAggregatedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string, featureGates map[string]bool) error {
	if err := installKarmadaAggregatedAPIServer(client, cfg.KarmadaAggregatedAPIServer, name, namespace, featureGates); err != nil {
		return err
	}
	return createKarmadaAggregatedAPIServerService(client, name, namespace)
}

func installKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAPIServer, name, namespace string, featureGates map[string]bool) error {
	apiserverDeploymentBytes, err := util.ParseTemplate(KarmadaApiserverDeployment, struct {
		DeploymentName, Namespace, Image, EtcdClientService string
		ServiceSubnet, KarmadaCertsSecret, EtcdCertsSecret  string
		Replicas                                            *int32
		EtcdListenClientPort                                int32
	}{
		DeploymentName:       util.KarmadaAPIServerName(name),
		Namespace:            namespace,
		Image:                cfg.Image.Name(),
		EtcdClientService:    util.KarmadaEtcdClientName(name),
		ServiceSubnet:        *cfg.ServiceSubnet,
		KarmadaCertsSecret:   util.KarmadaCertSecretName(name),
		EtcdCertsSecret:      util.EtcdCertSecretName(name),
		Replicas:             cfg.Replicas,
		EtcdListenClientPort: constants.EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaApiserver deployment template: %w", err)
	}

	apiserverDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverDeploymentBytes, apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver deployment: %w", err)
	}
	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithExtraArgs(cfg.ExtraArgs).ForDeployment(apiserverDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, apiserverDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", apiserverDeployment.Name, err)
	}
	return nil
}

func createKarmadaAPIServerService(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAPIServer, name, namespace string) error {
	karmadaApiserverServiceBytes, err := util.ParseTemplate(KarmadaApiserverService, struct {
		ServiceName, Namespace, ServiceType string
	}{
		ServiceName: util.KarmadaAPIServerName(name),
		Namespace:   namespace,
		ServiceType: string(cfg.ServiceType),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaApiserver serive template: %w", err)
	}

	karmadaApiserverService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaApiserverServiceBytes, karmadaApiserverService); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver serive: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, karmadaApiserverService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", karmadaApiserverService.Name, err)
	}
	return nil
}

func installKarmadaAggregatedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAggregatedAPIServer, name, namespace string, featureGates map[string]bool) error {
	aggregatedAPIServerDeploymentBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerDeployment, struct {
		DeploymentName, Namespace, Image, EtcdClientService   string
		KubeconfigSecret, KarmadaCertsSecret, EtcdCertsSecret string
		Replicas                                              *int32
		EtcdListenClientPort                                  int32
	}{
		DeploymentName:       util.KarmadaAggregatedAPIServerName(name),
		Namespace:            namespace,
		Image:                cfg.Image.Name(),
		EtcdClientService:    util.KarmadaEtcdClientName(name),
		KubeconfigSecret:     util.AdminKubeconfigSecretName(name),
		KarmadaCertsSecret:   util.KarmadaCertSecretName(name),
		EtcdCertsSecret:      util.EtcdCertSecretName(name),
		Replicas:             cfg.Replicas,
		EtcdListenClientPort: constants.EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaAggregatedAPIServer deployment template: %w", err)
	}

	aggregatedAPIServerDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedAPIServerDeploymentBytes, aggregatedAPIServerDeployment); err != nil {
		return fmt.Errorf("err when decoding karmadaApiserver deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).ForDeployment(aggregatedAPIServerDeployment)

	if err := apiclient.CreateOrUpdateDeployment(client, aggregatedAPIServerDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", aggregatedAPIServerDeployment.Name, err)
	}
	return nil
}

func createKarmadaAggregatedAPIServerService(client clientset.Interface, name, namespace string) error {
	aggregatedAPIServerServiceBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerService, struct {
		ServiceName, Namespace string
	}{
		ServiceName: util.KarmadaAggregatedAPIServerName(name),
		Namespace:   namespace,
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
