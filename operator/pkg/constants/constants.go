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

package constants

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// KubeDefaultRepository defines the default of the k8s image repository
	KubeDefaultRepository = "registry.k8s.io"
	// KarmadaDefaultRepository defines the default of the karmada image repository
	KarmadaDefaultRepository = "docker.io/karmada"
	// EtcdDefaultVersion defines the default of the karmada etcd image tag
	EtcdDefaultVersion = "3.5.13-0"
	// KubeDefaultVersion defines the default of the karmada apiserver and kubeControllerManager image tag
	KubeDefaultVersion = "v1.28.9"
	// KarmadaDefaultServiceSubnet defines the default of the subnet used by k8s services.
	KarmadaDefaultServiceSubnet = "10.96.0.0/12"
	// KarmadaDefaultDNSDomain defines the default of the DNSDomain
	KarmadaDefaultDNSDomain = "cluster.local"

	// KarmadaOperator defines the name of the karmada operator.
	KarmadaOperator = "karmada-operator"
	// Etcd defines the name of the built-in etcd cluster component
	Etcd = "etcd"
	// KarmadaAPIServer defines the name of the karmada-apiserver component
	KarmadaAPIServer = "karmada-apiserver"
	// KubeAPIServer defines the repository name of the kube apiserver
	KubeAPIServer = "kube-apiserver"
	// KarmadaAggregatedAPIServer defines the name of the karmada-aggregated-apiserver component
	KarmadaAggregatedAPIServer = "karmada-aggregated-apiserver"
	// KubeControllerManager defines the name of the kube-controller-manager component
	KubeControllerManager = "kube-controller-manager"
	// KarmadaControllerManager defines the name of the karmada-controller-manager component
	KarmadaControllerManager = "karmada-controller-manager"
	// KarmadaScheduler defines the name of the karmada-scheduler component
	KarmadaScheduler = "karmada-scheduler"
	// KarmadaWebhook defines the name of the karmada-webhook component
	KarmadaWebhook = "karmada-webhook"
	// KarmadaSearch defines the name of the karmada-search component
	KarmadaSearch = "karmada-search"
	// KarmadaDescheduler defines the name of the karmada-descheduler component
	KarmadaDescheduler = "karmada-descheduler"
	// KarmadaMetricsAdapter defines the name of the karmada-metrics-adapter component
	KarmadaMetricsAdapter = "karmada-metrics-adapter"

	// KarmadaSystemNamespace defines the leader selection namespace for karmada components
	KarmadaSystemNamespace = "karmada-system"
	// KarmadaDataDir defines the karmada data dir
	KarmadaDataDir = "/var/lib/karmada"

	// EtcdListenClientPort defines the port etcd listen on for client traffic
	EtcdListenClientPort = 2379
	// EtcdMetricsPort is the port at which to obtain etcd metrics and health status
	EtcdMetricsPort = 2381
	// EtcdListenPeerPort defines the port etcd listen on for peer traffic
	EtcdListenPeerPort = 2380
	// KarmadaAPIserverListenClientPort defines the port karmada apiserver listen on for client traffic
	KarmadaAPIserverListenClientPort = 5443
	// EtcdDataVolumeName defines the name to etcd data volume
	EtcdDataVolumeName = "etcd-data"

	// CertificateValidity Certificate validity period
	CertificateValidity = time.Hour * 24 * 365
	// CaCertAndKeyName ca certificate key name
	CaCertAndKeyName = "ca"
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

	// KarmadaAPIserverComponent defines the name of karmada-apiserver component
	KarmadaAPIserverComponent = "KarmadaAPIServer"
	// KarmadaAggregatedAPIServerComponent defines the name of karmada-aggregated-apiserver component
	KarmadaAggregatedAPIServerComponent = "KarmadaAggregatedAPIServer"
	// KubeControllerManagerComponent defines the name of kube-controller-manager-component
	KubeControllerManagerComponent = "KubeControllerManager"
	// KarmadaControllerManagerComponent defines the name of karmada-controller-manager component
	KarmadaControllerManagerComponent = "KarmadaControllerManager"
	// KarmadaSchedulerComponent defines the name of karmada-scheduler component
	KarmadaSchedulerComponent = "KarmadaScheduler"
	// KarmadaWebhookComponent defines the name of the karmada-webhook component
	KarmadaWebhookComponent = "KarmadaWebhook"
	// KarmadaSearchComponent defines the name of the karmada-search component
	KarmadaSearchComponent = "KarmadaSearch"
	// KarmadaDeschedulerComponent defines the name of the karmada-descheduler component
	KarmadaDeschedulerComponent = "KarmadaDescheduler"
	// KarmadaMetricsAdapterComponent defines the name of the karmada-metrics-adapter component
	KarmadaMetricsAdapterComponent = "KarmadaMetricsAdapter"

	// KarmadaOperatorLabelKeyName defines a label key used by all resources created by karmada operator
	KarmadaOperatorLabelKeyName = "app.kubernetes.io/managed-by"

	// APIServiceName defines the karmada aggregated apiserver APIService resource name.
	APIServiceName = "v1alpha1.cluster.karmada.io"
)

var (
	// KarmadaOperatorLabel defines the default labels in the resource create by karmada operator
	KarmadaOperatorLabel = labels.Set{KarmadaOperatorLabelKeyName: KarmadaOperator}

	// KarmadaMetricsAdapterAPIServices defines the GroupVersions of all karmada-metrics-adapter APIServices
	KarmadaMetricsAdapterAPIServices = []schema.GroupVersion{
		{Group: "metrics.k8s.io", Version: "v1beta1"},
		{Group: "custom.metrics.k8s.io", Version: "v1beta1"},
		{Group: "custom.metrics.k8s.io", Version: "v1beta2"},
	}

	// KarmadaSearchAPIServices defines the GroupVersions of all karmada-search APIServices
	KarmadaSearchAPIServices = []schema.GroupVersion{
		{Group: "search.karmada.io", Version: "v1alpha1"},
	}
)
