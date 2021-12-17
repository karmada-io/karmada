package options

import (
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
)

const (
	//CaCertAndKeyName ca certificate key name
	CaCertAndKeyName = "ca"
	//EtcdServerCertAndKeyName etcd server certificate key name
	EtcdServerCertAndKeyName = "etcd-server"
	//EtcdClientCertAndKeyName etcd client certificate key name
	EtcdClientCertAndKeyName = "etcd-client"
	//KarmadaCertAndKeyName karmada certificate key name
	KarmadaCertAndKeyName = "karmada"
	//ClusterName karmada cluster name
	ClusterName = "karmada"
	//UserName karmada cluster user name
	UserName = "admin"
	//KarmadaKubeConfigName karmada kubeconfig name
	KarmadaKubeConfigName = "karmada-apiserver.config"
)

var (
	//EtcdImage etcd image
	EtcdImage string
	//EtcdInitImage etcd init image
	EtcdInitImage string
	//APIServerImage karmada kube-apiserver image
	APIServerImage string
	//SchedulerImage karmada scheduler image
	SchedulerImage string
	//KubeControllerManagerImage karmada kube-controllermanager image
	KubeControllerManagerImage string
	//ControllerManagerImage karmada controllermanager image
	ControllerManagerImage string
	//WebhookImage karmada webhook image
	WebhookImage string
	//Namespace kubernetes namespace
	Namespace string
	//KubeConfig kubernetes config
	KubeConfig string
	//StorageClassesName kubernetes storage classes name
	StorageClassesName string
	//EtcdStorageMode etcd storage mode
	EtcdStorageMode string
	//EtcdReplicas etcd replicas
	EtcdReplicas int32
	//EtcdDataPath etcd host data path
	EtcdDataPath string
	//EtcdStorageSize etcd storage pvc size
	EtcdStorageSize string
	//APIServerReplicas karmada apiserver replicas
	APIServerReplicas int32
	//ControllerManagerReplicas  karmada controller manager replicas
	ControllerManagerReplicas int32
	//KubeControllerManagerReplicas kube controller manager replicas
	KubeControllerManagerReplicas int32
	//SchedulerReplicas karmada scheduler replicas
	SchedulerReplicas int32
	//WebhookReplicas karmada webhook replicas
	WebhookReplicas int32
	//DataPath karmada data path
	DataPath string
	//CRDs karmada crds
	CRDs string
	//KarmadaMasterIP karmada apiserver ip
	KarmadaMasterIP string
	//KarmadaMasterPort karmada apiserver port
	KarmadaMasterPort int32
	//ExternalIP the external IP of Karmada certificate
	ExternalIP string
)

// AddFlags adds flags to the specified FlagSet.
func AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&KubeConfig, "kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "absolute path to the kubeconfig file")
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
