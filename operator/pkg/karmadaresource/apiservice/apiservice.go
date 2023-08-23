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
func EnsureAggregatedAPIService(aggregatorClient *aggregator.Clientset, client clientset.Interface, name, namespace string) error {
	if err := aggregatedApiserverService(client, name, namespace); err != nil {
		return err
	}

	return aggregatedAPIService(aggregatorClient, name, namespace)
}

func aggregatedAPIService(client *aggregator.Clientset, name, namespace string) error {
	apiServiceBytes, err := util.ParseTemplate(KarmadaAggregatedAPIService, struct {
		Namespace   string
		ServiceName string
	}{
		Namespace:   namespace,
		ServiceName: util.KarmadaAggregatedAPIServerName(name),
	})
	if err != nil {
		return fmt.Errorf("error when parsing AggregatedApiserver APIService template: %w", err)
	}

	apiService := &apiregistrationv1.APIService{}
	if err := runtime.DecodeInto(codecs.UniversalDecoder(), apiServiceBytes, apiService); err != nil {
		return fmt.Errorf("err when decoding AggregatedApiserver APIService: %w", err)
	}

	if err := apiclient.CreateOrUpdateAPIService(client, apiService); err != nil {
		return err
	}
	return nil
}

func aggregatedApiserverService(client clientset.Interface, name, namespace string) error {
	aggregatedApiserverServiceBytes, err := util.ParseTemplate(KarmadaAggregatedApiserverService, struct {
		Namespace   string
		ServiceName string
	}{
		Namespace:   namespace,
		ServiceName: util.KarmadaAggregatedAPIServerName(name),
	})
	if err != nil {
		return fmt.Errorf("error when parsing AggregatedApiserver Service template: %w", err)
	}

	aggregatedService := &corev1.Service{}
	if err := runtime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aggregatedApiserverServiceBytes, aggregatedService); err != nil {
		return fmt.Errorf("err when decoding AggregatedApiserver Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, aggregatedService); err != nil {
		return err
	}
	return nil
}

// EnsureMetricsAdapterAPIService creates APIService and a service for karmada-metrics-adapter
func EnsureMetricsAdapterAPIService(aggregatorClient *aggregator.Clientset, client clientset.Interface, name, namespace string) error {
	if err := karmadaMetricsAdapterService(client, name, namespace); err != nil {
		return err
	}

	return karmadaMetricsAdapterAPIService(aggregatorClient, name, namespace)
}

func karmadaMetricsAdapterAPIService(client *aggregator.Clientset, name, namespace string) error {
	for _, gv := range constants.KarmadaMetricsAdapterAPIServices {
		// The APIService name to metrics adapter is "$version.$group"
		apiServiceName := fmt.Sprintf("%s.%s", gv.Version, gv.Group)

		apiServiceBytes, err := util.ParseTemplate(KarmadaMetricsAdapterAPIService, struct {
			Name, Namespace             string
			ServiceName, Group, Version string
		}{
			Name:        apiServiceName,
			Namespace:   namespace,
			Group:       gv.Group,
			Version:     gv.Version,
			ServiceName: util.KarmadaMetricsAdapterName(name),
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

func karmadaMetricsAdapterService(client clientset.Interface, name, namespace string) error {
	aggregatedApiserverServiceBytes, err := util.ParseTemplate(KarmadaMetricsAdapterService, struct {
		Namespace   string
		ServiceName string
	}{
		Namespace:   namespace,
		ServiceName: util.KarmadaMetricsAdapterName(name),
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
