/*
Copyright 2021 The Karmada Authors.

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

package options

const (
	// KarmadaCertAndKeyName is the default admin kubeconfig client cert/key base name.
	KarmadaCertAndKeyName = "karmada"
	// EtcdCaCertAndKeyName is the etcd CA cert/key base name.
	EtcdCaCertAndKeyName = "etcd-ca"
	// FrontProxyCaCertAndKeyName is the front-proxy CA cert/key base name.
	FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// EtcdServerCertAndKeyName is the etcd server cert/key base name.
	EtcdServerCertAndKeyName = "etcd-server"
	// EtcdClientCertAndKeyName is the etcd client cert/key base name.
	EtcdClientCertAndKeyName = "etcd-client"
	// ApiserverCertAndKeyName is the legacy karmada apiserver cert/key base name.
	ApiserverCertAndKeyName = "apiserver"
	// FrontProxyClientCertAndKeyName is the front-proxy client cert/key base name.
	FrontProxyClientCertAndKeyName = "front-proxy-client"

	// KarmadaAPIServerCertAndKeyName is the split-layout server cert/key base name for karmada-apiserver.
	KarmadaAPIServerCertAndKeyName = "karmada-apiserver"
	// KarmadaAggregatedAPIServerCertAndKeyName is the split-layout server cert/key base name for karmada-aggregated-apiserver.
	KarmadaAggregatedAPIServerCertAndKeyName = "karmada-aggregated-apiserver"
	// KarmadaWebhookCertAndKeyName is the server cert/key base name for karmada-webhook.
	KarmadaWebhookCertAndKeyName = "karmada-webhook"
	// KarmadaSearchCertAndKeyName is the server cert/key base name for karmada-search.
	KarmadaSearchCertAndKeyName = "karmada-search"
	// KarmadaMetricsAdapterCertAndKeyName is the server cert/key base name for karmada-metrics-adapter.
	KarmadaMetricsAdapterCertAndKeyName = "karmada-metrics-adapter"
	// KarmadaSchedulerEstimatorCertAndKeyName is the server cert/key base name for karmada-scheduler-estimator.
	KarmadaSchedulerEstimatorCertAndKeyName = "karmada-scheduler-estimator"

	// KarmadaAPIServerClientCertAndKeyName is the client cert/key base name for karmada-apiserver.
	KarmadaAPIServerClientCertAndKeyName = "karmada-apiserver-client"
	// KarmadaAggregatedAPIServerClientCertAndKeyName is the client cert/key base name for karmada-aggregated-apiserver.
	KarmadaAggregatedAPIServerClientCertAndKeyName = "karmada-aggregated-apiserver-client"
	// KarmadaWebhookClientCertAndKeyName is the client cert/key base name for karmada-webhook.
	KarmadaWebhookClientCertAndKeyName = "karmada-webhook-client"
	// KarmadaSearchClientCertAndKeyName is the client cert/key base name for karmada-search.
	KarmadaSearchClientCertAndKeyName = "karmada-search-client"
	// KarmadaMetricsAdapterClientCertAndKeyName is the client cert/key base name for karmada-metrics-adapter.
	KarmadaMetricsAdapterClientCertAndKeyName = "karmada-metrics-adapter-client"
	// KarmadaSchedulerEstimatorClientCertAndKeyName is the client cert/key base name for karmada-scheduler-estimator.
	KarmadaSchedulerEstimatorClientCertAndKeyName = "karmada-scheduler-estimator-client"
	// KarmadaControllerManagerClientCertAndKeyName is the client cert/key base name for karmada-controller-manager.
	KarmadaControllerManagerClientCertAndKeyName = "karmada-controller-manager-client"
	// KubeControllerManagerClientCertAndKeyName is the client cert/key base name for kube-controller-manager.
	KubeControllerManagerClientCertAndKeyName = "kube-controller-manager-client"
	// KarmadaSchedulerClientCertAndKeyName is the client cert/key base name for karmada-scheduler.
	KarmadaSchedulerClientCertAndKeyName = "karmada-scheduler-client"
	// KarmadaDeschedulerClientCertAndKeyName is the client cert/key base name for karmada-descheduler.
	KarmadaDeschedulerClientCertAndKeyName = "karmada-descheduler-client"

	// KarmadaAPIServerEtcdClientCertAndKeyName is the etcd client cert/key base name for karmada-apiserver.
	KarmadaAPIServerEtcdClientCertAndKeyName = "karmada-apiserver-etcd-client"
	// KarmadaAggregatedAPIServerEtcdClientCertAndKeyName is the etcd client cert/key base name for karmada-aggregated-apiserver.
	KarmadaAggregatedAPIServerEtcdClientCertAndKeyName = "karmada-aggregated-apiserver-etcd-client"
	// KarmadaSearchEtcdClientCertAndKeyName is the etcd client cert/key base name for karmada-search.
	KarmadaSearchEtcdClientCertAndKeyName = "karmada-search-etcd-client"

	// KarmadaSchedulerGrpcCertAndKeyName is the gRPC client cert/key base name for karmada-scheduler.
	KarmadaSchedulerGrpcCertAndKeyName = "karmada-scheduler-grpc"
	// KarmadaDeschedulerGrpcCertAndKeyName is the gRPC client cert/key base name for karmada-descheduler.
	KarmadaDeschedulerGrpcCertAndKeyName = "karmada-descheduler-grpc"
	// ClusterName is the karmada cluster name.
	ClusterName = "karmada-apiserver"
	// UserName is the karmada cluster user name.
	UserName = "karmada-admin"
	// KarmadaKubeConfigName is the kubeconfig file name for karmada.
	KarmadaKubeConfigName = "karmada-apiserver.config"
	// WaitComponentReadyTimeout is the seconds to wait for components ready.
	WaitComponentReadyTimeout = 120
)

// CN (CommonName) for certificates
const (
	// KarmadaEtcdServerCN is the CommonName for etcd server certificate.
	KarmadaEtcdServerCN = "system:karmada:etcd-server"
	// KarmadaEtcdClientCN is the CommonName for etcd client certificate.
	KarmadaEtcdClientCN = "system:karmada:etcd-etcd-client"

	// KarmadaAPIServerCN is the CommonName for karmada-apiserver.
	KarmadaAPIServerCN = "system:karmada:karmada-apiserver"
	// KarmadaAPIServerEtcdClientCN is the CommonName for karmada-apiserver etcd client.
	KarmadaAPIServerEtcdClientCN = "system:karmada:karmada-apiserver-etcd-client"

	// KarmadaAggregatedAPIServerCN is the CommonName for karmada-aggregated-apiserver.
	KarmadaAggregatedAPIServerCN = "system:karmada:karmada-aggregated-apiserver"
	// KarmadaAggregatedAPIServerEtcdClientCN is the CommonName for karmada-aggregated-apiserver etcd client.
	KarmadaAggregatedAPIServerEtcdClientCN = "system:karmada:karmada-aggregated-apiserver-etcd-client"

	// KarmadaWebhookCN is the CommonName for karmada-webhook.
	KarmadaWebhookCN = "system:karmada:karmada-webhook"

	// KarmadaSearchCN is the CommonName for karmada-search.
	KarmadaSearchCN = "system:karmada:karmada-search"
	// KarmadaSearchEtcdClientCN is the CommonName for karmada-search etcd client.
	KarmadaSearchEtcdClientCN = "system:karmada:karmada-search-etcd-client"

	// KarmadaMetricsAdapterCN is the CommonName for karmada-metrics-adapter.
	KarmadaMetricsAdapterCN = "system:karmada:karmada-metrics-adapter"

	// KarmadaSchedulerEstimatorCN is the CommonName for karmada-scheduler-estimator.
	KarmadaSchedulerEstimatorCN = "system:karmada:karmada-scheduler-estimator"

	// KarmadaControllerManagerCN is the CommonName for karmada-controller-manager.
	KarmadaControllerManagerCN = "system:karmada:karmada-controller-manager"
	// KubeControllerManagerCN is the CommonName for kube-controller-manager.
	KubeControllerManagerCN = "system:karmada:kube-controller-manager"

	// KarmadaSchedulerCN is the CommonName for karmada-scheduler.
	KarmadaSchedulerCN = "system:karmada:karmada-scheduler"
	// KarmadaSchedulerGrpcCN is the CommonName for karmada-scheduler gRPC.
	KarmadaSchedulerGrpcCN = "system:karmada:karmada-scheduler-grpc"

	// KarmadaDeschedulerCN is the CommonName for karmada-descheduler.
	KarmadaDeschedulerCN = "system:karmada:karmada-descheduler"
	// KarmadaDeschedulerGrpcCN is the CommonName for karmada-descheduler gRPC.
	KarmadaDeschedulerGrpcCN = "system:karmada:karmada-descheduler-grpc"

	// KarmadaFrontProxyClientCN is the CommonName for the front-proxy client.
	KarmadaFrontProxyClientCN = "front-proxy-client"
)
