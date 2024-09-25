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
func EnsureAggregatedAPIService(aggregatorClient aggregator.Interface, client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace, caBundle string) error {
	if err := aggregatedApiserverService(client, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace); err != nil {
		return err
	}

	return aggregatedAPIService(aggregatorClient, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle)
}

func aggregatedAPIService(client aggregator.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle string) error {
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
func EnsureMetricsAdapterAPIService(aggregatorClient aggregator.Interface, client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace, caBundle string) error {
	if err := karmadaMetricsAdapterService(client, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace); err != nil {
		return err
	}

	return karmadaMetricsAdapterAPIService(aggregatorClient, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle)
}

func karmadaMetricsAdapterAPIService(client aggregator.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle string) error {
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

// EnsureSearchAPIService creates APIService and a service for karmada-metrics-adapter
func EnsureSearchAPIService(aggregatorClient aggregator.Interface, client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace, caBundle string) error {
	if err := karmadaSearchService(client, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace); err != nil {
		return err
	}

	return karmadaSearchAPIService(aggregatorClient, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle)
}

func karmadaSearchAPIService(client aggregator.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, caBundle string) error {
	apiServiceBytes, err := util.ParseTemplate(KarmadaSearchAPIService, struct {
		ServiceName, Namespace string
		CABundle               string
	}{
		Namespace:   karmadaControlPlaneNamespace,
		ServiceName: util.KarmadaSearchAPIServerName(karmadaControlPlaneServiceName),
		CABundle:    caBundle,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaSearch APIService template: %s", err.Error())
	}

	apiService := &apiregistrationv1.APIService{}
	if err = runtime.DecodeInto(codecs.UniversalDecoder(), apiServiceBytes, apiService); err != nil {
		return fmt.Errorf("err when decoding KarmadaSearch APIService: %s", err.Error())
	}

	if err = apiclient.CreateOrUpdateAPIService(client, apiService); err != nil {
		return err
	}

	return nil
}

func karmadaSearchService(client clientset.Interface, karmadaControlPlaneServiceName, karmadaControlPlaneNamespace, hostClusterServiceName, hostClusterNamespace string) error {
	searchApiserverServiceBytes, err := util.ParseTemplate(KarmadaSearchService, struct {
		Namespace              string
		ServiceName            string
		HostClusterServiceName string
		HostClusterNamespace   string
	}{
		Namespace:              karmadaControlPlaneNamespace,
		ServiceName:            util.KarmadaSearchName(karmadaControlPlaneServiceName),
		HostClusterServiceName: util.KarmadaSearchName(hostClusterServiceName),
		HostClusterNamespace:   hostClusterNamespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing KarmadaSearch Service template: %w", err)
	}

	searchService := &corev1.Service{}
	if err := runtime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), searchApiserverServiceBytes, searchService); err != nil {
		return fmt.Errorf("err when decoding KarmadaSearch Service: %w", err)
	}

	if err := apiclient.CreateOrUpdateService(client, searchService); err != nil {
		return err
	}
	return nil
}
