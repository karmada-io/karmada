package constants

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
)

const (
	// KubeDefaultRepository defines the default of the k8s image repository
	KubeDefaultRepository = "registry.k8s.io"
	// KarmadaDefaultRepository defines the default of the karmada image repository
	KarmadaDefaultRepository = "docker.io/karmada"
	// EtcdDefaultVersion defines the default of the karmada etcd image tag
	EtcdDefaultVersion = "3.5.3-0"
	// KarmadaDefaultVersion defines the default of the karmada components image tag
	KarmadaDefaultVersion = "v1.4.0"
	// KubeDefaultVersion defines the default of the karmada apiserver and kubeControllerManager image tag
	KubeDefaultVersion = "v1.25.2"
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

	// KarmadaAPIserverComponent defines the name of karmada apiserver component
	KarmadaAPIserverComponent = "KarmadaAPIServer"
	// KarmadaAggregatedAPIServerComponent defines the name of karmada aggregated apiserver component
	KarmadaAggregatedAPIServerComponent = "KarmadaAggregatedAPIServer"
	// KubeControllerManagerComponent defines the name of kube controller manager component
	KubeControllerManagerComponent = "KubeControllerManager"
	// KarmadaControllerManagerComponent defines the name of karmada controller manager component
	KarmadaControllerManagerComponent = "KarmadaControllerManager"
	// KarmadaSchedulerComponent defines the name of karmada scheduler component
	KarmadaSchedulerComponent = "KarmadaScheduler"
	// KarmadaWebhookComponent defines the name of the karmada-webhook component
	KarmadaWebhookComponent = "KarmadaWebhook"

	// KarmadaOperatorLabelKeyName defines a label key used by all of resources created by karmada operator
	KarmadaOperatorLabelKeyName = "app.kubernetes.io/managed-by"
)

var (
	// KarmadaOperatorLabel defines the default labels in the resource create by karmada operator
	KarmadaOperatorLabel = labels.Set{KarmadaOperatorLabelKeyName: KarmadaOperator}
)
