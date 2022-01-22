package options

import (
	"fmt"
	"net"

	"github.com/spf13/pflag"

	"github.com/karmada-io/karmada/pkg/version"
)

const (
	// CaCertAndKeyName ca certificate key name
	CaCertAndKeyName = "ca"
	// EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
	// EtcdClientCertAndKeyName etcd client certificate key name
	EtcdClientCertAndKeyName = "etcd-client"
	// KarmadaCertAndKeyName karmada certificate key name
	KarmadaCertAndKeyName = "karmada"
	// FrontProxyCaCertAndKeyName front-proxy-client  certificate key name
	FrontProxyCaCertAndKeyName = "front-proxy-ca"
	// FrontProxyClientCertAndKeyName front-proxy-client  certificate key name
	FrontProxyClientCertAndKeyName = "front-proxy-client"
	// ClusterName karmada cluster name
	ClusterName = "karmada"
	// UserName karmada cluster user name
	UserName = "admin"
	// KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
)

var (
	// EtcdImage etcd image
	EtcdImage string
	// EtcdHostDataPath etcd data local path
	EtcdHostDataPath string
	// KarmadaAPIServerImage karmada apiserver image
	KarmadaAPIServerImage string
	// KarmadaSchedulerImage karmada scheduler image
	KarmadaSchedulerImage string
	// KubeControllerManagerImage k8s controller manager image
	KubeControllerManagerImage string
	// KarmadaControllerManagerImage karmada controller manager image
	KarmadaControllerManagerImage string
	// KarmadaWebhookImage karmada webhook image
	KarmadaWebhookImage string
	// KarmadaAggregatedAPIServerImage karmada aggregated apiserver image
	KarmadaAggregatedAPIServerImage string
	// KarmadaDataPath karmada data local path
	KarmadaDataPath string
	// CRDs karmada resource
	CRDs string
	// ExternalIP external ip of karmada certificate
	ExternalIP string
	// ExternalDNS external dns of karmada certificate
	ExternalDNS string
	// KarmadaAPIServerIP karmada apiserver ip
	KarmadaAPIServerIP []net.IP
)

// AddFlags adds flags to the specified FlagSet.
func AddFlags(flags *pflag.FlagSet) {
	// cert
	flags.StringVar(&ExternalIP, "cert-external-ip", "", "the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)")
	flags.StringVar(&ExternalDNS, "cert-external-dns", "", "the external DNS of Karmada certificate (e.g localhost,localhost.com)")
	// karmada
	crdURL := fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", version.Get().GitVersion)
	flags.StringVar(&CRDs, "crds", crdURL, "Karmada crds resource. local file e.g. --crds /root/crds.tar.gz")
	flags.StringVarP(&KarmadaDataPath, "karmada-data", "d", "/etc/karmada", "karmada data path. kubeconfig cert and crds files")
	flags.StringVarP(&KarmadaAPIServerImage, "karmada-apiserver-image", "", "k8s.gcr.io/kube-apiserver:v1.21.7", "Kubernetes apiserver image")
	flags.StringVarP(&KarmadaSchedulerImage, "karmada-scheduler-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-scheduler:latest", "karmada scheduler image")
	flags.StringVarP(&KubeControllerManagerImage, "karmada-kube-controller-manager-image", "", "k8s.gcr.io/kube-controller-manager:v1.21.7", "Kubernetes controller manager image")
	flags.StringVarP(&KarmadaControllerManagerImage, "karmada-controller-manager-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:latest", "karmada controller manager  image")
	flags.StringVarP(&KarmadaWebhookImage, "karmada-webhook-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-webhook:latest", "karmada webhook image")
	flags.StringVarP(&KarmadaAggregatedAPIServerImage, "karmada-aggregated-apiserver-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-aggregated-apiserver:latest", "karmada aggregated apiserver image")
	// etcd
	flags.StringVarP(&EtcdImage, "etcd-image", "", "k8s.gcr.io/etcd:3.5.1-0", "etcd image")
	flags.StringVarP(&EtcdHostDataPath, "etcd-data", "", "/var/lib/karmada-etcd", "etcd data path,valid in hostPath mode.")
}
