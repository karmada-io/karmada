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
    // CA certificates
    // KarmadaCertAndKeyName karmada certificate key name
    KarmadaCertAndKeyName = "karmada"
    // EtcdCaCertAndKeyName etcd ca certificate key name
    EtcdCaCertAndKeyName = "etcd-ca"
    // FrontProxyCaCertAndKeyName front-proxy-ca certificate key name
    FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
    // EtcdClientCertAndKeyName etcd client certificate key name
    EtcdClientCertAndKeyName = "etcd-client"
    // ApiserverCertAndKeyName karmada apiserver certificate key name
    ApiserverCertAndKeyName = "apiserver"
    // FrontProxyClientCertAndKeyName front-proxy-client  certificate key name
    FrontProxyClientCertAndKeyName = "front-proxy-client"

    // Server certificates (split layout, independent per component)
    KarmadaApiServerCertAndKeyName           = "karmada-apiserver"
    KarmadaAggregatedApiServerCertAndKeyName = "karmada-aggregated-apiserver"
    KarmadaWebhookCertAndKeyName             = "karmada-webhook"
    KarmadaSearchCertAndKeyName              = "karmada-search"
    KarmadaMetricsAdapterCertAndKeyName      = "karmada-metrics-adapter"
    KarmadaSchedulerEstimatorCertAndKeyName  = "karmada-scheduler-estimator"

    // Client certificates (component clients)
    KarmadaApiServerClientCertAndKeyName           = "karmada-apiserver-client"
    KarmadaAggregatedApiServerClientCertAndKeyName = "karmada-aggregated-apiserver-client"
    KarmadaWebhookClientCertAndKeyName             = "karmada-webhook-client"
    KarmadaSearchClientCertAndKeyName              = "karmada-search-client"
    KarmadaMetricsAdapterClientCertAndKeyName      = "karmada-metrics-adapter-client"
    KarmadaSchedulerEstimatorClientCertAndKeyName  = "karmada-scheduler-estimator-client"
    KarmadaControllerManagerClientCertAndKeyName   = "karmada-controller-manager-client"
    KubeControllerManagerClientCertAndKeyName      = "kube-controller-manager-client"
    KarmadaSchedulerClientCertAndKeyName           = "karmada-scheduler-client"
    KarmadaDeschedulerClientCertAndKeyName         = "karmada-descheduler-client"

    // ETCD client certificates (component-specific etcd clients)
    KarmadaApiServerEtcdClientCertAndKeyName           = "karmada-apiserver-etcd-client"
    KarmadaAggregatedApiServerEtcdClientCertAndKeyName = "karmada-aggregated-apiserver-etcd-client"
    KarmadaSearchEtcdClientCertAndKeyName              = "karmada-search-etcd-client"

    // GRPC client certificates
    KarmadaSchedulerGrpcCertAndKeyName  = "karmada-scheduler-grpc"
    KarmadaDeschedulerGrpcCertAndKeyName = "karmada-descheduler-grpc"
    // ClusterName karmada cluster name
    ClusterName = "karmada-apiserver"
	// UserName karmada cluster user name
	UserName = "karmada-admin"
	// KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
	// WaitComponentReadyTimeout wait component ready time
	WaitComponentReadyTimeout = 120
)

// CN (CommonName) for certificates
const (
    KarmadaEtcdServerCN = "system:karmada:etcd-server"
    KarmadaEtcdClientCN = "system:karmada:etcd-etcd-client"

    KarmadaApiServerCN           = "system:karmada:karmada-apiserver"
    KarmadaApiServerEtcdClientCN = "system:karmada:karmada-apiserver-etcd-client"

    KarmadaAggregatedApiServerCN           = "system:karmada:karmada-aggregated-apiserver"
    KarmadaAggregatedApiServerEtcdClientCN = "system:karmada:karmada-aggregated-apiserver-etcd-client"

    KarmadaWebhookCN = "system:karmada:karmada-webhook"

    KarmadaSearchCN           = "system:karmada:karmada-search"
    KarmadaSearchEtcdClientCN = "system:karmada:karmada-search-etcd-client"

    KarmadaMetricsAdapterCN = "system:karmada:karmada-metrics-adapter"

    KarmadaSchedulerEstimatorCN = "system:karmada:karmada-scheduler-estimator"

    KarmadaControllerManagerCN = "system:karmada:karmada-controller-manager"
    KubeControllerManagerCN    = "system:karmada:kube-controller-manager"

    KarmadaSchedulerCN     = "system:karmada:karmada-scheduler"
    KarmadaSchedulerGrpcCN = "system:karmada:karmada-scheduler-grpc"

    KarmadaDeschedulerCN     = "system:karmada:karmada-descheduler"
    KarmadaDeschedulerGrpcCN = "system:karmada:karmada-descheduler-grpc"

    KarmadaFrontProxyClientCN = "front-proxy-client"
)
