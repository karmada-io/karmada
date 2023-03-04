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
)

// EnsureKarmadaAPIServer creates karmada apiserver deployment and service resource
func EnsureKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string) error {
	if err := installKarmadaAPIServer(client, cfg.KarmadaAPIServer, name, namespace); err != nil {
		return err
	}

	return createKarmadaAPIServerService(client, cfg.KarmadaAPIServer, name, namespace)
}

// EnsureKarmadaAggregratedAPIServer creates karmada aggregated apiserver deployment and service resource
func EnsureKarmadaAggregratedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents, name, namespace string) error {
	if err := installKarmadaAggregratedAPIServer(client, cfg.KarmadaAggregratedAPIServer, name, namespace); err != nil {
		return err
	}
	return createKarmadaAggregratedAPIServerService(client, name, namespace)
}

func installKarmadaAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAPIServer, name, namespace string) error {
	apiserverDeploymentbytes, err := util.ParseTemplate(KarmadaApiserverDeployment, struct {
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
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), apiserverDeploymentbytes, apiserverDeployment); err != nil {
		return fmt.Errorf("error when decoding karmadaApiserver deployment: %w", err)
	}

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

func installKarmadaAggregratedAPIServer(client clientset.Interface, cfg *operatorv1alpha1.KarmadaAggregratedAPIServer, name, namespace string) error {
	aggregatedAPIServerDeploymentBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerDeployment, struct {
		DeploymentName, Namespace, Image, EtcdClientService   string
		KubeconfigSecret, KarmadaCertsSecret, EtcdCertsSecret string
		Replicas                                              *int32
		EtcdListenClientPort                                  int32
	}{
		DeploymentName:       util.KarmadaAggratedAPIServerName(name),
		Namespace:            namespace,
		Image:                cfg.Image.Name(),
		EtcdClientService:    util.KarmadaEtcdClientName(name),
		KubeconfigSecret:     util.AdminKubeconfigSercretName(name),
		KarmadaCertsSecret:   util.KarmadaCertSecretName(name),
		EtcdCertsSecret:      util.EtcdCertSecretName(name),
		Replicas:             cfg.Replicas,
		EtcdListenClientPort: constants.EtcdListenClientPort,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaAggregratedAPIServer deployment template: %w", err)
	}

	aggregratedAPIServerDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedAPIServerDeploymentBytes, aggregratedAPIServerDeployment); err != nil {
		return fmt.Errorf("err when decoding karmadaApiserver deployment: %w", err)
	}

	if err := apiclient.CreateOrUpdateDeployment(client, aggregratedAPIServerDeployment); err != nil {
		return fmt.Errorf("error when creating deployment for %s, err: %w", aggregratedAPIServerDeployment.Name, err)
	}
	return nil
}

func createKarmadaAggregratedAPIServerService(client clientset.Interface, name, namespace string) error {
	aggregatedAPIServerServiceBytes, err := util.ParseTemplate(KarmadaAggregatedAPIServerService, struct {
		ServiceName, Namespace string
	}{
		ServiceName: util.KarmadaAggratedAPIServerName(name),
		Namespace:   namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmadaAggregratedAPIServer serive template: %w", err)
	}

	aggregatedAPIServerService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedAPIServerServiceBytes, aggregatedAPIServerService); err != nil {
		return fmt.Errorf("err when decoding karmadaAggregratedAPIServer serive: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, aggregatedAPIServerService); err != nil {
		return fmt.Errorf("err when creating service for %s, err: %w", aggregatedAPIServerService.Name, err)
	}
	return nil
}
