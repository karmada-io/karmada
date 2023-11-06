package apiservice

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
)

func init() {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(apiregistrationv1.AddToScheme(scheme))
}

// EnsureAggregatedAPIService creates aggregated APIService and a service
func EnsureAggregatedAPIService(aggregatorClient *aggregator.Clientset, client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace, caBundle string) error {
	if err := aggregatedApiserverService(client, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace); err != nil {
		return err
	}

	return aggregatedAPIService(aggregatorClient, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle)
}

func aggregatedAPIService(client *aggregator.Clientset, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle string) error {
	apiServiceBytes, err := util.ParseTemplate(KarmadaAggregatedAPIService, struct {
		Namespace   string
		ServiceName string
		CABundle    string
	}{
		Namespace:   karmadaControlPlaneNamespace,
		ServiceName: util.KarmadaAggregatedAPIServerName(karmadaControlPlaneServiceName),
		CABundle:    caBundle,
	})
	if err != nil {
		return fmt.Errorf("error when parsing AggregatedApiserver APIService template: %w", err)
	}

	apiService := &apiregistrationv1.APIService{}
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), apiServiceBytes, apiService); err != nil {
		return fmt.Errorf("err when decoding AggregatedApiserver APIService: %w", err)
	}

	return apiclient.CreateOrUpdateAPIService(client, apiService)
}

func aggregatedApiserverService(client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace string) error {
	aggregatedApiserverServiceBytes, err := util.ParseTemplate(KarmadaAggregatedApiserverService, struct {
		Namespace              string
		ServiceName            string
		HostClusterServiceName string
		HostClusterNamespace   string
	}{
		Namespace:              karmadaControlPlaneNamespace,
		ServiceName:            util.KarmadaAggregatedAPIServerName(karmadaControlPlaneServiceName),
		HostClusterServiceName: util.KarmadaAggregatedAPIServerName(hostClusterServiceName),
		HostClusterNamespace:   hostClusterNamespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing AggregatedApiserver Service template: %w", err)
	}

	aggregatedService := &corev1.Service{}
	if err := runtime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedApiserverServiceBytes, aggregatedService); err != nil {
		return fmt.Errorf("err when decoding AggregatedApiserver Service: %w", err)
	}

	return apiclient.CreateOrUpdateService(client, aggregatedService)
}

// EnsureMetricsAdapterAPIService creates APIService and a service for karmada-metrics-adapter
func EnsureMetricsAdapterAPIService(aggregatorClient *aggregator.Clientset, client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace, caBundle string) error {
	if err := karmadaMetricsAdapterService(client, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace); err != nil {
		return err
	}

	return karmadaMetricsAdapterAPIService(aggregatorClient, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle)
}

func karmadaMetricsAdapterAPIService(client *aggregator.Clientset, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle string) error {
	for _, gv := range constants.KarmadaMetricsAdapterAPIServices {
		// The APIService name to metrics adapter is "$version.$group"
		apiServiceName := fmt.Sprintf("%s.%s", gv.Version, gv.Group)

		apiServiceBytes, err := util.ParseTemplate(KarmadaMetricsAdapterAPIService, struct {
			Name, Namespace             string
			ServiceName, Group, Version string
			CABundle                    string
		}{
			Name:        apiServiceName,
			Namespace:   karmadaControlPlaneNamespace,
			Group:       gv.Group,
			Version:     gv.Version,
			ServiceName: util.KarmadaMetricsAdapterName(karmadaControlPlaneServiceName),
			CABundle:    caBundle,
		})
		if err != nil {
			return fmt.Errorf("error when parsing KarmadaMetricsAdapter APIService %s template: %w", apiServiceName, err)
		}

		apiService := &apiregistrationv1.APIService{}
		if err := runtime.DecodeInto(codecs.UniversalDecoder(), apiServiceBytes, apiService); err != nil {
			return fmt.Errorf("err when decoding KarmadaMetricsAdapter APIService %s: %w", apiServiceName, err)
		}

		if err := apiclient.CreateOrUpdateAPIService(client, apiService); err != nil {
			return err
		}
	}

	return nil
}

func karmadaMetricsAdapterService(client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace string) error {
	aggregatedApiserverServiceBytes, err := util.ParseTemplate(KarmadaMetricsAdapterService, struct {
		Namespace              string
		ServiceName            string
		HostClusterServiceName string
		HostClusterNamespace   string
	}{
		Namespace:              karmadaControlPlaneNamespace,
		ServiceName:            util.KarmadaMetricsAdapterName(karmadaControlPlaneServiceName),
		HostClusterServiceName: util.KarmadaMetricsAdapterName(hostClusterServiceName),
		HostClusterNamespace:   hostClusterNamespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaMetricsAdapter Service template: %w", err)
	}

	aggregatedService := &corev1.Service{}
	if err := runtime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedApiserverServiceBytes, aggregatedService); err != nil {
		return fmt.Errorf("err when decoding KarmadaMetricsAdapter Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, aggregatedService); err != nil {
		return err
	}
	return nil
}
