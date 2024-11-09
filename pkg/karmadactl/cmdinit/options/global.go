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
	// EtcdCaCertAndKeyName etcd ca certificate key name
	EtcdCaCertAndKeyName = "etcd-ca"
	// EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
	// EtcdClientCertAndKeyName etcd client certificate key name
	EtcdClientCertAndKeyName = "etcd-client"
	// KarmadaCertAndKeyName karmada certificate key name
	KarmadaCertAndKeyName = "karmada"
	// ApiserverCertAndKeyName karmada apiserver certificate key name
	ApiserverCertAndKeyName = "apiserver"
	// FrontProxyCaCertAndKeyName front-proxy-client  certificate key name
	FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// FrontProxyClientCertAndKeyName front-proxy-client  certificate key name
	FrontProxyClientCertAndKeyName = "front-proxy-client"
	// ClusterName karmada cluster name
	ClusterName = "karmada-apiserver"
	// UserName karmada cluster user name
	UserName = "karmada-admin"
	// KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
	// WaitComponentReadyTimeout wait component ready time
	WaitComponentReadyTimeout = 120

	// KarmadaConfigSecretSuffix karmada config secret suffix
	KarmadaConfigSecretSuffix = "-config" //nolint:gosec
	// KarmadaConfigFieldName the field stores karmada config in karmada config secret
	KarmadaConfigFieldName = "karmada.config" //nolint:gosec
	// KarmadaAggregatedApiserverConfig karmada config of karmada-aggregated-apiserver
	KarmadaAggregatedApiserverConfig = "karmada-aggregated-apiserver" + KarmadaConfigSecretSuffix
	// KarmadaControllerManagerConfig karmada config of karmada-controller-manager
	KarmadaControllerManagerConfig = "karmada-controller-manager" + KarmadaConfigSecretSuffix
	// KubeControllerManagerConfig karmada config of kube-controller-manager
	KubeControllerManagerConfig = "kube-controller-manager" + KarmadaConfigSecretSuffix
	// KarmadaSchedulerConfig karmada config of karmada-scheduler
	KarmadaSchedulerConfig = "karmada-scheduler" + KarmadaConfigSecretSuffix
	// KarmadaDeschedulerConfig karmada config of karmada-descheduler
	KarmadaDeschedulerConfig = "karmada-descheduler" + KarmadaConfigSecretSuffix
	// KarmadaMetricsAdapterConfig karmada config of karmada-metrics-adapter
	KarmadaMetricsAdapterConfig = "karmada-metrics-adapter" + KarmadaConfigSecretSuffix
	// KarmadaSearchConfig karmada config of karmada-search
	KarmadaSearchConfig = "karmada-search" + KarmadaConfigSecretSuffix
	// KarmadaWebhookConfig karmada config of karmada-webhook
	KarmadaWebhookConfig = "karmada-webhook" + KarmadaConfigSecretSuffix
)
